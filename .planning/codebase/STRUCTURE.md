# Codebase Structure

**Analysis Date:** 2026-08-16

## Directory Layout

```
scip/                              # Root Go module: github.com/scip-code/scip (CLI)
├── scip.proto                     # THE protocol schema — single source of truth
├── buf.gen.yaml                   # Protobuf codegen plugin config (all bindings + docs)
├── buf.yaml                       # Buf lint/breaking-change rules
├── go.work                        # Go workspace: root + bindings/go/scip + reprolang
├── go.mod / go.sum                # CLI module deps (urfave/cli, sqlite, zstd, ...)
├── flake.nix / flake.lock         # Nix build/check environment
├── checks.nix                     # Nix CI checks (proto generation, module checks)
├── cmd/
│   └── scip/                      # CLI entry point, one file per subcommand
│       ├── main.go                # App assembly, command registry, shared flags
│       ├── version.txt            # Version embedded via go:embed (currently 0.9.0)
│       ├── option_from.go         # readFromOption: .scip file/stdin -> *scip.Index
│       ├── lint.go / print.go / snapshot.go / stats.go / test.go / convert.go
│       └── *_test.go              # Per-command tests
├── bindings/                      # Language bindings, mostly generated
│   ├── go/scip/                   # Published Go module (generated + hand-written)
│   │   ├── scip.pb.go             # GENERATED protobuf types — never edit
│   │   ├── parse.go               # IndexVisitor + ParseStreaming
│   │   ├── symbol.go / symbol_parser.go / symbol_formatter.go
│   │   ├── canonicalize.go / flatten.go / sort.go / sanitize.go
│   │   ├── position.go / occurrence_range.go / source_file.go
│   │   ├── identifier.go / symbol_role.go / symbol_table.go
│   │   ├── assertions.go / assertions_noop.go   # -tags asserts toggle
│   │   ├── testutil/              # FormatSnapshots, RunTests, SnapshotTest harness
│   │   └── memtest/               # Low-memory streaming parse test (isolated)
│   ├── typescript/                # GENERATED: scip_pb.ts (+ package.json)
│   ├── rust/                      # GENERATED src/generated/ + hand-written src/symbol.rs
│   ├── haskell/                   # GENERATED: src/Proto/Scip.hs (+ scip.cabal)
│   ├── java/                      # GENERATED: org.scip_code.scip (+ pom.xml)
│   └── kotlin/                    # GENERATED: org.scip_code.scip (+ pom.xml)
├── reprolang/                     # Own Go module: tree-sitter test language
│   ├── grammar.js                 # Grammar SOURCE (edit this)
│   ├── generate-tree-sitter-parser.sh
│   ├── grammar/                   # GENERATED parser.c, grammar.json, cgo binding.go
│   ├── repro/                     # Hand-written indexer (ast, parser, namer, indexer, scip)
│   └── testdata/
│       ├── snapshots/input/       # Golden test inputs (one dir per scenario)
│       ├── snapshots/output/      # Golden expected snapshots (regen: -update-snapshots)
│       └── test_cmd/              # Fixtures for `scip test` (diagnostics, ranges, roles)
├── docs/                          # Development.md + GENERATED scip.md, CLI.md, DESIGN.md
├── .github/workflows/             # ci.yaml, release.yaml, jvm-bindings.yaml, proto-review.yaml, scip-examples.yaml
└── .planning/                     # GSD planning docs (this analysis)
```

## Directory Purposes

**`cmd/scip/`:**
- Purpose: The `scip` CLI binary — all user-facing subcommands.
- Contains: One `<command>.go` + `<command>_test.go` per subcommand; `main.go` registry.
- Key files: `cmd/scip/main.go`, `cmd/scip/option_from.go`, `cmd/scip/lint.go`, `cmd/scip/convert.go`

**`bindings/go/scip/`:**
- Purpose: The published Go library for SCIP (import path `github.com/scip-code/scip/bindings/go/scip`); used by this CLI, reprolang, and external tools.
- Contains: Generated protobuf types plus hand-written helpers; nested `testutil/` and `memtest/` packages.
- Key files: `bindings/go/scip/parse.go`, `bindings/go/scip/symbol_parser.go`, `bindings/go/scip/canonicalize.go`, `bindings/go/scip/position.go`

