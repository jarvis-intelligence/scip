<!-- GSD:project-start source:PROJECT.md -->

## Project

**scip-swift**

This repo is a fork of the SCIP ("skip") protocol monorepo — the Protobuf code-intelligence
schema, its Go CLI and bindings, and five generated language bindings — extended with
**scip-swift**, a new in-repo Swift indexer that emits SCIP indexes. It exists so that
jarvis (a local-first code-intelligence MCP server, distributed via
jarvis-intelligence/jarvis-index) can answer structural queries — goToDefinition,
findReferences, callHierarchy, typeHierarchy, documentSymbols — on Swift codebases.

**Core Value:** scip-swift produces accurate, `scip lint`-clean SCIP indexes for real Swift projects,
powering working code navigation in jarvis on macOS.

### Constraints

- **Tech stack**: Swift + SourceKit-LSP for semantics; tree-sitter (Swift grammar) for
  fallback — hybrid decided during questioning

- **Platform**: macOS arm64 + x86_64 for v1, full semantics on both
- **Integration**: Emitted index must pass `scip lint` and follow SCIP symbol-format
  conventions documented in `scip.proto`

- **Environment**: The repo's canonical toolchain is Nix (`nix develop`, flake checks);
  Swift toolchain integration must fit the flake/check matrix or justify a CI exception

- **Compatibility**: Must not break existing modules (Go workspace, bindings codegen,
  reprolang tests)
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->

## Technology Stack

## Overview

## Languages

- Go 1.25.0 - Three-module Go workspace (`go.work`): root module `github.com/scip-code/scip` (the CLI in `cmd/scip/`), library module `github.com/scip-code/scip/bindings/go/scip` (hand-written helpers + generated `bindings/go/scip/scip.pb.go`), and test-indexer module `github.com/scip-code/scip/reprolang`
- Protocol Buffers (proto3) - `scip.proto` is the product itself: the SCIP schema (~35KB, package `scip`)
- Rust 2021 edition (MSRV 1.81.0) - `bindings/rust/Cargo.toml`; hand-written wrappers over generated `bindings/rust/src/generated/`
- TypeScript (ESM) - `bindings/typescript/` (`@scip-code/scip` npm package); generated `scip_pb.ts` compiled with `tsc`
- Haskell (GHC2021; tested GHC 9.4.8–9.10.3) - `bindings/haskell/scip.cabal`; proto-lens generated
- Java 11 - `bindings/java/pom.xml` (`org.scip-code:scip-java-bindings`); protoc-built-in generated code
- Kotlin 2.0.21 (pinned `<2.1.0` by Renovate rule) - `bindings/kotlin/pom.xml` (`org.scip-code:scip-kotlin-bindings`)
- Tree-sitter grammar DSL (JavaScript) - `reprolang/grammar.js`, generated C parser checked in at `reprolang/grammar/parser.c` (ABI 14)
- Nix - `flake.nix`, `checks.nix` (declarative CI/check matrix)
- Bash - `reprolang/generate-tree-sitter-parser.sh`
- Yaml/Json - GitHub workflows, Renovate config

## Runtime

- Go toolchain 1.25.0 (workspace `go.work` spans `.` + `bindings/go/scip` + `reprolang`; per-module builds set `GOWORK=off` in Nix/CI)
- Node.js - required only for tree-sitter parser generation and TypeScript binding build (`bindings/typescript/package.json`)
- Nix (nixos-26.05 channel, `flake.lock`) - canonical entry point: `nix run .#proto-generate`, `nix develop` provides go, cargo, rustc, nodejs, tree-sitter, cabal, ghc
- CLI binaries are static (`CGO_ENABLED=0` in `release.yaml`) — no libc dependency; SQLite is pure-Go so CGo stays off
- Go modules - `go.mod`/`go.sum` at root, `bindings/go/scip/go.mod`, `reprolang/go.mod`; lockfiles present
- Cargo - `bindings/rust/Cargo.lock` present
- npm - `bindings/typescript/package-lock.json` present
- Cabal - `bindings/haskell/scip.cabal` (no lockfile; built via `callCabal2nix` in `checks.nix`)
- Maven - `bindings/java/pom.xml`, `bindings/kotlin/pom.xml` (built outside Nix because Kotlin resolves Java from Maven Central; see `.github/workflows/jvm-bindings.yaml`)
- Nix flake - `flake.lock` present

## Frameworks

