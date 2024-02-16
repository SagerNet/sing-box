#!/usr/bin/env bash

PROJECTS=$(dirname "$0")/../..

function updateClient() {
  pushd clients/$1
  git fetch
  git reset FETCH_HEAD --hard
  popd
  git add clients/$1
}

updateClient "apple"
updateClient "android"
