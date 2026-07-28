# hz - Hertz Code Generator

`hz` is the official code generation tool for the [Hertz](https://github.com/cloudwego/hertz) HTTP framework. It parses IDL (Interface Definition Language) files — Thrift or Protobuf — and generates a complete, scaffolded Hertz project including handlers, routers, models, and client code.

## Quick Start

```bash
# Build
cd cmd/hz && go build -o hz .

# Generate a new project from Thrift IDL
hz new --idl api.thrift --module github.com/example/myservice

# Generate a new project from Protobuf IDL
hz new --idl api.proto --module github.com/example/myservice

# Update an existing project after IDL changes (module read from go.mod)
hz update --idl api.thrift

# Generate model code only (module read from go.mod)
hz model --idl api.thrift

# Generate client code (module read from go.mod)
hz client --idl api.thrift --base_domain localhost:8888
```

## Commands

| Command  | Description |
|----------|-------------|
| `new`    | Scaffold a new Hertz project from IDL, generating layout, handlers, routers, and models |
| `update` | Incrementally add new handlers/routes for newly added IDL methods without overwriting existing code |
| `model`  | Generate only Go struct definitions (models) from IDL types |
| `client` | Generate Hertz HTTP client code from IDL service definitions |

## Flags

### Global

| Flag | Description |
|------|-------------|
| `--verbose`, `-vv` | Turn on verbose (debug-level) logging |

### Project Structure

| Flag | Commands | Description |
|------|----------|-------------|
| `--idl` | all | IDL file path (`.thrift` or `.proto`) |
| `--module`, `--mod` | all | Go module name (auto-detected from `go.mod` if present) |
| `--service` | `new` | Service name (default: `hertz_service`) |
| `--out_dir` | `new`, `update`, `model` | Project output directory (default: current directory) |
| `--handler_dir` | `new`, `update` | Handler directory relative to `out_dir` (default: `biz/handler`) |
| `--model_dir` | all | Model directory relative to `out_dir` (default: `biz/model`) |
| `--router_dir` | `new` | Router directory relative to `out_dir` (default: `biz/router`) |
| `--client_dir` | `new`, `update`, `client` | Client output directory. For `new`/`update`: no client code if omitted. For `client`: defaults to IDL-derived path |
| `--force_client_dir` | `client` | Client output directory without IDL namespace subdirectories |
| `--use` | `new`, `update`, `client` | Import models from an external package instead of generating them |

### Code Generation

| Flag | Commands | Description |
|------|----------|-------------|
| `--handler_by_method` | `new`, `update` | Generate a separate handler file per method (default: one file per service) |
| `--sort_router` | `new`, `update` | Sort router registration code for deterministic output |
| `--no_recurse` | all | Generate only the master IDL model, skip included/imported dependencies |
| `--force`, `-f` | `new` | Force overwrite an existing project |
| `--force_client` | `client` | Force regenerate `hertz_client.go` even if it already exists |
| `--base_domain` | `client` | Default request domain for generated client code |
| `--enable_extends` | `new`, `update`, `client` | Parse `extends` keyword in Thrift IDL |

### Struct Tags

| Flag | Commands | Description |
|------|----------|-------------|
| `--snake_tag` | all | Use snake_case for `form`, `query`, and `json` tags |
| `--json_enumstr` | all | Use string values instead of numbers for JSON enum fields (Thrift only) |
| `--unset_omitempty` | all | Remove `omitempty` from generated struct tags |
| `--pb_camel_json_tag` | all | Use camelCase for JSON tags (Protobuf only) |
| `--rm_tag` | all | Remove a default tag (e.g. `--rm_tag json`). Explicitly annotated tags are kept |
| `--query_enumint` | `client` | Use numeric values for enum query parameters in client code |
| `--enable_optional` | `client` | Omit optional Thrift fields from query if not set |

### IDL Compiler Options

| Flag | Commands | Description |
|------|----------|-------------|
| `--proto_path`, `-I` | all | Add an include search path for Protobuf imports |
| `--thriftgo`, `-t` | all | Pass-through arguments to thriftgo (e.g. `-t naming_style=golint`) |
| `--protoc`, `-p` | all | Pass-through arguments to protoc |
| `--thrift-plugins` | `new`, `update`, `client` | Additional thriftgo plugins (`{name}:{options}`) |
| `--protoc-plugins` | `new`, `update`, `client` | Additional protoc plugins (`{name}:{options}:{out_dir}`) |
| `--option_package`, `-P` | `new`, `update` | Map IDL include path to Go import path (`{include}={import}`) |
| `--trim_gopackage`, `--trim_pkg` | all | Trim prefix from protobuf `go_package` to avoid deeply nested directories |

### Template Customization

| Flag | Commands | Description |
|------|----------|-------------|
| `--customize_layout` | `new` | Path to custom layout template YAML |
| `--customize_layout_data_path` | `new` | Path to JSON data file for rendering layout templates |
| `--customize_package` | `new`, `update`, `client` | Path to custom package template YAML (overrides handler/router/middleware templates) |
| `--exclude_file`, `-E` | all | Exclude a file path from being generated/updated |

## Custom Templates

Create a YAML config file and pass it with `--customize_package`:

```yaml
layouts:
  - path: biz/handler/handler.go        # overrides default handler template
    delims: ["{{", "}}"]
    body: |
      package {{.PackageName}}
      // your custom handler template...

  - path: biz/custom/{{.ServiceName}}.go  # new file, path supports templates
    delims: ["{{", "}}"]
    loop_service: true                     # generate one file per service
    update_behavior:
      type: append                         # on update: skip/cover/append
      append_key: method                   # append by method or service
      append_content_tpl: |
        // new method: {{.Name}}
    body: |
      package custom
      // your template...
```

Template update behaviors:
- `skip` — do not modify existing file
- `cover` — overwrite existing file completely
- `append` — append new content (e.g. new handler methods) to existing file

## Example Output

Run `generate.sh` to see what hz generates for the test IDL files:

```bash
cd cmd/hz

# Generate examples for all IDL types (output in generate_out/)
./generate.sh

# Generate specific targets only
./generate.sh thrift proto3

# CI mode: generate, verify the output compiles, then clean up
./generate.sh --verify --clean
```

Targets: `thrift`, `proto2`, `proto3`, `handler_by_method`.

## Dependencies

- [thriftgo](https://github.com/cloudwego/thriftgo) — Thrift compiler (auto-installed if missing)
- [protoc](https://github.com/protocolbuffers/protobuf) — Protobuf compiler (must be installed manually)

## Architecture

See [DESIGN.md](DESIGN.md) for internal architecture, the dual-mode execution model, and the code generation pipeline.
