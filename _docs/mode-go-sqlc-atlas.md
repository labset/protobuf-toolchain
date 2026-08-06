## `mode=go-sqlc-atlas`

Turns every `ROLE_ENTITY` message into a Postgres backend layer: a schema, sqlc
CRUD queries, and the sqlc/Atlas config plus a `generate.go` that drives them.

> Assumes a `ROLE_ENTITY` message like the `Project` in the
> [README](../README.md#generators).

```yaml
# buf.gen.yaml
plugins:
  - local: protoc-gen-labset
    out: gen
    opt: mode=go-sqlc-atlas
```

### output

Entities sharing an output directory aggregate into one schema and config set,
with one query file per entity:

```
gen/projectmanagement/v1/sql/schema.sql          # CREATE SCHEMA + CREATE TABLE per entity
gen/projectmanagement/v1/sql/queries/project.sql # sqlc CRUD queries, one file per entity
gen/projectmanagement/v1/sqlc.yaml               # engine postgresql, pgx/v5, gofrs/uuid overrides
gen/projectmanagement/v1/atlas.hcl               # local env (dev docker db + DATABASE_URL)
gen/projectmanagement/v1/generate.go             # //go:generate atlas + sqlc
```

### conventions

- **Naming** — tables and columns are singular `snake_case`.
- **Identity & lifecycle** — the embedded `Entity` contributes `id uuid PRIMARY KEY`
  (application-generated), `created_at` / `updated_at` (default `now()`), and a
  nullable `deleted_at`.
- **Soft delete** — `DELETE` sets `deleted_at = now()`; reads carry
  `WHERE deleted_at IS NULL`.
- **Partial update** — `UPDATE` uses `COALESCE(sqlc.narg('col'), col)`, so a
  `nil` argument leaves that column unchanged.
- **Named parameters** — every query uses named `sqlc.arg` / `sqlc.narg`, never
  positional `$n`.
- **Explicit SQL** — queries list explicit columns (no `SELECT *` / `RETURNING *`)
  and order with an explicit `ASC`.

### column types

| proto | Postgres |
| --- | --- |
| `string` | `text` |
| `bool` | `boolean` |
| `int32` | `integer` |
| `int64` | `bigint` |
| `float` / `double` | `real` / `double precision` |
| `google.protobuf.Timestamp` | `timestamptz` |

Fields with presence (proto3 `optional`, message types, `oneof` members) are
nullable. Foreign keys (non-`Timestamp` message fields) and repeated fields are
skipped for now.

### schema

Tables live under a dedicated Postgres schema (`CREATE SCHEMA IF NOT EXISTS ...`
plus schema-qualified DDL and queries). It defaults to the proto package with
dots replaced by underscores (`projectmanagement.v1` → `projectmanagement_v1`),
overridable with `schema=<name>`.

### migrations

By default Atlas manages the schema declaratively (`atlas schema apply` against
`schema.sql`). Pass `migration=<format>` to switch to a versioned migrations
directory instead — the value is the Atlas dir format (`goose`, `flyway`,
`golang-migrate`, `dbmate`, `liquibase`, `atlas`).

### parameters

| parameter | default | effect |
| --- | --- | --- |
| `schema` | derived from the proto package | Postgres schema the tables live under |
| `migration` | declarative (`schema apply`) | Atlas migration directory format |

They compose:

```yaml
    opt: mode=go-sqlc-atlas,schema=app,migration=goose
```
