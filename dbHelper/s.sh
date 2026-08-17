#!/bin/bash

SCRIPT_DIR=$(cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR" || exit 1

# go run . --task compare-results
# go build -o db-helper .
go run . --task compare-results

