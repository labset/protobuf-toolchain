## protobuf-toolchain

[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=labset_protobuf-toolchain&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=labset_protobuf-toolchain)

A toolchain for working with Protocol Buffers in Go. It ships two `buf`-compatible
plugins:

- **`protoc-gen-labset`** — a `protoc` codegen plugin. It selects a generator by
  `mode` parameter, so a single binary can host several code generators.
- **`labset-lint-plugin`** — a [`bufplugin`](https://buf.build/docs/cli/buf-plugins/)
  check plugin that contributes custom lint rules to `buf lint`.

### usage

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

### wiring the plugins into buf

- codegen with `protoc-gen-labset`, selecting a generator with the `mode` option:

```yaml
# buf.gen.yaml
version: v2
plugins:
  - local: protoc-gen-labset
    out: gen
    opt: mode=echo
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

### generators

#### `mode=proto-service`

Annotate an entity with `(labset.plugin.v1.message)` — a `role` and the CRUD
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

```yaml
# buf.gen.yaml
plugins:
  - local: protoc-gen-labset
    out: gen
    opt: mode=proto-service
```

Emits one payload file per operation plus the service:

```
gen/projectmanagement/v1/rpc_create_project.proto   # CreateProjectRequest/Response
gen/projectmanagement/v1/rpc_read_project.proto     # GetProjectRequest (id validated as uuid)
gen/projectmanagement/v1/rpc_update_project.proto   # UpdateProjectRequest + update_mask
gen/projectmanagement/v1/rpc_delete_project.proto
gen/projectmanagement/v1/rpc_list_project.proto     # ListProjectCollectionRequest + read_mask
gen/projectmanagement/v1/service_project.proto      # ProjectService
```

The payloads use `google.protobuf.FieldMask` and validate `id` with
[`protovalidate`](https://buf.build/bufbuild/protovalidate), so consumers that
compile the output need the dependency:

```yaml
# buf.yaml (consumer of the generated protos)
version: v2
deps:
  - buf.build/bufbuild/protovalidate
```

```bash
buf dep update
```

#### `mode=go-sqlc-atlas`

Turns every `ROLE_ENTITY` message into a Postgres backend layer: a schema, sqlc
CRUD queries, and the sqlc/Atlas config plus a `generate.go` that drives them.
Entities sharing an output directory are aggregated into one schema and query
file plus one config set.

```yaml
# buf.gen.yaml
plugins:
  - local: protoc-gen-labset
    out: gen
    opt: mode=go-sqlc-atlas
```

For the `Project` entity above it emits, per package directory:

```
gen/projectmanagement/v1/sql/schema.sql          # CREATE TABLE project (id uuid PK, ..., soft delete)
gen/projectmanagement/v1/sql/queries/project.sql # sqlc CRUD queries, one file per entity
gen/projectmanagement/v1/sqlc.yaml               # engine postgresql, pgx/v5, gofrs/uuid overrides
gen/projectmanagement/v1/atlas.hcl               # local env (dev docker db + DATABASE_URL)
gen/projectmanagement/v1/generate.go             # //go:generate atlas + sqlc
```

The schema aggregates every entity in the directory; each entity gets its own
`sql/queries/<table>.sql`.

Conventions: table and column names are singular `snake_case`; the embedded
`Entity` contributes `id uuid PRIMARY KEY` (application-generated) plus
`created_at` / `updated_at` (default `now()`) and a nullable `deleted_at`;
`DELETE` is a soft delete and reads carry `WHERE deleted_at IS NULL`. Scalar
fields map by type (`string`→`text`, `int64`→`bigint`, `bool`→`boolean`,
`Timestamp`→`timestamptz`; proto3 `optional` → nullable); message fields other
than `Timestamp` (foreign keys) are skipped for now.

By default Atlas manages the schema declaratively (`atlas schema apply` against
`schema.sql`). Pass `migration=<format>` to switch to a versioned migrations
directory instead — the value is the Atlas dir format (`goose`, `flyway`,
`golang-migrate`, `dbmate`, `liquibase`, `atlas`):

```yaml
    opt: mode=go-sqlc-atlas,migration=goose
```

### lint rules

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

## Contributing

Local setup, project layout, how to add a generator or lint rule, and the release
process live in [CONTRIBUTING.md](CONTRIBUTING.md).