**`bindings/go/scip/testutil/`:**
- Purpose: Reusable snapshot/test-file machinery shared by the CLI and reprolang.
- Contains: `format.go` (snapshot rendering), `test_runner.go` (`scip test` core), `snapshot_testing.go` (golden harness with `-update-snapshots`).

**`bindings/{typescript,rust,haskell,java,kotlin}/`:**
- Purpose: Distribute the schema to other ecosystems.
- Contains: Generated code plus per-ecosystem manifests (`package.json`, `Cargo.toml`, `scip.cabal`, `pom.xml`); only `bindings/rust/src/symbol.rs` is hand-written.

**`reprolang/`:**
- Purpose: Toy language + reference indexer used to test SCIP semantics end-to-end; not for external use (stated in `docs/Development.md`).
- Contains: Tree-sitter grammar (source `reprolang/grammar.js`, generated `reprolang/grammar/`), indexer in `reprolang/repro/`, golden data in `reprolang/testdata/`.
- Key files: `reprolang/repro/indexer.go`, `reprolang/repro/parser.go`, `reprolang/repro/namer.go`

**`docs/`:**
- Purpose: Developer and user documentation.
- Contains: Hand-written `Development.md`, `DESIGN.md`, `test_file_format.md`; generated `scip.md` (from proto comments) and `CLI.md`; template `scip.sprig`.
- Key files: `docs/Development.md` (project structure + workflows), `docs/CLI.md` (command reference)

**`.github/workflows/`:**
- Purpose: CI, release, and binding-publishing pipelines.
- Key files: `.github/workflows/ci.yaml` (PR checks incl. Nix), `.github/workflows/release.yaml` (tags, crates.io, Maven Central, binaries)

## Key File Locations

**Entry Points:**
- `cmd/scip/main.go`: CLI `main()` and command registry (`commands()`); add new subcommands here.
- `reprolang/repro/indexer.go`: `repro.Index(...)` — reference indexer entry for the test language.
- `bindings/go/scip/parse.go`: `IndexVisitor.ParseStreaming` — streaming entry for large indexes.

**Configuration:**
- `scip.proto`: Protocol schema — every structural change starts here.
- `buf.gen.yaml`: Which generators run and where they write (Go, TS, Rust, Haskell, Java, Kotlin, docs).
- `buf.yaml`: Protobuf lint exceptions and breaking-change policy (`FILE` mode).
- `go.work`: Workspace composition of the three modules.
- `flake.nix` / `checks.nix`: Nix dev shell and CI checks; `nix run .#proto-generate` regenerates bindings.
- `cmd/scip/version.txt`: Version single-source (must match Rust/Java/Kotlin manifests and `docs/CLI.md`).

**Core Logic:**
- `cmd/scip/lint.go`: Symbol table + full lint error taxonomy.
- `cmd/scip/convert.go`: SQLite schema, chunking, zstd framing for `expt-convert`.
- `bindings/go/scip/symbol_parser.go`: Symbol string grammar parser.
- `bindings/go/scip/canonicalize.go` + `flatten.go` + `sort.go`: Index normalization pipeline.
- `bindings/go/scip/occurrence_range.go`: Typed-range-precedence accessors.

**Testing:**
- `cmd/scip/*_test.go`: Unit tests per command.
- `bindings/go/scip/*_test.go`: Library tests (parser, formatter, canonicalize, position, sort).
- `bindings/go/scip/testutil/snapshot_testing.go`: Golden harness; run with `-update-snapshots` to regenerate.
- `reprolang/testdata/snapshots/`: Golden input/output scenario dirs (cyclic-reference, diagnostics, relationships, ...).
- `reprolang/testdata/test_cmd/`: `.scip`-annotated test files for the `test` command.

## Naming Conventions

