#!/usr/bin/env bash

set -e -o pipefail

DIR=$(dirname "$0")
PROJECT=$DIR/../..

pushd $PROJECT
git fetch
git reset FETCH_HEAD --hard
git clean -fdx
popd

$DIR/reinstall.sh