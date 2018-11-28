package main

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tarFile struct {
	Name string
	Data []byte
}

func readTar(r io.Reader) ([]tarFile, error) {
	var files []tarFile
	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		file := tarFile{
			Name: header.Name,
		}

		if file.Data, err = ioutil.ReadAll(tr); err != nil {
			return nil, err
		}

		files = append(files, file)
	}

	return files, nil
}

func TestTarAddFile(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Add file
	data := []byte("foobar")
	header := &tar.Header{
		Name: "foo/bar",
		Size: int64(len(data)),
	}

	written, err := TarAddFile(tw, header, bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, int64(len(data)), written)

	// Close tar
	require.NoError(t, tw.Close())

	// Read tar
	files, err := readTar(&buf)
	require.NoError(t, err)
	assert.ElementsMatch(t, []tarFile{
		{Name: "foo/bar", Data: data},
	}, files)
}
