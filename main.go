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
	Config  string `long:"config" description:"Path to config file" value-name:"PATH"`
	CWD     string `long:"cwd" description:"Set working directory" value-name:"PATH"`
	Debug   bool   `long:"debug" description:"Enable debug mode"`
	Version bool   `long:"version" short:"v" description:"Print version information"`
}

const (
	layercakeBaseDir = ".layercake"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	globalOptions GlobalOptions
	cwd           string

	globalCtx = newContext(context.Background())
	parser    = flags.NewNamedParser("layercake", flags.HelpFlag|flags.PassDoubleDash)
)

func init() {
	if _, err := parser.AddGroup("Global Options", "", &globalOptions); err != nil {
		panic(err)
	}
}

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
				printHelp()
				return
			}
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printHelp() {
	if globalOptions.Version {
		fmt.Printf("%s, commit %s, built at %s", version, commit, date)
	} else {
		parser.WriteHelp(os.Stderr)
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
