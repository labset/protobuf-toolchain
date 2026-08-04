## protobuf-toolchain

[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=labset_protobuf-toolchain&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=labset_protobuf-toolchain)

A toolchain for working with Protocol Buffers in Go. It ships two `buf`-compatible
plugins:

- **`protoc-gen-labset`** — a `protoc` codegen plugin. It selects a generator by
  `mode` parameter, so a single binary can host several code generators.
- **`labset-lint-plugin`** — a [`bufplugin`](https://buf.build/docs/cli/buf-plugins/)
  check plugin that contributes custom lint rules to `buf lint`.

### usage

- install with Homebrew (installs both binaries)

```bash
brew install labset/tap/protobuf-toolchain
```

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

import "labset/plugin/v1/options.proto";

message Project {
  option (labset.plugin.v1.message) = {
    role: ROLE_ENTITY
    operations: [OPERATION_CREATE, OPERATION_READ, OPERATION_UPDATE, OPERATION_DELETE, OPERATION_LIST]
  };

  string id = 1;
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

## Development

### requirements

- [mise](https://mise.jdx.dev/) manages `go`, `buf`, `golangci-lint` and `goreleaser` versions (see `.config/mise/conf.d`)

```bash
mise install
```

### layout

```
cmd/protoc-gen-labset    # codegen plugin entrypoint
cmd/labset-lint-plugin   # lint plugin entrypoint
internal/codegen         # generators + mode dispatch (GeneratorForMode)
internal/rules           # lint rule specs (rules.All)
protos                   # proto sources (schema module)
```

### adding a generator

Implement `codegen.Generator` — a single `Generate(*protogen.Plugin) error` — and
register it by `mode`:

```go
// internal/codegen/modes.go
case "proto-service":
    return &protoServiceGenerator{}, nil
```

Keep the Go side to discovering _what_ to emit; let templates own the output shape.

- **templates** — embed `internal/codegen/templates/<mode>/*.tmpl`:

```go
//go:embed templates/proto-service/*.tmpl
var protoServiceTemplateFS embed.FS
```

- **golden files** — expected output under `internal/codegen/golden/<mode>/`. The
  test builds an in-memory `pluginpb.CodeGeneratorRequest`, runs the generator,
  and diffs each generated file against its golden. Regenerate after an
  intentional change:

```bash
mise exec -- go test ./internal/codegen/ -update
```

- **tests** — cover the golden output plus the generator's skip/error states; see
  `internal/codegen/generator_proto_service_test.go`.

### adding a lint rule

Add a `check.RuleSpec` to `rules.All` in `internal/rules/checks.go`;
`labset-lint-plugin` serves whatever is registered there.

### housekeeping tasks

- list available tasks

```bash
mise tasks
```

- generate code from proto files

```bash
mise run generate
```

- lint the protos and the Go code

```bash
mise run lint
```

- format and auto-fix

```bash
mise run schema:lint:fix
mise run toolchain:lint:fix
```

- tidy dependencies

```bash
mise run schema:tidy      # buf dep update
mise run toolchain:tidy   # go mod tidy
```

- build the binaries

```bash
mise run build
```

- clean build artifacts

```bash
mise run clean
```

### releasing

Releases are cut by [GoReleaser](https://goreleaser.com/) from the `release` workflow, which
triggers when a GitHub release is created. It builds the binaries, publishes the archives to the
release, and pushes a Homebrew cask to the tap.

**One-time setup** (required before the first release publishes the Homebrew tap):

1. Create the tap repository `labset/homebrew-tap` (an empty repo is fine). GoReleaser commits the
   generated cask into it.
2. Add a `HOMEBREW_TAP_GITHUB_TOKEN` repository secret, a GitHub personal access token with
   `contents: write` permission on `labset/homebrew-tap`. The default `GITHUB_TOKEN` cannot push to
   another repository, so this separate token is required.

**Cutting a release:**

```bash
git tag v0.1.0
git push origin v0.1.0
# then create a GitHub release for the tag (this triggers the release workflow)
```

To validate the release pipeline locally without publishing:

```bash
mise run build   # goreleaser build --clean --snapshot
```
