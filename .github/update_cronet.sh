#!/usr/bin/env bash

PROJECTS=$(dirname "$0")/../..
git -C $PROJECTS/cronet-go fetch origin go
go get -x github.com/sagernet/cronet-go/all@$(git -C $PROJECTS/cronet-go rev-parse origin/go)

go mod tidy
