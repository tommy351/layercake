package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

var (
	config             *Config
	defaultConfigPaths = []string{"layercake.yml", "layercake.yaml"}
)

type Config struct {
	Build map[string]BuildConfig `yaml:"build"`
}

func (c *Config) FindDependencies(name string) StringSet {
	deps := NewStringSet()
	build := c.Build[name]

	for _, script := range build.Scripts {
		if dep := script.Import; dep != "" {
			deps.Insert(dep)
		}
	}

	return deps
}

func (c *Config) FindDependants(name string) StringSet {
	result := NewStringSet()

	for k := range c.Build {
		deps := c.FindDependencies(k)

		if deps.Contains(name) {
			result.Insert(k)
		}
	}

	return result
}

func (c *Config) SortBuilds() *OrderedStringSet {
	result := NewOrderedStringSet()
	depMap := map[string]StringSet{}

	for name := range c.Build {
		depMap[name] = c.FindDependencies(name)
	}

	for len(depMap) > 0 {
		for k, deps := range depMap {
			for dep := range deps {
				if result.Contains(dep) {
					delete(deps, dep)
				}
			}

			if len(deps) == 0 {
				delete(depMap, k)
				result.Insert(k)
			}
		}
	}

	return result
}

type BuildConfig struct {
	From      string            `yaml:"from"`
	Image     string            `yaml:"image"`
	Args      map[string]string `yaml:"args"`
	Scripts   []BuildScript     `yaml:"scripts"`
	CacheFrom []string          `yaml:"cache_from"`
	Labels    map[string]string `yaml:"labels"`
}

func (b BuildConfig) Dockerfile() string {
	lines := []string{"FROM " + b.From}

	for _, script := range b.Scripts {
		lines = append(lines, script.Dockerfile())
	}

	return strings.Join(lines, "\n")
}

type BuildScript struct {
	Run         string `yaml:"run"`
	Arg         string `yaml:"arg"`
	WorkDir     string `yaml:"workdir"`
	Env         string `yaml:"env"`
	Label       string `yaml:"label"`
	Expose      string `yaml:"expose"`
	Add         string `yaml:"add"`
	Copy        string `yaml:"copy"`
	Entrypoint  string `yaml:"entrypoint"`
	Volume      string `yaml:"volume"`
	User        string `yaml:"user"`
	Cmd         string `yaml:"cmd"`
	Maintainer  string `yaml:"maintainer"`
	OnBuild     string `yaml:"onbuild"`
	StopSignal  string `yaml:"stopsignal"`
	HealthCheck string `yaml:"healthcheck"`
	Shell       string `yaml:"shell"`
	Import      string `yaml:"import"`
}

func (b BuildScript) Dockerfile() string {
	if b.Run != "" {
		return "RUN " + b.Run
	}

	if b.Arg != "" {
		return "ARG " + b.Arg
	}

	if b.WorkDir != "" {
		return "WORKDIR " + b.WorkDir
	}

	if b.Env != "" {
		return "ENV " + b.Env
	}

	if b.Label != "" {
		return "LABEL " + b.Label
	}

	if b.Expose != "" {
		return "EXPOSE " + b.Expose
	}

	if b.Add != "" {
		return "ADD " + b.Add
	}

	if b.Copy != "" {
		return "COPY " + b.Copy
	}

	if b.Entrypoint != "" {
		return "ENTRYPOINT " + b.Entrypoint
	}

	if b.Volume != "" {
		return "VOLUME " + b.Volume
	}

	if b.User != "" {
		return "USER " + b.User
	}

	if b.Cmd != "" {
		return "CMD " + b.Cmd
	}

	if b.Maintainer != "" {
		return "MAINTAINER " + b.Maintainer
	}

	if b.OnBuild != "" {
		return "ONBUILD " + b.OnBuild
	}

	if b.StopSignal != "" {
		return "STOPSIGNAL " + b.StopSignal
	}

	if b.HealthCheck != "" {
		return "HEALTHCHECK " + b.HealthCheck
	}

	if b.Shell != "" {
		return "sHELL " + b.Shell
	}

	if b.Import != "" {
		return fmt.Sprintf("ADD %s/%s.tar /", layercakeBaseDir, b.Import)
	}

	return ""
}

func LoadConfig(data []byte) (*Config, error) {
	var conf Config

	if err := yaml.Unmarshal(data, &conf); err != nil {
		return nil, err
	}

	return &conf, nil
}

func LoadConfigFile(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, err
	}

	return LoadConfig(data)
}

func initConfig() (err error) {
	if path := globalOptions.Config; path != "" {
		config, err = LoadConfigFile(path)
	} else {
		for _, path := range defaultConfigPaths {
			if config, err = LoadConfigFile(path); config != nil {
				err = nil
				return
			}
		}
	}

	return
}
