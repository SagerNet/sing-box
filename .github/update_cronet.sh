#!/usr/bin/env bash

set -e -o pipefail

SCRIPT_DIR=$(dirname "$0")
PROJECTS=$SCRIPT_DIR/../..

git -C $PROJECTS/cronet-go fetch origin main
git -C $PROJECTS/cronet-go fetch origin go
go get -x github.com/sagernet/cronet-go/all@$(git -C $PROJECTS/cronet-go rev-parse origin/go)
go get -x github.com/sagernet/cronet-go@$(git -C $PROJECTS/cronet-go rev-parse origin/go)
go mod tidy
git -C $PROJECTS/cronet-go rev-parse origin/HEAD > "$SCRIPT_DIR/CRONET_GO_VERSION"
