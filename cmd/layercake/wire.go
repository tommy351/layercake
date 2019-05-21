// +build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/spf13/cobra"
	"github.com/tommy351/layercake/pkg/build"
	"github.com/tommy351/layercake/pkg/config"
	"github.com/tommy351/layercake/pkg/docker"
	"github.com/tommy351/layercake/pkg/log"
)

func InitializeBuilder(cmd *cobra.Command, args []string) (*build.Builder, func(), error) {
	wire.Build(
		config.LoadConfig,
		log.Set,
		docker.NewClient,
		build.Set,
	)
	return nil, nil, nil
}
