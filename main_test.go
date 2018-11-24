package main

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	logger.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}
