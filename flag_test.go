package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stringP(s string) *string {
	return &s
}

func TestFlagMap_UnmarshalFlag(t *testing.T) {
	tests := []struct {
		Name     string
		Arg      string
		Expected FlagMap
	}{
		{
			Name: "Only key",
			Arg:  "foo",
			Expected: FlagMap{
				Key: "foo",
			},
		},
		{
			Name: "Key and value",
			Arg:  "foo=bar",
			Expected: FlagMap{
				Key:   "foo",
				Value: stringP("bar"),
			},
		},
		{
			Name: "Multiple delimiters",
			Arg:  "foo=a=b=c",
			Expected: FlagMap{
				Key:   "foo",
				Value: stringP("a=b=c"),
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			var fm FlagMap
			require.NoError(t, fm.UnmarshalFlag(test.Arg))
			assert.Equal(t, test.Expected, fm)
		})
	}
}
