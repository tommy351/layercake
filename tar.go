package main

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ansel1/merry"
	"github.com/sabhiram/go-gitignore"
)

func TarAddFile(tw *tar.Writer, header *tar.Header, r io.Reader) (int64, error) {
	if err := tw.WriteHeader(header); err != nil {
		return 0, merry.Wrap(err)
	}

	written, err := io.Copy(tw, r)
	return written, merry.Wrap(err)
}

func TarAddDir(tw *tar.Writer, base string, ignoreParser ignore.IgnoreParser) error {
	return filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		name := strings.TrimPrefix(filepath.ToSlash(strings.TrimPrefix(path, base)), "/")
		log := logger.WithField("file", name)

		// Determine if the path should be ignored
		if info.IsDir() {
			if ignoreParser.MatchesPath(name + "/") {
				return filepath.SkipDir
			}

			return nil
		}

		if ignoreParser.MatchesPath(name) {
			return nil
		}

		header := &tar.Header{
			Name:    name,
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}

		file, err := os.Open(path)

		if err != nil {
			log.Error("Failed to open the file")
			return merry.Wrap(err)
		}

		defer file.Close()

		written, err := TarAddFile(tw, header, file)

		if err != nil {
			log.Error("Failed to write the file to tar")
			return merry.Wrap(err)
		}

		log.WithField("size", written).Debug("File is written to tar")
		return nil
	})
}
