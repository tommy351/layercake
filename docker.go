package main

import (
	"context"

	"github.com/ansel1/merry"
	"github.com/docker/docker/client"
)

func NewDockerClient(ctx context.Context) (*client.Client, error) {
	c, err := client.NewClientWithOpts(client.FromEnv)

	if err != nil {
		logger.Error("Failed to initialize a Docker client")
		return nil, merry.Wrap(err)
	}

	c.NegotiateAPIVersion(ctx)
	logger.WithField("version", c.ClientVersion()).Debug("Docker client is initialized")
	return c, nil
}
