package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ansel1/merry"
	"github.com/jessevdk/go-flags"
)

type GlobalOptions struct {
	Config string `long:"config" description:"Path to config file" value-name:"PATH"`
	CWD    string `long:"cwd" description:"Set working directory" value-name:"PATH"`
	Debug  bool   `long:"debug" description:"Enable debug mode"`
}

const layercakeBaseDir = ".layercake"

var (
	globalOptions GlobalOptions
	cwd           string

	globalCtx = newContext(context.Background())
	parser    = flags.NewParser(&globalOptions, flags.HelpFlag|flags.PassDoubleDash)
)

func main() {
	parser.CommandHandler = func(command flags.Commander, args []string) error {
		if err := RunSeries(initCWD, initLogger); err != nil {
			return err
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

func initCWD() (err error) {
	if globalOptions.CWD == "" {
		cwd, err = os.Getwd()
		return merry.Wrap(err)
	}

	cwd, err = filepath.Abs(globalOptions.CWD)
	return merry.Wrap(err)
}
