#!/bin/bash

set -euo pipefail

go get github.com/google/wire/cmd/wire

echo "Generate Go files"
go generate ./...
