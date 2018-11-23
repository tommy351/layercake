package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
)

type GlobalOptions struct {
	Config string `long:"config" description:"Path to config file" value-name:"PATH"`
	Debug  bool   `long:"debug" description:"Enable debug mode"`
}

const layercakeBaseDir = ".layercake"

var (
	globalOptions GlobalOptions

	globalCtx = newContext(context.Background())
	parser    = flags.NewParser(nil, flags.HelpFlag|flags.PassDoubleDash)
)

func init() {
	parser.AddGroup("Global Options", "", &globalOptions)
}

func main() {
	parser.CommandHandler = func(command flags.Commander, args []string) (err error) {
		if err = initLogger(); err != nil {
			return
		}

		// Try to load the config
		if err = initConfig(); err != nil {
			return
		}

		// Execute the command
		return command.Execute(args)
	}

	if _, err := parser.Parse(); err != nil {
		if e, ok := err.(*flags.Error); ok {
			switch e.Type {
			case flags.ErrHelp, flags.ErrCommandRequired:
				parser.WriteHelp(os.Stderr)
				return
			}
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
