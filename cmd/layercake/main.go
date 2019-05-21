package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// nolint: gochecknoglobals
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "layercake",
		Version:      fmt.Sprintf("%s, commit %s, built at %s", version, commit, date),
		SilenceUsage: true,
	}

	cmd.SetVersionTemplate("{{ .Version }}")

	pf := cmd.PersistentFlags()

	pf.String("config", "", "path to config file")
	pf.String("cwd", "", "path to working directory")

	pf.String("log-level", "info", "set log level")
	_ = viper.BindPFlag("log.level", pf.Lookup("log-level"))

	cmd.AddCommand(newBuildCommand())

	return cmd
}

func main() {
	rootCmd := newRootCommand()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
