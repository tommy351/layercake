package main

import (
	"context"
	"os"
	"testing"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDockerClient(t *testing.T) {
	ctx := context.Background()

	t.Run("Default", func(t *testing.T) {
		c, err := NewDockerClient(ctx)
		require.NoError(t, err)
		assert.Equal(t, client.DefaultDockerHost, c.DaemonHost())
	})

	t.Run("From env", func(t *testing.T) {
		host := "tcp://127.0.0.1:2375"
		os.Setenv("DOCKER_HOST", host)
		defer os.Unsetenv("DOCKER_HOST")

		c, err := NewDockerClient(ctx)
		require.NoError(t, err)
		assert.Equal(t, host, c.DaemonHost())
	})
}
