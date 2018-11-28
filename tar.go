package main

import (
	"archive/tar"
	"io"

	"github.com/ansel1/merry"
)

func TarAddFile(tw *tar.Writer, header *tar.Header, r io.Reader) (int64, error) {
	if err := tw.WriteHeader(header); err != nil {
		return 0, merry.Wrap(err)
	}

	written, err := io.Copy(tw, r)
	return written, merry.Wrap(err)
}
