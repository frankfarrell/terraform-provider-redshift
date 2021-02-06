#!/usr/bin/env bash

VERSION=$(cat VERSION)

for tuple in darwin,amd64 linux,amd64 windows,amd64; do
  IFS=","; set -- $tuple;
  os=$1; arch=$2

  filename=terraform-provider-redshift_v${VERSION}_x4

  if [ "$os" == "windows" ]; then
    filename=$filename.exe
  fi
  filepath=dist/$os/$arch/$filename
  echo "Generating $filepath"
  GOOS=$os GOARCH=$arch go build -o $filepath
done
