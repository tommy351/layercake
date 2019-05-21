package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newBuildCommand() *cobra.Command {
	var buildArg map[string]string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build images",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			for k, v := range buildArg {
				viper.Set("build.args."+k, v)
			}

			builder, cleanup, err := InitializeBuilder(cmd, args)

			if err != nil {
				return err
			}

			defer cleanup()

			return builder.Start()
		},
	}

	f := cmd.Flags()

	f.StringToStringVar(&buildArg, "build-arg", nil, "set build-time variables")

	f.Bool("dry-run", false, "print Dockerfile only")
	_ = viper.BindPFlag("build.dry_run", f.Lookup("dry-run"))

	f.Bool("no-cache", false, "do not use cache when building the image")
	_ = viper.BindPFlag("build.no_cache", f.Lookup("no-cache"))

	return cmd
}
