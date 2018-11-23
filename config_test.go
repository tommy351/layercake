package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func normalizeYAMLString(input string) string {
	return strings.Replace(strings.TrimSpace(input), "\t", "  ", -1)
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
