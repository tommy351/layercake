package main

import "strings"

type FlagMap struct {
	Key   string
	Value *string
}

// nolint: unparam
func (f *FlagMap) UnmarshalFlag(value string) error {
	parts := strings.SplitN(value, "=", 2)
	f.Key = parts[0]

	if len(parts) == 2 {
		f.Value = &parts[1]
	}

	return nil
}
