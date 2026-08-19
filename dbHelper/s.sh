#!/bin/bash

SCRIPT_DIR=$(cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR" || exit 1

helperParam="--task compare-results --dbName tpcds10"

# Parameters:
#   --task    insert, compare-results, or run-sqls
#   --dbName  default is test
#   --address default is 10.2.12.124
#   --port    default is 8001
#   --user    default is root


if [[ "${1:-}" == "background" ]]; then
    go build -o db-helper . || exit 1
    ./db-helper ${helperParam}
else
    go run . ${helperParam}
fi

