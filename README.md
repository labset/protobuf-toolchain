## protobuf-toolchain

[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=labset_go-protobuf-toolchain-template&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=labset_go-protobuf-toolchain-template)

Template repository for building protobuf toolchains (protoc plugins) in Go.

### using this template

This template uses `labset` as the GitHub org. After creating your repo, replace it with your own:

```bash
grep -rl labset . --exclude-dir=.git | xargs sed -i '' 's/labset/YOUR_ORG/g'   # sed -i on Linux
```

This covers the Go module path and imports, the Homebrew tap owner, and the install commands below.
Run `go mod tidy` afterwards. If you also rename the repo, update `go-protobuf-toolchain-template`
to match.

### usage

- install it with Homebrew

```bash
brew install labset/tap/go-protobuf-toolchain-template
```

This installs both `protoc-gen-echo` and `echo-lint-plugin`.

- install it with `go install`

```bash
go install github.com/labset/go-protobuf-toolchain-template/cmd/protoc-gen-echo@latest
go install github.com/labset/go-protobuf-toolchain-template/cmd/echo-lint-plugin@latest
```

- install it with mise (via the `go` backend)

```bash
mise use "go:github.com/labset/go-protobuf-toolchain-template/cmd/protoc-gen-echo@latest"
mise use "go:github.com/labset/go-protobuf-toolchain-template/cmd/echo-lint-plugin@latest"
```

or pin them in a project's `mise.toml`:

```toml
[tools]
"go:github.com/labset/go-protobuf-toolchain-template/cmd/protoc-gen-echo" = "latest"
"go:github.com/labset/go-protobuf-toolchain-template/cmd/echo-lint-plugin" = "latest"
```

## Development

### requirements

- [mise](https://mise.jdx.dev/) manages `go`, `buf`, `golangci-lint` and `goreleaser` versions (see `.config/mise/conf.d`)

```bash
mise install
```

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
release, and pushes a Homebrew formula to the tap.

**One-time setup** (required before the first release publishes the Homebrew tap):

1. Create the tap repository `labset/homebrew-tap` (an empty repo is fine). GoReleaser commits the
   generated formula into it.
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
