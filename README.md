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

- **adding a generator**: implement `codegen.Generator` and register its `mode`
  in `internal/codegen/modes.go`.
- **adding a lint rule**: add a `check.RuleSpec` to `rules.All` in
  `internal/rules/checks.go`.

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
