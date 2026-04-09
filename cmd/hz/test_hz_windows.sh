#!/usr/bin/env bash
#
# CI test script for hz on Windows (Git Bash / WSL).
# Installs dependencies, then runs generate.sh with --verify --clean.

set -e

cd "$(dirname "$0")"

LOCAL_DIR="$PWD/.local"
mkdir -p "$LOCAL_DIR/bin"
export PATH="$LOCAL_DIR/bin:$PATH"

# install thriftgo
go install github.com/cloudwego/thriftgo@latest

# install protoc if not already present (standard layout: bin/protoc + include/google/)
if [ ! -f "$LOCAL_DIR/bin/protoc" ] && [ ! -f "$LOCAL_DIR/bin/protoc.exe" ]; then
  curl -sLO https://github.com/protocolbuffers/protobuf/releases/download/v3.19.4/protoc-3.19.4-win64.zip
  unzip -qd "$LOCAL_DIR" protoc-3.19.4-win64.zip
  rm -f protoc-3.19.4-win64.zip
fi

# build hz
go build -o hz.exe .

# run all targets with build verification, cleanup after
./generate.sh --hz "$PWD/hz.exe" --verify --clean thrift proto2 proto3

echo "hz execute success"
