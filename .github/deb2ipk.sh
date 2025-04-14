#!/usr/bin/env bash
# mod from https://gist.github.com/pldubouilh/c5703052986bfdd404005951dee54683

set -e -o pipefail

PROJECT=$(dirname "$0")/../..
TMP_PATH=`mktemp -d`
cp $2 $TMP_PATH
pushd $TMP_PATH

DEB_NAME=`ls *.deb`
ar x $DEB_NAME

mkdir control
pushd control
tar xf ../control.tar.gz
rm md5sums
sed "s/Architecture:\\ \w*/Architecture:\\ $1/g" ./control -i
cat control
tar czf ../control.tar.gz ./*
popd

DEB_NAME=${DEB_NAME%.deb}
tar czf $DEB_NAME.ipk control.tar.gz data.tar.gz debian-binary
popd

cp $TMP_PATH/$DEB_NAME.ipk $3
rm -r $TMP_PATH
