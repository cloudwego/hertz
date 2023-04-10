#! /usr/bin/env bash

# const value define
moduleName="github.com/cloudwego/hertz/cmd/hz/test"
curDir=`pwd`
thriftIDL=$curDir"/testdata/thrift/psm.thrift"
protobuf2IDL=$curDir"/testdata/protobuf2/psm/psm.proto"
proto2Search=$curDir"/testdata/protobuf2"
protobuf3IDL=$curDir"/testdata/protobuf3/psm/psm.proto"
proto3Search=$curDir"/testdata/protobuf3"
protoSearch="/usr/local/include"

judge_exit() {
  code=$1
  if [ $code != 0 ]; then
    exit $code
  fi
}

compile_hz() {
  go build -o hz
  judge_exit "$?"
}

install_dependent_tools() {
  # install thriftgo
  go install github.com/cloudwego/thriftgo@latest

  # install protoc
  wget https://github.com/protocolbuffers/protobuf/releases/download/v3.19.4/protoc-3.19.4-linux-x86_64.zip
  unzip -d protoc-3.19.4-linux-x86_64 protoc-3.19.4-linux-x86_64.zip
  cp protoc-3.19.4-linux-x86_64/bin/protoc /usr/local/bin/protoc
  cp -r protoc-3.19.4-linux-x86_64/include/google /usr/local/include/google
}

test_thrift() {
  mkdir -p test
  cd test
  ../hz new --idl=$thriftIDL --mod=$moduleName -f --model_dir=hertz_model --handler_dir=hertz_handler --router_dir=hertz_router
  judge_exit "$?"
  go mod tidy && go build .
  judge_exit "$?"
  ../hz update --idl=$thriftIDL
  judge_exit "$?"
  ../hz model --idl=$thriftIDL --model_dir=hertz_model
  judge_exit "$?"
  ../hz client --idl=$thriftIDL --client_dir=hertz_client
  judge_exit "$?"
  cd ..
  rm -rf test
}

test_protobuf2() {
  # test protobuf2
  mkdir -p test
  cd test
  ../hz new -I=$protoSearch -I=$proto2Search --idl=$protobuf2IDL --mod=$moduleName -f --model_dir=hertz_model --handler_dir=hertz_handler --router_dir=hertz_router
  judge_exit "$?"
  go mod tidy && go build .
  judge_exit "$?"
  ../hz update -I=$protoSearch -I=$proto2Search --idl=$protobuf2IDL
  judge_exit "$?"
  ../hz model -I=$protoSearch -I=$proto2Search --idl=$protobuf2IDL --model_dir=hertz_model
  judge_exit "$?"
  ../hz client -I=$protoSearch -I=$proto2Search --idl=$protobuf2IDL --client_dir=hertz_client
  judge_exit "$?"
  cd ..
  rm -rf test
}

test_protobuf3() {
  # test protobuf2
  mkdir -p test
  cd test
  ../hz new -I=$protoSearch -I=$proto3Search --idl=$protobuf3IDL --mod=$moduleName -f --model_dir=hertz_model --handler_dir=hertz_handler --router_dir=hertz_router
  judge_exit "$?"
  go mod tidy && go build .
  judge_exit "$?"
  ../hz update -I=$protoSearch -I=$proto3Search --idl=$protobuf3IDL
  judge_exit "$?"
  ../hz model -I=$protoSearch -I=$proto3Search --idl=$protobuf3IDL --model_dir=hertz_model
  judge_exit "$?"
  ../hz client -I=$protoSearch -I=$proto3Search --idl=$protobuf3IDL --client_dir=hertz_client
  judge_exit "$?"
  cd ..
  rm -rf test
}

main() {
  compile_hz
  judge_exit "$?"
  install_dependent_tools
  judge_exit "$?"
  echo "test thrift......"
  test_thrift
  judge_exit "$?"
  echo "test protobuf2......"
  test_protobuf2
  judge_exit "$?"
  echo "test protobuf3......"
  test_protobuf3
  judge_exit "$?"
  echo "hz execute success"
}
main
