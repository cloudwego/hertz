#!/usr/bin/env bash
#
# CI test script for hz on Linux/macOS.
# Installs dependencies, then runs generate.sh with --verify --clean.

set -e

cd "$(dirname "$0")"

LOCAL_DIR="$PWD/.local"
mkdir -p "$LOCAL_DIR/bin"
export PATH="$LOCAL_DIR/bin:$PATH"

# install thriftgo
go install github.com/cloudwego/thriftgo@latest

# install protoc if not already present (standard layout: bin/protoc + include/google/)
if [ ! -f "$LOCAL_DIR/bin/protoc" ]; then
  wget -q https://github.com/protocolbuffers/protobuf/releases/download/v3.19.4/protoc-3.19.4-linux-x86_64.zip
  unzip -qd "$LOCAL_DIR" protoc-3.19.4-linux-x86_64.zip
  rm -f protoc-3.19.4-linux-x86_64.zip
fi

# build hz
go build -o hz .

# run all targets with build verification, cleanup after
./generate.sh --hz ./hz --verify --clean thrift proto2 proto3

echo "hz execute success"
