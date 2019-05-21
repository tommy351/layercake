package config

import (
	"os"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"
)

type Config struct {
	Build BuildConfig `mapstructure:"build"`
	Log   LogConfig   `mapstructure:"log"`
	CWD   string
}

func LoadConfig(cmd *cobra.Command) (*Config, error) {
	cwd := cmd.Flag("cwd").Value.String()

	if cwd == "" {
		var err error
		cwd, err = os.Getwd()

		if err != nil {
			return nil, xerrors.Errorf("failed to get working directory: %w", err)
		}
	}

	viper.SetConfigType("yaml")

	if path := cmd.Flag("config").Value.String(); path != "" {
		viper.SetConfigFile(path)
	} else {
		viper.SetConfigName("layercake")
		viper.AddConfigPath(cwd)
	}

	if err := viper.MergeInConfig(); err != nil {
		return nil, xerrors.Errorf("failed to read config: %w", err)
	}

	var conf Config
	err := viper.Unmarshal(&conf, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		decodeBuildScript,
	)))

	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal config: %w", err)
	}

	conf.CWD = cwd
	return &conf, nil
}
