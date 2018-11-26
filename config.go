package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/ansel1/merry"
	"gopkg.in/yaml.v2"
)

var (
	defaultConfigPaths = []string{"layercake.yml", "layercake.yaml"}
	errNoConfigFound   = merry.New("unable to find the config file")
)

type Config struct {
	Build map[string]BuildConfig `yaml:"build"`
}

func (c *Config) FindDependencies(name string) StringSet {
	return c.Build[name].FindImports()
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
	Tags      []string          `yaml:"tags"`
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

func (b BuildConfig) FindImports() StringSet {
	result := NewStringSet()

	for _, script := range b.Scripts {
		if s := script.Import; s != "" {
			result.Insert(s)
		}
	}

	return result
}

type BuildScript struct {
	Raw         string
	Instruction string
	Value       string
	Import      string
}

func (b BuildScript) Dockerfile() string {
	if b.Raw != "" {
		return b.Raw
	}

	if b.Import != "" {
		return fmt.Sprintf("ADD %s/%s.tar /", layercakeBaseDir, b.Import)
	}

	return b.Instruction + " " + b.Value
}

func (b *BuildScript) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string

	if err := unmarshal(&s); err == nil {
		b.Raw = s
		return nil
	}

	var m yaml.MapSlice

	if err := unmarshal(&m); err != nil {
		return err
	}

	if len(m) == 0 {
		return errors.New("build script should be a string or a map")
	}

	item := m[0]
	key, err := b.encode(item.Key)

	if err != nil {
		return err
	}

	key = strings.ToUpper(key)

	if key == "IMPORT" {
		b.Import = item.Value.(string)
		return nil
	}

	value, err := b.encode(item.Value)

	if err != nil {
		return err
	}

	b.Instruction = key
	b.Value = value

	return nil
}

func (b BuildScript) encode(data interface{}) (string, error) {
	if m, ok := data.(yaml.MapSlice); ok {
		var result []string

		for _, item := range m {
			key, err := b.encode(item.Key)

			if err != nil {
				return "", err
			}

			value, err := b.encode(item.Value)

			if err != nil {
				return "", err
			}

			result = append(result, key+"="+strconv.Quote(value))
		}

		return strings.Join(result, " "), nil
	}

	v := reflect.ValueOf(data)

	switch v.Kind() {
	case reflect.String:
		return v.String(), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10), nil

	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64), nil

	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil

	case reflect.Array, reflect.Slice:
		result, err := json.Marshal(v.Interface())

		if err != nil {
			return "", err
		}

		return string(result), nil
	}

	return "", fmt.Errorf("unsupported type %T in build script", data)
}

func LoadConfig(data []byte) (*Config, error) {
	var conf Config

	if err := yaml.UnmarshalStrict(data, &conf); err != nil {
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

func InitConfig() (*Config, error) {
	if path := globalOptions.Config; path != "" {
		return LoadConfigFile(path)
	}

	for _, path := range defaultConfigPaths {
		config, err := LoadConfigFile(path)

		// Return if config is loaded
		if config != nil {
			return config, nil
		}

		// Return if the error is because of parsing
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	return nil, errNoConfigFound
}
