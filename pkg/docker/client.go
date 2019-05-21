package docker

import (
	"context"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/xerrors"
)

const negotiateTimeout = time.Second * 5

type Client interface {
	ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImageSave(ctx context.Context, ids []string) (io.ReadCloser, error)
}

func NewClient() (Client, error) {
	c, err := client.NewClientWithOpts(client.FromEnv)

	if err != nil {
		return nil, xerrors.Errorf("failed to create a docker client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), negotiateTimeout)
	defer cancel()

	c.NegotiateAPIVersion(ctx)
	return c, nil
}