- `github.com/urfave/cli/v3` v3.10.1 - CLI framework for the `scip` binary (`cmd/scip/main.go`); subcommands: `lint`, `print`, `snapshot`, `stats`, `test`, `expt-convert`
- `google.golang.org/protobuf` v1.36.12 - Protobuf runtime for Go library and CLI
- Buf - protobuf codegen orchestrator (`buf.yaml` v1 lint/breaking config; `buf.gen.yaml` v2 plugin list)
- `github.com/stretchr/testify` v1.11.1 - assertions across all three Go modules
- `github.com/hexops/autogold/v2` v2.3.1 - golden snapshot testing in `reprolang/` and `cmd/scip` (`go test ./cmd/scip -update-snapshots`)
- `pgregory.net/rapid` v1.3.0 - property-based testing in `bindings/go/scip` (e.g. `symbol_parser.go` roundtrips)
- `github.com/google/go-fuzz` + `gofuzz` - fuzzing support in `bindings/go/scip`
- `pretty_assertions` 1.4.1 - Rust dev-dependency (`bindings/rust/Cargo.toml`)
- Nix `checks` - every binding builds in CI as a flake check (`checks.nix`), including a `reprolang-generated` check that regenerates the tree-sitter parser and diffs it
- Nix flake (`flake.nix`) - packages: `scip` (Go CLI), `proto-generate` (one-command protobuf regen for ALL bindings), `default`; checks: `github-actions` (action-validator), `formatting` (prettier/buf/gofmt/goimports/nixfmt), `go-bindings`, `haskell-bindings`, `reprolang`, `reprolang-generated`, `rust-bindings`, `typescript-bindings`
- protoc plugin set (pinned in `flake.nix`): `protoc-gen-go`, `protoc-gen-es` (TypeScript), `protoc-gen-rs` (= `protobuf-codegen` crate 3.7.2, built from source in Nix), `proto-lens-protoc` (Haskell), protoc-builtin `java`/`kotlin`, `protoc-gen-doc` (generates `docs/scip.md` via `docs/scip.sprig` template)
- Prettier - repo-wide formatter for ts/js/json/md/yml (`.prettierrc`, `.prettierignore`)
- tree-sitter CLI - `tree-sitter generate --abi 14` for reprolang parser
- Maven - JVM bindings build/publish
- Renovate - dependency automation (`.github/renovate.json`); protobuf-java and Kotlin versions are deliberately managed by hand (packageRules disable/pin them)

## Key Dependencies