**Files:**
- Go source: lower_snake_case matching content — `symbol_formatter.go`, `occurrence_range.go`, `canonicalize.go`.
- CLI subcommands: `<command>.go` exposing `<command>Command() cli.Command` (e.g. `cmd/scip/lint.go` -> `lintCommand`), with pure logic in `<command>Main` / `<command>MainPure`.
- Tests: `*_test.go` adjacent to the code under test (Go convention).
- Generated: `scip.pb.go` (Go), `scip_pb.ts` (TS), `src/generated/scip.rs` (Rust), `Scip.hs`/`Scip_Fields.hs` (Haskell).

**Directories:**
- One directory per published module or ecosystem: `bindings/go/scip`, `bindings/rust`, ...
- Test fixture scenarios are kebab-case directories: `reprolang/testdata/snapshots/input/forward-def`, `.../cyclic-reference`.
- Repo dirs are short lowercase names: `cmd`, `bindings`, `docs`, `reprolang`.

## Where to Add New Code

**New CLI subcommand:**
- Primary code: `cmd/scip/<command>.go` with `<command>Command() cli.Command`; register it in `commands()` at `cmd/scip/main.go:22`; document it in `docs/CLI.md`.
- Tests: `cmd/scip/<command>_test.go`.
- Read indexes via `readFromOption` (`cmd/scip/option_from.go`), not ad-hoc file I/O.

**New protocol field/message:**
- Primary code: `scip.proto` (update doc comments; if adding an `Index` field, also add the `IndexVisitor` callback and `ParseStreaming` support — see the comment at `scip.proto:23`).
- Regenerate: `nix run .#proto-generate`; commit regenerated bindings; update `docs/CLI.md` if user-visible.
- Tests: snapshot tests via reprolang scenarios under `reprolang/testdata/snapshots/input/` + `-update-snapshots`.

**New library utility (Go):**
- Implementation: `bindings/go/scip/<topic>.go` with `*_test.go` beside it. Never edit `bindings/go/scip/scip.pb.go` — add extension methods in a sibling file.
- Only move code into this module if it must be shared with external consumers; otherwise keep it in `cmd/scip`.

**New reprolang test scenario:**
- Implementation: new directory under `reprolang/testdata/snapshots/input/<scenario>/` with `*.repro` files; run `go test ./cmd/scip -update-snapshots` (or reprolang tests) to generate `snapshots/output/<scenario>/`.

**Utilities:**
- Shared helpers: `bindings/go/scip/` (e.g. `source_file.go`, `position.go`); CLI-only helpers stay in `cmd/scip/` (e.g. generic `set[T]`, `mySlice[T]` in `cmd/scip/lint.go:437-494`).

## Special Directories

**`bindings/` (all non-Go subdirs + `bindings/go/scip/scip.pb.go`):**
- Purpose: Generated protocol bindings for six ecosystems.
- Generated: Yes (by `buf.gen.yaml`); exceptions that are hand-written: `bindings/go/scip/*.go` helpers, `bindings/rust/src/symbol.rs`, `bindings/rust/src/mod.rs`.
- Committed: Yes.

**`reprolang/grammar/`:**
- Purpose: Generated tree-sitter parser artifacts (`parser.c`, `grammar.json`, `node-types.json`) plus cgo `binding.go`.
- Generated: Yes — regenerate with `reprolang/generate-tree-sitter-parser.sh` after editing `reprolang/grammar.js`.
- Committed: Yes.

**`reprolang/testdata/`:**
- Purpose: Golden inputs and expected outputs driving snapshot and `scip test` coverage.
- Generated: Input dirs are hand-authored; `snapshots/output/` is regenerated via `-update-snapshots`.
- Committed: Yes.

**`docs/` (`scip.md`, and `CLI.md` partly):**
- Purpose: Documentation; `scip.md` rendered from proto comments via protoc-gen-doc with `docs/scip.sprig` template.
- Generated: `scip.md` yes; `Development.md`, `DESIGN.md`, `test_file_format.md` no.
- Committed: Yes.

**`.planning/`:**
- Purpose: GSD planning/codebase-analysis documents (this file's home).
- Generated: No.
- Committed: Yes.

---

*Structure analysis: 2026-08-16*
