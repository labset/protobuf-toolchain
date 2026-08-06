# Contributing

Thanks for contributing to protobuf-toolchain. This guide covers local setup and
the project's development workflow.

## requirements

- [mise](https://mise.jdx.dev/) manages `go`, `buf`, `golangci-lint` and `goreleaser` versions (see `.config/mise/conf.d`)

```bash
mise install
```

## layout

```
cmd/protoc-gen-labset    # codegen plugin entrypoint
cmd/labset-lint-plugin   # lint plugin entrypoint
internal/codegen         # generators + mode dispatch (GeneratorForMode)
internal/rules           # lint rule specs (rules.All)
protos                   # proto sources (schema module)
```

## adding a generator

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
- **docs** — add a `_docs/<mode>.md` with the mode's output layout, conventions
  and parameters, and link it from the generators table in the README. Keep the
  README a lean index; detail lives in `_docs`.

## adding a lint rule

Register a `check.RuleSpec` in `rules.All` (`internal/rules/checks.go`);
`labset-lint-plugin` serves whatever is listed there. Give each rule its own
`check_<name>.go` file (mirroring `generator_<mode>.go`) — see
`internal/rules/check_entity_embedded_field.go`, which uses
`checkutil.NewMessageRuleHandler` to walk every message and
`responseWriter.AddAnnotation` to report a violation. Each rule needs a unique
uppercase `ID`, a `Purpose` ending in a period, and `Type: check.RuleTypeLint`.

- **fixtures** — proto inputs live under `internal/rules/golden/<check>/<case>/`,
  compiled with `checktest`. Their labset imports resolve against the real
  `protos` module (not copies) via the fixture's `DirPaths`.
- **tests** — one `check_<name>_test.go` per rule; a table of cases runs the rule
  over its fixtures and asserts which annotations fire. `TestSpec` validates every
  registered spec.

## housekeeping tasks

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

- format the protos and the Go code

```bash
mise run format
```

- auto-fix lint issues

```bash
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

## releasing

Releases are cut by [GoReleaser](https://goreleaser.com/) from the `release` workflow, which
triggers when a GitHub release is created. It builds the binaries and publishes the archives to the
release. Consumers install the binaries via `go install` or mise (see the README).

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
