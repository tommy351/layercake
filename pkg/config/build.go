package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/xerrors"
)

type BuildConfig struct {
	Images  map[string]BuildImage `mapstructure:"images"`
	Args    map[string]string     `mapstructure:"args"`
	NoCache bool                  `mapstructure:"no_cache"`
	DryRun  bool                  `mapstructure:"dry_run"`
}

type BuildImage struct {
	From      string            `mapstructure:"from"`
	Tags      []string          `mapstructure:"tags"`
	Args      map[string]string `mapstructure:"args"`
	Scripts   []BuildScript     `mapstructure:"scripts"`
	CacheFrom []string          `mapstructure:"cache_from"`
	Labels    map[string]string `mapstructure:"labels"`
}

type BuildScript struct {
	Raw         string
	Instruction string
	Value       string
	Import      string
}

func decodeBuildScript(fromType, toType reflect.Type, data interface{}) (interface{}, error) {
	if toType != reflect.TypeOf(BuildScript{}) {
		return data, nil
	}

	switch fromType.Kind() {
	case reflect.Map:
		for k, v := range data.(map[interface{}]interface{}) {
			key := strings.ToUpper(toString(k))
			value := toString(v)

			if key == "IMPORT" {
				return BuildScript{Import: value}, nil
			}

			return BuildScript{
				Instruction: key,
				Value:       value,
			}, nil
		}
	case reflect.String:
		return BuildScript{Raw: data.(string)}, nil
	}

	return nil, xerrors.Errorf("expected map or string in BuildScript, got %T", fromType)
}

func toString(input interface{}) string {
	v := reflect.ValueOf(input)

	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Float32:
		return strconv.FormatFloat(v.Float(), 'f', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Array, reflect.Slice:
		output := make([]string, v.Len())

		for i := 0; i < v.Len(); i++ {
			output[i] = strconv.Quote(toString(v.Index(i).Interface()))
		}

		return "[" + strings.Join(output, ",") + "]"
	case reflect.Map:
		output := make([]string, 0, v.Len())
		iter := v.MapRange()

		for iter.Next() {
			output = append(output, fmt.Sprintf("%v=%q", iter.Key().Interface(), iter.Value().Interface()))
		}

		return strings.Join(output, " ")
	}

	return ""
}
