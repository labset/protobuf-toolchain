## `mode=proto-service`

Turns a `ROLE_ENTITY` message with a non-empty `operations` list into a CRUD
service split across proto files — one payload file per operation plus the
service definition.

> Assumes a `ROLE_ENTITY` message like the `Project` in the
> [README](../README.md#generators).

```yaml
# buf.gen.yaml
plugins:
  - local: protoc-gen-labset
    out: gen
    opt: mode=proto-service
```

### output

For the annotated `Project` (full CRUD) it emits, per entity:

```
gen/projectmanagement/v1/rpc_create_project.proto   # CreateProjectRequest/Response
gen/projectmanagement/v1/rpc_read_project.proto     # GetProjectRequest (id validated as uuid)
gen/projectmanagement/v1/rpc_update_project.proto   # UpdateProjectRequest + update_mask
gen/projectmanagement/v1/rpc_delete_project.proto
gen/projectmanagement/v1/rpc_list_project.proto     # ListProjectCollectionRequest + read_mask
gen/projectmanagement/v1/service_project.proto      # ProjectService
```

### conventions

- **Method names** — `READ` → `Get<Entity>`; `LIST` → `List<Entity>Collection`
  with a `repeated <Entity> items` response. Other operations are `<Op><Entity>`.
- **Payload fields** — the single-entity payload field is `item` everywhere
  (create/read/update); the list response field is `items`.
- **Field masks** — `UPDATE` carries a `google.protobuf.FieldMask update_mask`;
  `LIST` carries a `read_mask`.
- **Validation** — bare `id` request fields (`READ`/`DELETE`) validate as UUID.

### consumer dependency

The payloads use `google.protobuf.FieldMask` and validate `id` with
[`protovalidate`](https://buf.build/bufbuild/protovalidate), so a module that
compiles the generated protos needs the dependency:

```yaml
# buf.yaml (consumer of the generated protos)
version: v2
deps:
  - buf.build/bufbuild/protovalidate
```

```bash
buf dep update
```
