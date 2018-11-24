package main

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	logger.SetOutput(ioutil.Discard)

	if err := initCWD(); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}
