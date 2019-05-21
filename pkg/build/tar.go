package build

import (
	"archive/tar"
	"bytes"
	"io"

	"golang.org/x/xerrors"
)

const dockerFilePath = ".layercake/Dockerfile"

func addDockerFileToTar(base io.Reader, content []byte) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := tw.WriteHeader(&tar.Header{
		Name: dockerFilePath,
		Size: int64(len(content)),
		Mode: 0600,
	})

	if err != nil {
		return nil, xerrors.Errorf("failed to write tar header: %w", err)
	}

	if _, err := tw.Write(content); err != nil {
		return nil, xerrors.Errorf("failed to write tar content: %w", err)
	}

	if err := tw.Flush(); err != nil {
		return nil, xerrors.Errorf("failed to flush tar: %w", err)
	}

	return io.MultiReader(&buf, base), nil
}

func listTarHeaders(tr *tar.Reader) ([]*tar.Header, error) {
	var headers []*tar.Header

	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, xerrors.Errorf("failed to read tar: %w", err)
		}

		headers = append(headers, header)
	}

	return headers, nil
}
