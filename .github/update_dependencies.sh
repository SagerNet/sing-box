#!/usr/bin/env bash

PROJECTS=$(dirname "$0")/../..
go get -x github.com/sagernet/$1@$(git -C $PROJECTS/$1 rev-parse HEAD)
go mod tidy
