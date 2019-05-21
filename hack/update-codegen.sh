#!/bin/bash

set -euo pipefail

echo "Generate Go files"
go generate ./...
