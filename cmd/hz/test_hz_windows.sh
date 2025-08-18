#! /usr/bin/env bash

set -e

# const value define
moduleName="github.com/cloudwego/hertz/cmd/hz/test"
curDir=`pwd`
thriftIDL=$curDir"/testdata/thrift/psm.thrift"
protobuf2IDL=$curDir"/testdata/protobuf2/psm/psm.proto"
proto2Search=$curDir"/testdata/protobuf2"
protobuf3IDL=$curDir"/testdata/protobuf3/psm/psm.proto"
proto3Search=$curDir"/testdata/protobuf3"
protoSearch=$curDir"/testdata/include"

compile_hz() {
  go install .
}

install_dependent_tools() {
 # install thriftgo
 go install github.com/cloudwego/thriftgo@latest
}

go_tidy_build() {
  # make sure we get the latest version for testing
  go get github.com/cloudwego/hertz@develop
  go mod tidy && go build .
}

test_thrift() {
  # test thrift
  mkdir -p test
  cd test
  hz new --idl=$thriftIDL --mod=$moduleName -f --model_dir=hertz_model --handler_dir=hertz_handler --router_dir=hertz_router
  go_tidy_build
  hz update --idl=$thriftIDL
  hz model --idl=$thriftIDL --model_dir=hertz_model
  hz client --idl=$thriftIDL --client_dir=hertz_client
  cd ..
  rm -rf test
}

test_protobuf2() {
  # test protobuf2
  mkdir -p test
  cd test
  hz new -I=$protoSearch -I=$proto2Search --idl=$protobuf2IDL --mod=$moduleName -f --model_dir=hertz_model --handler_dir=hertz_handler --router_dir=hertz_router
  go_tidy_build
  hz update -I=$protoSearch -I=$proto2Search --idl=$protobuf2IDL
  hz model -I=$protoSearch -I=$proto2Search --idl=$protobuf2IDL --model_dir=hertz_model
  hz client -I=$protoSearch -I=$proto2Search --idl=$protobuf2IDL --client_dir=hertz_client
  cd ..
  rm -rf test
}

test_protobuf3() {
  # test protobuf2
  mkdir -p test
  cd test
  hz new -I=$protoSearch -I=$proto3Search --idl=$protobuf3IDL --mod=$moduleName -f --model_dir=hertz_model --handler_dir=hertz_handler --router_dir=hertz_router
  go_tidy_build
  hz update -I=$protoSearch -I=$proto3Search --idl=$protobuf3IDL
  hz model -I=$protoSearch -I=$proto3Search --idl=$protobuf3IDL --model_dir=hertz_model
  hz client -I=$protoSearch -I=$proto3Search --idl=$protobuf3IDL --client_dir=hertz_client
  cd ..
  rm -rf test
}

main() {
  compile_hz
  install_dependent_tools
# todo: add thrift test when thriftgo fixed windows
  echo "test thrift......"
  test_thrift
  echo "test protobuf2......"
  test_protobuf2
  echo "test protobuf3......"
  test_protobuf3
  echo "hz execute success"
}
main
