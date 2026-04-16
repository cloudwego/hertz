# hz Internal Design

This document describes the internal architecture of `hz` for contributors and maintainers. For usage, see [README.md](README.md).

## Dual-Mode Execution

`hz` runs in two distinct modes:

1. **CLI Mode** (normal): Parses arguments, generates project layout, then invokes the IDL compiler (thriftgo/protoc) as a subprocess.
2. **Plugin Mode**: When invoked *by* the IDL compiler as a plugin (`thrift-gen-hertz` or `protoc-gen-hertz`), it reads the parsed AST from stdin and generates Hertz-specific code.

This means a single `hz new` command actually runs `hz` twice:
```
User -> hz (CLI mode) -> thriftgo/protoc -> hz (Plugin mode) -> generated code
```

The plugin mode is detected via the `HERTZ_PLUGIN_MODE` environment variable, set by the CLI before invoking the compiler.

## Package Structure

```
cmd/hz/
├── app/          # CLI commands (new/update/model/client) and plugin triggering
├── config/       # Argument parsing, validation, and compiler command construction
├── generator/    # Code generation engine (handlers, routers, models, clients, templates)
│   └── model/    # IDL-to-Go type system and Go code backend
├── thrift/       # Thrift IDL plugin: AST conversion, type resolution, annotation extraction
├── protobuf/     # Protobuf IDL plugin (parallel structure to thrift/)
├── meta/         # Constants, version info, and `.hz` manifest management
└── util/         # Shared utilities (string ops, file helpers, env detection, logging)
```

## Code Generation Pipeline

### 1. Layout Generation (`new` command only)

Creates the project skeleton:
- `main.go` with Hertz server bootstrap
- `go.mod` with required dependencies
- `router.go` with route registration entry point
- Directory structure: `biz/handler/`, `biz/model/`, `biz/router/`

### 2. Plugin Execution

The IDL compiler parses the `.thrift`/`.proto` file and passes the AST to `hz` running in plugin mode. The plugin:

1. **Converts** the IDL AST into internal `Service` and `HttpMethod` structs, extracting HTTP annotations (paths, methods, serializers)
2. **Resolves** type references across IDL files into Go import paths
3. **Generates** code via the `HttpPackageGenerator`

### 3. Handler Generation

Two modes controlled by `--handler_by_method`:

- **By service** (default): All handlers for a service go into one file (e.g., `user_service.go`). On `update`, new methods are appended using the `handler_single.go` template.
- **By method**: Each handler gets its own file (e.g., `create_user.go`). On `update`, new files are created but existing ones are never modified.

### 4. Router Generation

Routes are organized as a tree (`RouterNode`), built by inserting each method's HTTP path. The tree is then:

1. Traversed to assign unique middleware group names (`DyeGroupName`)
2. Rendered into Go route registration code with `r.Group()`/`r.GET()`/etc.
3. Middleware stubs are generated for each route group

### 5. Template System

All generated code is rendered from Go templates. Templates support:

- **Custom delimiters**: Override `{{ }}` if your template contains Go template syntax
- **Update behaviors**: `skip` (don't touch existing), `cover` (regenerate), `append` (add new content)
- **Loop modes**: Generate one file per service (`loop_service`) or per method (`loop_method`)
- **Custom templates**: Override any default template via `--customize_package`

## Manifest File (`.hz`)

The `.hz` YAML file in the project root tracks:
- hz version used to generate the project
- Handler, model, and router directory paths

This allows `hz update` to locate existing generated code without requiring the user to re-specify all flags.

## Argument Passing Between CLI and Plugin

Since the plugin runs as a child process of the IDL compiler (not `hz` directly), CLI arguments are serialized into a comma-separated string via reflection (`util.PackArgs`) and passed through compiler plugin options. The plugin deserializes them back with `util.UnpackArgs`.

Format: `FieldName=value,SliceField=val1;val2;val3,MapField=k1=v1;k2=v2`
