package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func normalizeYAMLString(input string) string {
	return strings.Replace(strings.TrimSpace(input), "\t", "  ", -1)
}

func writeTempFile(t *testing.T, content []byte) (*os.File, error) {
	file, err := ioutil.TempFile("", "layercake")

	if err != nil {
		return nil, err
	}

	defer file.Close()

	if _, err := file.Write(content); err != nil {
		return nil, err
	}

	return file, nil
}

func TestConfig_FindDependencies(t *testing.T) {
	config := Config{
		Build: map[string]BuildConfig{
			"foo": {
				Scripts: []BuildScript{
					{Raw: "RUN foo"},
					{Import: "a"},
					{Import: "b"},
					{Import: "a"},
				},
			},
		},
	}

	expected := NewStringSet()
	expected.Insert("a", "b")
	assert.Equal(t, expected, config.FindDependencies("foo"))
}

func TestConfig_FindDependants(t *testing.T) {
	config := Config{
		Build: map[string]BuildConfig{
			"a": {
				Scripts: []BuildScript{
					{Import: "foo"},
				},
			},
			"b": {
				Scripts: []BuildScript{
					{Import: "bar"},
				},
			},
			"c": {
				Scripts: []BuildScript{
					{Import: "foo"},
				},
			},
		},
	}

	expected := NewStringSet()
	expected.Insert("a", "c")
	assert.Equal(t, expected, config.FindDependants("foo"))
}

func TestConfig_SortBuilds(t *testing.T) {
	config := Config{
		Build: map[string]BuildConfig{
			"a": {
				Scripts: []BuildScript{
					{Import: "c"},
				},
			},
			"b": {
				Scripts: []BuildScript{
					{Import: "a"},
				},
			},
			"c": {},
		},
	}

	expected := NewOrderedStringSet()
	expected.Insert("c", "a", "b")
	assert.Equal(t, expected, config.SortBuilds())
}

func TestBuildConfig_Dockerfile(t *testing.T) {
	config := BuildConfig{
		From: "alpine",
		Scripts: []BuildScript{
			{Raw: "RUN foo"},
			{Instruction: "CMD", Value: "bar"},
		},
	}

	assert.Equal(t, strings.TrimSpace(`
FROM alpine
RUN foo
CMD bar
`), config.Dockerfile())
}

func TestBuildConfig_FindImports(t *testing.T) {
	config := BuildConfig{
		Scripts: []BuildScript{
			{Raw: "RUN foo"},
			{Import: "a"},
			{Import: "b"},
			{Import: "a"},
		},
	}

	expected := NewStringSet()
	expected.Insert("a", "b")
	assert.Equal(t, expected, config.FindImports())
}

func TestBuildScript_Dockerfile(t *testing.T) {
	t.Run("Raw", func(t *testing.T) {
		script := BuildScript{Raw: "RUN foo"}
		assert.Equal(t, script.Raw, script.Dockerfile())
	})

	t.Run("Import", func(t *testing.T) {
		script := BuildScript{Import: "foo"}
		assert.Equal(t, fmt.Sprintf("ADD %s/%s.tar /", layercakeBaseDir, script.Import), script.Dockerfile())
	})

	t.Run("Instruction", func(t *testing.T) {
		script := BuildScript{Instruction: "RUN", Value: "bar"}
		assert.Equal(t, "RUN bar", script.Dockerfile())
	})
}

func TestBuildScript_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		Name     string
		Input    string
		Expected BuildScript
	}{
		{
			Name:     "string",
			Input:    "RUN foo",
			Expected: BuildScript{Raw: "RUN foo"},
		},
		{
			Name:     "Import",
			Input:    "import: foo",
			Expected: BuildScript{Import: "foo"},
		},
		{
			Name:     "Instruction: string",
			Input:    "run: foo",
			Expected: BuildScript{Instruction: "RUN", Value: "foo"},
		},
		{
			Name:     "Instruction: int",
			Input:    "expose: 80",
			Expected: BuildScript{Instruction: "EXPOSE", Value: "80"},
		},
		{
			Name:     "Instruction: float",
			Input:    "run: 3.14",
			Expected: BuildScript{Instruction: "RUN", Value: "3.14"},
		},
		{
			Name:     "Instruction: array",
			Input:    "cmd: ['echo', 'hello', 'world']",
			Expected: BuildScript{Instruction: "CMD", Value: `["echo","hello","world"]`},
		},
		{
			Name:     "Instruction: bool",
			Input:    "run: true",
			Expected: BuildScript{Instruction: "RUN", Value: "true"},
		},
		{
			Name: "Instruction: map",
			Input: normalizeYAMLString(`
env:
	a: b
	c: d
`),
			Expected: BuildScript{Instruction: "ENV", Value: `a="b" c="d"`},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var actual BuildScript
			err := yaml.Unmarshal([]byte(test.Input), &actual)
			require.NoError(t, err)
			assert.Equal(t, test.Expected, actual)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		config, err := LoadConfig([]byte(normalizeYAMLString(`
build:
	foo: 
		from: alpine
		image: foo-alpine
`)))

		require.NoError(t, err)
		assert.Equal(t, &Config{
			Build: map[string]BuildConfig{
				"foo": {
					From:  "alpine",
					Image: "foo-alpine",
				},
			},
		}, config)
	})

	t.Run("Error", func(t *testing.T) {
		config, err := LoadConfig([]byte(normalizeYAMLString(`
build:
	foo: 
		args: 123
`)))

		assert.Error(t, err)
		assert.Nil(t, config)
	})
}

func TestLoadConfigFile(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		file, err := writeTempFile(t, []byte(normalizeYAMLString(`
build:
	foo: 
		from: alpine
		image: foo-alpine
`)))
		require.NoError(t, err)
		defer os.Remove(file.Name())

		config, err := LoadConfigFile(file.Name())
		require.NoError(t, err)
		assert.Equal(t, &Config{
			Build: map[string]BuildConfig{
				"foo": {
					From:  "alpine",
					Image: "foo-alpine",
				},
			},
		}, config)
	})

	t.Run("Error", func(t *testing.T) {
		config, err := LoadConfigFile("foo")
		assert.Error(t, err)
		assert.Nil(t, config)
	})
}
