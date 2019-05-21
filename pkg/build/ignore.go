package build

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/builder/dockerignore"
	"github.com/tommy351/layercake/pkg/config"
	"golang.org/x/xerrors"
)

type ExcludedPatterns []string

func LoadIgnoreFile(conf *config.Config) (ExcludedPatterns, error) {
	path := filepath.Join(conf.CWD, ".dockerignore")
	file, err := os.Open(path)

	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, xerrors.Errorf("failed to open file: %w", err)
	}

	defer file.Close()

	patterns, err := dockerignore.ReadAll(file)

	if err != nil {
		return nil, xerrors.Errorf("failed to read the ignore file: %w", err)
	}

	return patterns, nil
}
