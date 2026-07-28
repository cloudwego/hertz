#!/usr/bin/env bash
#
# Generates hz example/test output using the test IDL files.
#
# Usage:
#   ./generate.sh [OPTIONS] [TARGETS...]
#
# Targets: thrift, proto2, proto3, handler_by_method (default: all)
#
# Options:
#   --verify   Run "go mod tidy && go build" to verify generated code compiles
#   --clean    Remove output after verification (for CI)
#   --hz PATH  Path to hz binary (default: builds from source)
#
# Examples:
#   ./generate.sh                        # generate all examples to examples/output/
#   ./generate.sh thrift proto3           # generate specific targets
#   ./generate.sh --verify --clean        # CI mode: generate, build, cleanup

set -e

HZ_DIR="$(cd "$(dirname "$0")" && pwd)"
OUTPUT_DIR="$HZ_DIR/generate_out"
MODULE="github.com/cloudwego/hertz/cmd/hz/test"

THRIFT_IDL="$HZ_DIR/testdata/thrift/psm.thrift"
PROTO2_IDL="$HZ_DIR/testdata/protobuf2/psm/psm.proto"
PROTO2_SEARCH="$HZ_DIR/testdata/protobuf2"
PROTO3_IDL="$HZ_DIR/testdata/protobuf3/psm/psm.proto"
PROTO3_SEARCH="$HZ_DIR/testdata/protobuf3"
# Locate protoc's well-known types include dir.
# Search: relative to protoc binary, then common system paths.
PROTO_SYSTEM=""
for _dir in \
  "$(cd "$(dirname "$(command -v protoc 2>/dev/null)")/.." 2>/dev/null && pwd)/include" \
  "/usr/include" \
  "/usr/local/include" \
  "$HZ_DIR/.local/include"; do
  if [ -d "$_dir/google/protobuf" ] 2>/dev/null; then
    PROTO_SYSTEM="$_dir"
    break
  fi
done

HZ=""
VERIFY=false
CLEAN=false
TARGETS=()

# parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --verify) VERIFY=true; shift ;;
    --clean)  CLEAN=true; shift ;;
    --hz)     HZ="$(cd "$(dirname "$2")" && pwd)/$(basename "$2")"; shift 2 ;;
    -*)       echo "unknown option: $1" >&2; exit 1 ;;
    *)        TARGETS+=("$1"); shift ;;
  esac
done

# default: all targets
if [[ ${#TARGETS[@]} -eq 0 ]]; then
  TARGETS=(thrift proto2 proto3 handler_by_method)
fi

# build hz if not provided
if [[ -z "$HZ" ]]; then
  echo "==> Building hz..."
  cd "$HZ_DIR"
  go build -o hz .
  HZ="$HZ_DIR/hz"
fi

cleanup() { cd "$HZ_DIR"; }
trap cleanup EXIT

# run_target NAME NEW_ARGS...
# Runs: hz new, hz update, hz model, hz client in a subdirectory.
run_target() {
  local name="$1"; shift
  local dir="$OUTPUT_DIR/$name"

  echo "==> $name: new..."
  rm -rf "$dir"
  mkdir -p "$dir"
  cd "$dir"
  "$HZ" new --mod="$MODULE" -f "$@"

  if $VERIFY; then
    echo "  build..."
    go get github.com/cloudwego/hertz@main
    go mod tidy && go build .
  fi

  echo "  update..."
  "$HZ" update "$@"
  echo "  model..."
  "$HZ" model "$@"
  echo "  client..."
  "$HZ" client "$@" --client_dir=hertz_client

  if $CLEAN; then
    rm -rf "$dir"
  fi
}

for target in "${TARGETS[@]}"; do
  case "$target" in
    thrift)
      run_target thrift --idl="$THRIFT_IDL"
      ;;
    proto2)
      run_target proto2 ${PROTO_SYSTEM:+-I="$PROTO_SYSTEM"} -I="$PROTO2_SEARCH" --idl="$PROTO2_IDL"
      ;;
    proto3)
      run_target proto3 ${PROTO_SYSTEM:+-I="$PROTO_SYSTEM"} -I="$PROTO3_SEARCH" --idl="$PROTO3_IDL"
      ;;
    handler_by_method)
      dir="$OUTPUT_DIR/handler_by_method"
      echo "==> handler_by_method: new..."
      rm -rf "$dir"
      mkdir -p "$dir"
      cd "$dir"
      "$HZ" new --idl="$THRIFT_IDL" --mod="$MODULE" -f --handler_by_method
      if $VERIFY; then
        echo "  build..."
        go get github.com/cloudwego/hertz@main
        go mod tidy && go build .
      fi
      if $CLEAN; then rm -rf "$dir"; fi
      ;;
    *)
      echo "unknown target: $target" >&2; exit 1
      ;;
  esac
done

if ! $CLEAN; then
  echo ""
  echo "Done. Generated examples in: $OUTPUT_DIR"
  echo ""
  echo "Directory layout:"
  cd "$OUTPUT_DIR"
  find . -type f -name "*.go" | sort | head -80
fi