- `github.com/urfave/cli/v3` v3.10.1 - CLI surface
- `google.golang.org/protobuf` v1.36.12 - index (de)serialization; note `bindings/go/scip/parse.go` also contains a hand-written fast protobuf varint parser for performance
- `zombiezen.com/go/sqlite` v1.4.2 (modernc pure-Go SQLite) - `scip expt-convert` writes `index.db` (`cmd/scip/convert.go`)
- `github.com/klauspost/compress` v1.19.2 - zstd decompression of indexes (`cmd/scip/print.go`, `convert.go`)
- `github.com/hhatto/gocloc` v0.7.0 - comment/line counting for `scip stats` (`cmd/scip/stats.go`)
- `github.com/montanaflynn/stats` v0.12.3 - percentile/mean/stddev aggregation in `scip stats`
- `github.com/k0kubun/pp/v3` v3.5.2 - pretty-printing index output
- `github.com/sourcegraph/beaut` - symbol formatting used by snapshot output
- `github.com/hexops/gotextdiff` - diffing for snapshot tests
- `github.com/fatih/color` v1.19.0 - colored output (respects https://no-color.org/, see `cmd/scip/print.go`)
- `github.com/tree-sitter/go-tree-sitter` v0.25.0 - CGo bindings to the generated parser (`reprolang/grammar/binding.go`); the only CGo dependency in the repo and it is test-only
- `protobuf` =3.7.2 (exact pin — must match the `protoc-gen-rs` codegen version in `flake.nix`)
- `@bufbuild/protobuf` ^2.11.0 - runtime for protoc-gen-es output
- `com.google.protobuf:protobuf-java`/`protobuf-kotlin` 4.34.2 (pinned to match Nix protoc; Renovate updates disabled)

## Configuration

- No runtime config files or env vars for the CLI — all configuration is CLI flags (`--from`, `--project-root`, `--comment-syntax`, `--output`, `--cpu-profile`; defaults in `cmd/scip/main.go`)
- Version single-source-of-truth: `cmd/scip/version.txt` (0.9.0); every binding manifest version must match it and is asserted in CI (`checks.nix` assertions, `jvm-bindings.yaml` xpath check)
- No `.env` files exist in the repo
- `buf.yaml` (lint DEFAULT minus naming exceptions; breaking check `FILE`), `buf.gen.yaml` (six codegen plugins)
- `flake.nix` / `checks.nix` / `flake.lock` - Nix build + check matrix
- `go.work` - Go multi-module workspace
- `.prettierrc` / `.prettierignore` - Prettier (semi: false, singleQuote, es5 trailing commas)
- `.gitattributes`, `.gitignore` - repo hygiene (ignores `vendor/`, `/bindings/typescript/dist/`, `/result`)

## Platform Requirements

- Nix is the only required dependency (`docs/Development.md`); `nix develop` supplies Go, Rust, Node, GHC/cabal, tree-sitter
- Without Nix: Go 1.25+, plus per-binding toolchains only when touching that binding
- macOS (arm64/amd64) or Linux (amd64/arm64) hosts; CI runs `ubuntu-latest` and `macos-latest`
- The `scip` CLI ships as static tarballs for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` via GitHub Releases (`release.yaml`)
- Published artifacts: Go module, npm `@scip-code/scip`, crates.io `scip`, Hackage `scip`, Maven Central `org.scip-code:{scip-java-bindings,scip-kotlin-bindings}`

<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

## Naming Patterns

- Use `snake_case.go` file names, one primary concern per file: `symbol.go`, `symbol_parser.go`, `symbol_formatter.go`, `occurrence_range.go`, `canonicalize.go` (all in `bindings/go/scip/`).
- Co-locate tests as `<file>_test.go`: `symbol_test.go`, `sort_test.go`, `lint_test.go`.
- Generated protobuf code lives in `scip.pb.go` — never hand-edit; regenerate with `nix run .#proto-generate` (see `flake.nix`).
- Pair a build-tag'd file with a no-op counterpart: `assertions.go` (`//go:build asserts`) and `assertions_noop.go` (`//go:build !asserts`).
- Exported functions are `PascalCase`; use verb-prefixed names that say what they do: `ParseSymbol`, `ParseSymbolUTF8With`, `ValidateSymbolUTF8`, `FormatSnapshots`, `RunTests`, `SortOccurrences`, `FindOccurrences`, `NewSourcesFromDirectory`.
- Constructors use `NewX`: `NewConverter`, `NewRangeUnchecked`, `newSymbolTable`, `newSet`. Note lowercase `newX` for internal constructors (`newSet[T comparable]()`, `newSymbolTable()`).
- Predicates use `IsX`: `IsGlobalSymbol`, `IsLocalSymbol`; `HasPrefix`-style helpers follow stdlib idioms.
- In `cmd/scip/`, follow the three-tier CLI pattern per subcommand: `xxxCommand() cli.Command` (flag wiring), `xxxMain(...)` (I/O orchestration), `xxxMainPure(...)` (testable core without I/O) — see `lintCommand`/`lintMain`/`lintMainPure` in `cmd/scip/lint.go`, and `convertCommand`/`convertMain` in `cmd/scip/convert.go`.
- Use `SourceX`-style getters on protobuf messages: `occ.SourceRange()` returns `(scip.Range, bool)`; `pos.IsSingleLine()`.
- `camelCase`; short receivers are the first letter or short word of the type (`func (f *SymbolFormatter)`, `func (e errorSet)`, `func (s symbolAttributeTestCase)`).
- Group flags into a struct per command: `type testFlags struct { from string; commentSyntax string; ... }` in `cmd/scip/test.go`, `type snapshotFlags struct {...}` in `cmd/scip/snapshot.go`.
- Use `lowerCamel` constants; exported sentinel vars are allowed at package level (`var Reproducible = "" // set by ldflags in CI` in `cmd/scip/main.go`).
- Struct error/warning types named `xxxError` / `xxxWarning` implementing `error`: `emptyStringError`, `nonCanonicalSymbolError`, `duplicateSymbolInfoWarning` in `cmd/scip/lint.go`.
- String-typed enum kinds with typed constants: `type symbolAttributeKind string` with `definitionAttrKind symbolAttributeKind = "definition"` in `bindings/go/scip/testutil/test_runner.go`.
- Prefer type aliases for unwieldy composite types: `type stringMap = map[string][]string` and `type occurrenceMap = map[occurrenceKey]*scip.Occurrence` (in `cmd/scip/lint_test.go` and `cmd/scip/lint.go`), `type indexFunction = func(...)` in `testutil/snapshot_testing.go`.
- Configuration via plain exported-struct options (no functional options): `ParseSymbolOptions{IncludeDescriptors bool, RecordOutput *Symbol}` in `bindings/go/scip/symbol.go`; `SymbolFormatter` in `bindings/go/scip/symbol_formatter.go` uses struct-of-functions for pluggable behavior.
- Visitor via callback fields, not interfaces: `scip.IndexVisitor{VisitDocument: func(...)}` (see `bindings/go/scip/memtest/low_mem_test.go`).
- Generics are welcome for small utilities: `set[T comparable]`, `mySlice[T]`, `setToString[T comparable]` in `cmd/scip/lint.go`; `unwrap[T any]` in `reprolang/repro/test_cmd_test.go`.

## Code Style

- `gofmt` and `goimports` are the authoritative formatters; enforced by the Nix `formatting` check in `checks.nix`:
- Tabs for Go (gofmt default); no `.editorconfig` needed.
- Non-Go files: `prettier --check '**/*{ts,js(on)?,md,yml}'` with `.prettierrc` = `{ "semi": false, "trailingComma": "es5", "singleQuote": true, "useTabs": false, "arrowParens": "avoid" }`; `buf format --diff --exit-code scip.proto` for protobuf; `nixfmt --check *.nix` for Nix files.
- No golangci-lint configuration exists (`Not detected`). Lint enforcement is: gofmt/goimports diffs, `buf lint` (DEFAULT rules with exceptions for `PACKAGE_VERSION_SUFFIX`, `ENUM_VALUE_PREFIX`, `ENUM_VALUE_UPPER_SNAKE_CASE`, `ENUM_ZERO_VALUE_SUFFIX` in `buf.yaml`), `action-validator` for workflows, and version-consistency asserts across `cmd/scip/version.txt` / `bindings/*` manifests in `checks.nix`.
- Protobuf breaking-change detection: `breaking: use: [FILE]` in `buf.yaml` — schema changes must be backward compatible at the file level.
- CI (`.github/workflows/ci.yaml`) matrix-builds every attribute of `checks` and `packages` from `flake.nix`; the `packages` job runs `nix run .#<pkg>` then `git diff --exit-code` so generated code (e.g. `proto-generate`) can never drift.
- Use Go 1.21+ stdlib (`slices`, `sort` replaced by `slices.Sort`, `slices.Contains`, `slices.Equal`) — see `bindings/go/scip/testutil/test_runner.go`.
- Add a section comment banner for long files: `// --- Main types ---`, `// --- All possible errors ---`, `// --- Miscellaneous utility types ---` in `cmd/scip/lint.go`; `// --------------------------------- Utils ---------------------------------` in `test_runner.go`.
- Use build tags for expensive invariants: `assert()` panics under `-tags asserts`, is a no-op otherwise (`bindings/go/scip/assertions.go` / `assertions_noop.go`). Nix builds compile tests with `buildTags = [ "asserts" ]` (`checks.nix`).
- Embed static assets with `//go:embed version.txt` (`cmd/scip/main.go`); inject build metadata via ldflags `-X main.Reproducible=true` (`flake.nix`).

## Import Organization

- Alias the stdlib `errors` package as `stderrors` when a file also uses another errors-heavy import or to be explicit (`cmd/scip/lint.go`); otherwise import `errors` unaliased (`cmd/scip/convert.go`).
- Alias rarely: `fuzz "github.com/google/gofuzz"` in `bindings/go/scip/parse_test.go`.
- None; the bindings module path is the canonical import path `github.com/scip-code/scip/bindings/go/scip` (and `.../testutil`).

## Error Handling

- Return `error` as the last value; never ignore errors in library code.
- **Typed struct errors for user-facing diagnostics** — each lint finding is its own struct so tests can assert on exact messages (`cmd/scip/lint.go`):
- **Aggregate** many errors with a custom container and `errors.Join`: `lintMain` does `return stderrors.Join(lintMainPure(scipIndex).data...)` after collecting into `errorSet` (`cmd/scip/lint.go`).
- **Wrap with context** using `fmt.Errorf("...: %w", err)` — see `readFromOption` in `cmd/scip/option_from.go`:
- **Validate arguments early** at the CLI boundary: `if indexPath == "" { return stderrors.New("missing argument for path to SCIP index") }` (`cmd/scip/lint.go`, `cmd/scip/convert.go`).
- **Panic only for programmer errors / unreachable states**: `panic(fmt.Sprintf("Unknown symbolAttributeKind: %s", str))` in `testutil/test_runner.go`; `log.Fatal` reserved for CLI startup and truly fatal I/O (`cmd/scip/main.go`, `cmd/scip/convert.go`).
- Prefer `, ok` / `, bool` second returns over errors for "no result" cases: `sel, ok := parseRangeSelection(...)`, `pos, ok := occ.SourceRange()`.

## Logging

- User-facing CLI output goes through the cli command's writer or stdout: `fmt.Fprintf(cmd.Writer, "Successfully converted ...", outputPath)` in `cmd/scip/convert.go`; `fmt.Println("done: " + output)` in `cmd/scip/snapshot.go`.
- Structured warnings use `slog` with key-value attrs (`cmd/scip/convert.go`):
- Test-runner results use colored check/cross markers: `red.Fprintf(output, "✗ %s\n", ...)` / `green.Fprintf(output, "✓ %s (%d assertions)\n", ...)` in `testutil/test_runner.go`; tests force `os.Setenv("NO_COLOR", "1")` with `t.Cleanup` to unpollute golden output (`reprolang/repro/test_cmd_test.go`).
- Errors returned from `cli.Command.Action` are printed by `main` via `log.Fatal(err)` — prefer returning errors over printing-and-continuing.

## Comments

- Every exported identifier gets a godoc comment starting with the identifier name (`bindings/go/scip/symbol.go`):
- Add a `CAUTION:` line for sharp edges: `// CAUTION: Does not perform full validation of the symbol string's contents.`
- Explain *why*, not what, for non-obvious passes: `// This has to run in a second pass so that all symbols are first populated.` (`cmd/scip/lint.go`).
- Document parsing DSLs with a grammar block comment (see `parseSymbolInfo` in `cmd/scip/lint_test.go`).
- Use `TODO:` comments for known debt: `// TODO: Replace with cmp.Compare go 1.21 or newer.` (`bindings/go/scip/position_test.go`).
- Mark file/directory-level policy with sentinel comments/files: `bindings/go/scip/memtest/DO_NOT_ADD_NEW_TEST_FILES_HERE`.
- Multi-paragraph godoc with a blank comment line between paragraphs; reference related APIs by name (`ParseSymbol` ↔ `ParseSymbolUTF8` ↔ `ParseSymbolUTF8With`).
- Struct fields get line comments when non-obvious: `// path may be empty for external symbols` (`cmd/scip/lint.go`), `// IncludeDescriptors indicates whether ...` (`bindings/go/scip/symbol.go`).

## Function Design

## Module Design

- The library package `scip` (`bindings/go/scip`) exposes protobuf types plus helpers as free functions on the package (`scip.ParseSymbol`, `scip.NewRangeUnchecked`); avoid methods on generated types except thin `Source*` accessors.
- `testutil` (`bindings/go/scip/testutil`) exports the reusable test harnesses: `RunTests`, `SnapshotTest`, `SnapshotTestDirectories`, `FormatSnapshots` — intended for external indexer authors too.
- CLI package `main` (`cmd/scip`) exports nothing; command list assembled in `commands()` (`cmd/scip/main.go`).
- Not used (idiomatic Go — import the specific package).
- All bindings under `bindings/{typescript,rust,haskell,java,kotlin}` and `bindings/go/scip/scip.pb.go` are generated from `scip.proto` via `buf generate` (`buf.gen.yaml`); regenerate rather than edit. Hand-written Go additions go in sibling files (`symbol.go`, `sort.go`, ...), never in `scip.pb.go`.

<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

## System Overview

```
| TOP: Schema                                                                           |
|   scip.proto  (messages: Index, Document, Occurrence, Symbol, SymbolInformation, ...)  |
| MIDDLE: Generated bindings |  Go binding library (hand-written + generated)            |
|  bindings/typescript/      |  bindings/go/scip/                                      |
|  bindings/rust/            |   scip.pb.go (generated) + parse.go, symbol_parser.go,   |
|  bindings/haskell/         |   canonicalize.go, flatten.go, sort.go, sanitize.go,     |
|  bindings/java/            |   position.go, occurrence_range.go, source_file.go,     |
|  bindings/kotlin/          |   symbol_formatter.go   |  testutil/  |  memtest/       |
| APPLICATION: scip CLI (cmd/scip/, module github.com/scip-code/scip)                   |
|   main.go -> lint.go, print.go, snapshot.go, stats.go, test.go, convert.go            |
|   (option_from.go reads .scip protobuf; convert.go emits SQLite + zstd)               |
| TEST HARNESS: reprolang (module github.com/scip-code/scip/reprolang)                  |
|   grammar/ (tree-sitter, cgo)  ->  repro/indexer.go (parse -> name -> emit SCIP)      |
|   testdata/snapshots/{input,output}, testdata/test_cmd                                |
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| Protobuf schema | Canonical definition of the SCIP protocol (Index/Document/Occurrence/Symbol model) | `scip.proto` |
| Codegen config | Drives generation of all bindings + docs from `scip.proto` | `buf.gen.yaml`, `buf.yaml` |
| Go bindings library | Published Go API: parsing, symbol formatting, canonicalization, streaming | `bindings/go/scip/` |
| testutil package | Snapshot rendering and test-file validation shared by `scip snapshot`/`scip test` | `bindings/go/scip/testutil/` |
| CLI | All user-facing commands operating on `.scip` index files | `cmd/scip/main.go` |
| `lint` command | Well-formedness checks on an index (typed error taxonomy) | `cmd/scip/lint.go` |
| `print` command | Debug pretty-print (color TTY or JSON) of an index | `cmd/scip/print.go` |
| `snapshot` command | Golden-test snapshot files with caret (`^`) annotations | `cmd/scip/snapshot.go` |
| `stats` command | JSON statistics (documents, occurrences, percentiles, LOC) | `cmd/scip/stats.go` |
| `test` command | Validate index against human-readable `.scip`-annotated test files | `cmd/scip/test.go` |
| `expt-convert` command | Convert index to SQLite database with zstd-compressed occurrence chunks | `cmd/scip/convert.go` |
| Index reading | Shared `.scip` file / stdin -> `*scip.Index` loading | `cmd/scip/option_from.go` |
| reprolang grammar | Tree-sitter grammar (cgo binding + generated parser) for the test language | `reprolang/grammar/binding.go`, `reprolang/grammar/parser.c` |
| reprolang indexer | Reference indexer emitting SCIP, used to exercise the CLI end-to-end | `reprolang/repro/indexer.go` |
| Nix checks | Reproducible builds, proto generation, dependency checks | `flake.nix`, `checks.nix` |
| CI / release | PR checks, JVM binding publishing, tag/release pipeline | `.github/workflows/ci.yaml`, `.github/workflows/release.yaml` |

## Pattern Overview

- One schema (`scip.proto`) is the single source of truth; all six language bindings and `docs/scip.md` are generated from it via `buf.gen.yaml`.
- The Go binding library is a separately versioned module (`github.com/scip-code/scip/bindings/go/scip`) consumed by external tools (e.g. Sourcegraph CLI) — treat its API as public.
- CLI uses urfave/cli v3 with one file per subcommand; each file exposes a `<name>Command() cli.Command` constructor registered in `cmd/scip/main.go:22` (`commands()`).
- Pure-core/testable-seam style: command files split thin CLI wiring from `*Main(...)` functions (e.g. `lintMain` -> `lintMainPure` in `cmd/scip/lint.go:33`).
- Golden-file (snapshot) testing throughout: `testutil.SnapshotTest` in `bindings/go/scip/testutil/snapshot_testing.go` with a global `-update-snapshots` flag.

## Layers

- Purpose: Define the protocol contract: `Index`, `Metadata`, `ToolInfo`, `Document`, `Occurrence`, `Symbol`/`Package`/`Descriptor`/`Signature`, `SymbolInformation`, `Relationship`, and enums (`SymbolRole`, `SyntaxKind`, `Severity`, `Language`, `PositionEncoding`, `TextEncoding`).
- Location: `scip.proto`
- Contains: proto3 message/enum definitions with normative doc comments.
- Depends on: Nothing (leaf).
- Used by: All bindings via codegen; `docs/scip.md` via protoc-gen-doc.
- Purpose: Rich utilities over the generated types: symbol string parse/format, canonicalization, sorting, streaming parse, position/range math, source-file helpers.
- Location: `bindings/go/scip/` (own Go module with `bindings/go/scip/go.mod`)
- Contains: Generated `scip.pb.go` plus hand-written files: `parse.go`, `symbol.go`, `symbol_parser.go`, `symbol_formatter.go`, `canonicalize.go`, `flatten.go`, `sort.go`, `sanitize.go`, `position.go`, `occurrence_range.go`, `source_file.go`, `identifier.go`, `symbol_role.go`, `symbol_table.go`.
- Depends on: `google.golang.org/protobuf`, `github.com/sourcegraph/beaut` (UTF-8-safe strings).
- Used by: The CLI (`cmd/scip/*`), reprolang indexer (`reprolang/repro/*`), external consumers (src-cli).
- Purpose: Snapshot formatting and test-file runner shared between the library and the CLI's `snapshot`/`test` commands.
- Location: `bindings/go/scip/testutil/`
- Contains: `format.go` (`FormatSnapshots`), `test_runner.go` (`RunTests`), `snapshot_testing.go` (`SnapshotTest` harness).
- Depends on: Parent `scip` package, `github.com/hexops/gotextdiff`, `github.com/fatih/color`.
- Used by: `cmd/scip/snapshot.go`, `cmd/scip/test.go`, reprolang snapshot tests (`reprolang/repro/snapshot_test.go`).
- Purpose: User-facing subcommands over `.scip` files.
- Location: `cmd/scip/` (module `github.com/scip-code/scip`)
- Contains: `main.go` (app assembly, flags helpers, version embed) plus one file per command.
- Depends on: Go bindings module (via `replace` directive in root `go.mod`), `urfave/cli/v3`, `zombiezen.com/go/sqlite` + `klauspost/compress/zstd` (convert), `k0kubun/pp` (print), `hhatto/gocloc` + `montanaflynn/stats` (stats).
- Used by: End users; CI release workflow builds binaries from `go build ./cmd/scip`.
- Purpose: A small tree-sitter-parsed language whose indexer emits SCIP, used to test CLI semantics (definitions, references, relationships, diagnostics) without a real language toolchain.
- Location: `reprolang/` (own Go module `github.com/scip-code/scip/reprolang`)
- Contains: `reprolang/grammar.js` (grammar source), `reprolang/grammar/` (cgo binding + generated `parser.c`, `grammar.json`, `node-types.json`), `reprolang/repro/` (indexer implementation), `reprolang/testdata/` (golden inputs/outputs).
- Depends on: `github.com/tree-sitter/go-tree-sitter` (cgo), Go bindings module.
- Used by: Its own tests and `cmd/scip` integration tests; explicitly not for external use.
- Purpose: Distribute the SCIP schema to other ecosystems.
- Location: `bindings/typescript/` (`scip_pb.ts`), `bindings/rust/` (`src/generated/scip.rs` + hand-written `src/symbol.rs`), `bindings/haskell/` (`src/Proto/Scip.hs`), `bindings/java/` (`org.scip_code.scip`), `bindings/kotlin/`.
- Contains: Fully generated code (Rust additionally has hand-written symbol formatting in `bindings/rust/src/symbol.rs`).
- Depends on: Nothing in-repo (generated in place).
- Used by: External indexers/consumers per ecosystem.

## Data Flow

### Primary Request Path (e.g. `scip snapshot index.scip`)

### Convert Flow (`scip expt-convert index.scip`)

### reprolang Indexing Flow (test harness)

- CLI commands are stateless per invocation; all state is local to the action function (e.g. `symbolTable` in `cmd/scip/lint.go:122`, `symbolToID`/`docPositions` maps in `cmd/scip/convert.go:265`).
- Library functions are pure transformations over protobuf messages; canonicalization mutates documents in place and returns them for convenience (`bindings/go/scip/canonicalize.go:7`).
- Module-level mutable state is limited to test hooks: `SkipLintSymbolParseTest` (`cmd/scip/lint.go:247`) and the `updateSnapshots` flag (`bindings/go/scip/testutil/snapshot_testing.go:21`).

## Key Abstractions

- Purpose: Root protocol messages; an index holds metadata, documents (per-file occurrences + symbols), and external symbols.
- Examples: `scip.proto:26`, `scip.proto:76`, generated `bindings/go/scip/scip.pb.go`
- Pattern: Protobuf messages with hand-written extension methods in separate files (never edit `scip.pb.go`).
- Purpose: The cross-repo identifier syntax, e.g. `scip python python-stdlib 3.11.4 module main/`.
- Examples: parser `bindings/go/scip/symbol_parser.go`, entry points `bindings/go/scip/symbol.go:25` (`ParseSymbol`, `ParseSymbolUTF8`), identifier charset rules `bindings/go/scip/identifier.go`, Rust equivalent `bindings/rust/src/symbol.rs`
- Pattern: Hand-rolled recursive-descent parsing over `beaut.UTF8String` with `ParseSymbolOptions` to control descriptor recording.
- Purpose: Configurable symbol rendering (verbose vs descriptor-only) used by snapshot output.
- Examples: `bindings/go/scip/symbol_formatter.go` (`VerboseSymbolFormatter`, `LenientVerboseSymbolFormatter`, `DescriptorOnlyFormatter`)
- Pattern: Struct of function fields (filter callbacks + `OnError`), not an interface — deliberately, to allow adding fields without breaking clients.
- Purpose: Stream an `Index` payload at document granularity from an `io.Reader` without holding the whole message in memory.
- Examples: `bindings/go/scip/parse.go:11` (`IndexVisitor`), `bindings/go/scip/parse.go:27` (`ParseStreaming`), exercised under a memory cap by `bindings/go/scip/memtest/low_mem_test.go`
- Pattern: Manual protobuf wire-format decoding (varint tags/lengths); struct-of-functions instead of interface for forward compatibility. The proto file carries an inline reminder to update it when `Index` gains fields (`scip.proto:23`).
- Purpose: Line/character math independent of the wire representation (deprecated `int32` arrays vs typed `SingleLineRange`/`MultiLineRange`).
- Examples: `bindings/go/scip/position.go`, `bindings/go/scip/occurrence_range.go` (`Occurrence.SourceRange()` prefers `typed_range` over the deprecated `range`)
- Pattern: Value types with `Compare`/`Less`/`Contains`/`Intersects`; validation via `NewRange`/`Range.Validate`.
- Purpose: Deterministic, well-formed index normalization for storage/diffing.
- Examples: `bindings/go/scip/canonicalize.go` (sort + merge + sanitize), `bindings/go/scip/flatten.go` (dedupe documents/symbols), `bindings/go/scip/sort.go` (binary-searchable ordering), `bindings/go/scip/sanitize.go` (UTF-8 fixing)
- Pattern: Composable pure functions; post-condition documented per function.
- Purpose: Uniform absolute/relative path + text + lines representation for files fed to indexers or snapshotters.
- Examples: `bindings/go/scip/source_file.go` (`NewSourceFile`, `NewSourcesFromDirectory`)
- Pattern: Simple value struct factory.
- Purpose: Experimental relational projection of an index (documents, chunks, global_symbols, mentions, defn_enclosing_ranges).
- Examples: `cmd/scip/convert.go:237`
- Pattern: Prepared statements within one immediate transaction; zstd BLOB framing self-described by wrapping in a `scip.Document`/`scip.SymbolInformation` message.

## Entry Points

- Location: `cmd/scip/main.go`
- Triggers: User invocation of the built binary (`go build ./cmd/scip`); release binaries from `.github/workflows/release.yaml`.
- Responsibilities: Assemble the cli app (`scipApp`), register the six subcommands (`commands()` at `cmd/scip/main.go:22`), embed `version.txt`, compute dev version suffix from Go build info (`gitSuffix`), and provide shared flag helpers (`fromFlag`, `commentSyntaxFlag`, `projectRootFlag`).
- Location: `bindings/go/scip/` (importable module `github.com/scip-code/scip/bindings/go/scip`)
- Triggers: Go code in other repositories (Sourcegraph CLI, indexers).
- Responsibilities: All non-CLI functionality; keep this API stable and self-contained.
- Location: `reprolang/repro/indexer.go` (`repro.Index`)
- Triggers: reprolang module tests (`reprolang/repro/snapshot_test.go`, `test_cmd_test.go`).
- Responsibilities: Produce a `*scip.Index` from `*.repro` sources + dependencies.

## Architectural Constraints

- **Go workspace:** Three modules in `go.work` (root, `bindings/go/scip`, `reprolang`) linked by `replace` directives — new shared code goes in the bindings module, not the root module.
- **cgo required:** `reprolang/grammar/binding.go` uses cgo to expose the tree-sitter parser; reprolang builds require a C toolchain.
- **Threading:** No goroutine-based concurrency in the command paths; `memtest` tests must not run in parallel (`bindings/go/scip/memtest/low_mem_test.go` comment). The CLI is single-threaded per invocation.
- **Global state:** Minimal and test-only — `SkipLintSymbolParseTest` (`cmd/scip/lint.go:247`), `Reproducible` ldflags var (`cmd/scip/main.go:35`), `updateSnapshots` flag (`bindings/go/scip/testutil/snapshot_testing.go:21`).
- **Build tags:** `assert` is a real panic under `-tags asserts` and a noop otherwise (`bindings/go/scip/assertions.go`, `bindings/go/scip/assertions_noop.go`).
- **Circular imports:** None; dependency direction is strictly `cmd/scip` -> `bindings/go/scip` (+testutil) and `reprolang` -> `bindings/go/scip`.
- **Schema evolution:** Any new `Index` field requires a matching `IndexVisitor` function and `ParseStreaming` update (`scip.proto:23` comment); buf breaking-change detection is `FILE` mode (`buf.yaml`).
- **Version coupling:** `cmd/scip/version.txt` must match `bindings/rust/Cargo.toml`, `bindings/java/pom.xml`, `bindings/kotlin/pom.xml`, and `docs/CLI.md` — enforced by `.github/workflows/jvm-bindings.yaml`.

## Anti-Patterns

### Editing generated files

### Adding a subcommand without registering it

### Reading occurrences' deprecated `range` directly

### Loading an index ad hoc in a command

## Error Handling

- Typed lint diagnostics: each lint finding is a dedicated struct implementing `error` with prefixed text (`error:`/`warning:`/`note:`) — see the taxonomy in `cmd/scip/lint.go:273-410` (`duplicateSymbolInfoWarning`, `nonCanonicalSymbolError`, `missingSymbolForOccurrenceError`, ...), aggregated via `errorSet` and joined with `stderrors.Join` (`cmd/scip/lint.go:38`).
- `fmt.Errorf` with `%w` wrapping plus context (path, document index) throughout `cmd/scip/convert.go`.
- `errors.Join` for deferred close/transaction errors (`cmd/scip/convert.go:111`, `bindings/go/scip/testutil/format.go:60`).
- Non-fatal degradation: `scip stats` logs and continues when LOC counting fails (`cmd/scip/stats.go` `countStatistics`).
- Formatter errors are routed through `SymbolFormatter.OnError` so lenient modes can drop them (`bindings/go/scip/symbol_formatter.go:12`).

## Cross-Cutting Concerns

<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
