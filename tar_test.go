package main

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/sabhiram/go-gitignore"
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

func TestTarAddDir(t *testing.T) {
	base, err := ioutil.TempDir("", "layercake")
	require.NoError(t, err)
	defer os.RemoveAll(base)

	inputFiles := []struct {
		Path string
		Data []byte
	}{
		{
			Path: "foo",
			Data: []byte("a"),
		},
		{
			Path: "bar/baz",
			Data: []byte("b"),
		},
		{
			Path: "baz/foo",
			Data: []byte("c"),
		},
	}

	for _, file := range inputFiles {
		path := filepath.Join(base, file.Path)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), os.ModePerm))
		require.NoError(t, ioutil.WriteFile(path, file.Data, os.ModePerm))
	}

	ignorePattern, err := ignore.CompileIgnoreLines("baz/")
	require.NoError(t, err)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	written, err := TarAddDir(tw, base, ignorePattern)
	require.NoError(t, err)
	assert.Equal(t, int64(2), written)

	// Close tar
	require.NoError(t, tw.Close())

	// Read tar
	files, err := readTar(&buf)
	require.NoError(t, err)
	assert.ElementsMatch(t, []tarFile{
		{Name: "foo", Data: []byte("a")},
		{Name: "bar/baz", Data: []byte("b")},
	}, files)
}
