# testing locally against a project

How to try a working-tree build of the plugins against a real project before
cutting a release. `buf` resolves `local: protoc-gen-labset` (and the
`labset-lint-plugin`) off `PATH`, so the trick is to put your local build there
in place of any released copy.

## install from your checkout

`go install` from the toolchain checkout builds the current working tree and
drops the binaries in `GOBIN`, which mise already keeps on `PATH`:

```bash
# from the protobuf-toolchain checkout
mise exec -- go install ./cmd/protoc-gen-labset ./cmd/labset-lint-plugin
```

This lands the working-tree binaries in the mise Go toolchain's `GOBIN`, so the
next `buf` run picks up your changes. Confirm they resolve there and not to some
other copy on `PATH` — if you previously installed via `mise use "go:..."`, that
binary lives in a separate mise dir, so check which one wins:

```bash
which protoc-gen-labset labset-lint-plugin
```

> Uncommitted changes are included — `go install` builds the working tree, not a
> tagged version. No need to commit or tag to test.

## point a project at it

In the target project, wire the plugins exactly as a consumer would (see the
[README](../README.md)) — codegen in `buf.gen.yaml`, lint in `buf.yaml`. Nothing
here is local-only; it's the same config a released install uses:

```yaml
# buf.gen.yaml (target project)
version: v2
plugins:
  - local: protoc-gen-labset
    out: gen
    opt: mode=proto-service   # or go-sqlc-atlas, with its params
```

```yaml
# buf.yaml (target project)
version: v2
modules:
  - path: proto   # the project's own protos
plugins:
  - plugin: labset-lint-plugin
lint:
  use:
    - STANDARD
```

The project's entities need the `(labset.plugin.v1.message)` annotation and the
embedded `labset.plugin.v1.Entity`, which come from this module's
`protos/labset/plugin/v1` (`options.proto`, `entity.proto`, `enums.proto`).
This module isn't published to a registry yet, and a buf v2 workspace can't
reference protos outside its root, so vendor those three files into the target
project — copy them under `proto/labset/plugin/v1/` so that
`import "labset/plugin/v1/options.proto";` resolves within the project's module.

## generate and inspect

```bash
buf lint                                     # exercises labset-lint-plugin
buf generate                                 # runs protoc-gen-labset (default buf.gen.yaml)
buf generate --template buf.gen.labset.yaml  # if you keep the toolchain template separate
```

Then read the emitted tree (`gen/...`) and check it against the mode's
documented output — see the [proto-service docs](mode-proto-service.md) and the
[go-sqlc-atlas docs](mode-go-sqlc-atlas.md). For `go-sqlc-atlas`, take the layer
the rest of the way to confirm it's usable downstream:

```bash
cd gen/<package>/v1 && go generate   # drives atlas + sqlc
```

## iterate

After each change to the toolchain, reinstall and rerun generate in the target
project:

```bash
mise exec -- go install ./cmd/protoc-gen-labset ./cmd/labset-lint-plugin
# back in the target project
buf generate
```

No fixture over the real thing beats this loop, but the golden tests
(`mise run test`) are the faster inner loop while shaping output — reach for a
real project to validate the whole path (annotation → codegen → `buf`/`sqlc`
compiling the result).

## restore the released build

When you're done, replace the working-tree binaries with a published version so
the project isn't left on an unreleased build:

```bash
mise exec -- go install github.com/labset/protobuf-toolchain/cmd/protoc-gen-labset@latest
mise exec -- go install github.com/labset/protobuf-toolchain/cmd/labset-lint-plugin@latest
# or, if pinned via mise in the project
mise install
```
