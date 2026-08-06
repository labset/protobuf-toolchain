# protobuf-toolchain

[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=labset_protobuf-toolchain&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=labset_protobuf-toolchain)

A toolchain for working with Protocol Buffers in Go. It ships two `buf`-compatible
plugins:

- **`protoc-gen-labset`** — a `protoc` codegen plugin. It selects a generator by
  `mode` parameter, so a single binary can host several code generators.
- **`labset-lint-plugin`** — a [`bufplugin`](https://buf.build/docs/cli/buf-plugins/)
  check plugin that contributes custom lint rules to `buf lint`.

## usage

- install with `go install`

```bash
go install github.com/labset/protobuf-toolchain/cmd/protoc-gen-labset@latest
go install github.com/labset/protobuf-toolchain/cmd/labset-lint-plugin@latest
```

- install with mise (via the `go` backend)

```bash
mise use "go:github.com/labset/protobuf-toolchain/cmd/protoc-gen-labset@latest"
mise use "go:github.com/labset/protobuf-toolchain/cmd/labset-lint-plugin@latest"
```

or pin them in a project's `mise.toml`:

```toml
[tools]
"go:github.com/labset/protobuf-toolchain/cmd/protoc-gen-labset" = "latest"
"go:github.com/labset/protobuf-toolchain/cmd/labset-lint-plugin" = "latest"
```

## wiring the plugins into buf

- codegen with `protoc-gen-labset`, selecting a generator with the `mode` option:

```yaml
# buf.gen.yaml
version: v2
plugins:
  - local: protoc-gen-labset
    out: gen
    opt: mode=proto-service
```

- linting with `labset-lint-plugin`:

```yaml
# buf.yaml
version: v2
plugins:
  - plugin: labset-lint-plugin
lint:
  use:
    - STANDARD
```

## generators

`protoc-gen-labset` hosts several generators behind the `mode` option. Most are
driven by the `(labset.plugin.v1.message)` annotation — a `role` plus the CRUD
`operations` to expose:

```protobuf
// projectmanagement/v1/project.proto
syntax = "proto3";
package projectmanagement.v1;

import "labset/plugin/v1/entity.proto";
import "labset/plugin/v1/options.proto";

message Project {
  option (labset.plugin.v1.message) = {
    role: ROLE_ENTITY
    operations: [OPERATION_CREATE, OPERATION_READ, OPERATION_UPDATE, OPERATION_DELETE, OPERATION_LIST]
  };

  labset.plugin.v1.Entity entity = 1;
  string name = 2;
}
```

| `mode` | generates | docs |
| --- | --- | --- |
| `proto-service` | a CRUD service split across proto files, per annotated entity | [read the docs](_docs/mode-proto-service.md) |
| `go-sqlc-atlas` | a Postgres schema, sqlc queries and the sqlc/Atlas config, per annotated entity | [read the docs](_docs/mode-go-sqlc-atlas.md) |

## lint rules

`labset-lint-plugin` contributes rules that keep entity annotations well-formed.
They are on by default once the plugin is wired into `buf.yaml`:

- **`LABSET_ENTITY_ANNOTATION_ROOT_ONLY`** — the `(labset.plugin.v1.message)`
  role/operations annotation may only be applied to a top-level message. A nested
  message cannot be referenced by an unqualified name from generated files, so the
  codegen silently skips it; this rule surfaces the misplacement instead.
- **`LABSET_ENTITY_EMBEDDED_FIELD`** — a `ROLE_ENTITY` message must embed
  `labset.plugin.v1.Entity` at field number 1. `Entity` carries the shared `id`
  and the `created_at` / `updated_at` / `deleted_at` lifecycle timestamps:

```protobuf
message Project {
  option (labset.plugin.v1.message) = {
    role: ROLE_ENTITY
    operations: [OPERATION_CREATE]
  };

  labset.plugin.v1.Entity entity = 1; // id + lifecycle timestamps
  string name = 2;
}
```

## contributing

Local setup, project layout, how to add a generator or lint rule, and the release
process live in the [contributing guide](CONTRIBUTING.md).
