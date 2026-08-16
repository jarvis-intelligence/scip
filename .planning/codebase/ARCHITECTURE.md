<!-- refreshed: 2026-08-16 -->
# Architecture

**Analysis Date:** 2026-08-16

## System Overview

SCIP ("skip") is a language-agnostic Protobuf protocol for source-code intelligence.
This repository is a Go workspace (see `go.work`) hosting three Go modules — the CLI,
the published Go binding library, and a toy "reprolang" indexer used for end-to-end
testing — plus code-generated bindings for five other language ecosystems, all driven
from a single Protobuf schema.

```
+---------------------------------------------------------------------------------------+
| TOP: Schema                                                                           |
|   scip.proto  (messages: Index, Document, Occurrence, Symbol, SymbolInformation, ...)  |
+--------------------------------------------+------------------------------------------+
                                             |  buf codegen (buf.gen.yaml, nix run .#proto-generate)
+----------------------------+---------------+---------------------+--------------------+
| MIDDLE: Generated bindings |  Go binding library (hand-written + generated)            |
|  bindings/typescript/      |  bindings/go/scip/                                      |
|  bindings/rust/            |   scip.pb.go (generated) + parse.go, symbol_parser.go,   |
|  bindings/haskell/         |   canonicalize.go, flatten.go, sort.go, sanitize.go,     |
|  bindings/java/            |   position.go, occurrence_range.go, source_file.go,     |
|  bindings/kotlin/          |   symbol_formatter.go   |  testutil/  |  memtest/       |
+----------------------------+-------------------------+--------------+-----------------+
                                             |                                   |
+--------------------------------------------+-------------+---------------------+----+
| APPLICATION: scip CLI (cmd/scip/, module github.com/scip-code/scip)                   |
|   main.go -> lint.go, print.go, snapshot.go, stats.go, test.go, convert.go            |
|   (option_from.go reads .scip protobuf; convert.go emits SQLite + zstd)               |
+--------------------------------------------+------------------------------------------+
                                             |
+--------------------------------------------+------------------------------------------+
| TEST HARNESS: reprolang (module github.com/scip-code/scip/reprolang)                  |
|   grammar/ (tree-sitter, cgo)  ->  repro/indexer.go (parse -> name -> emit SCIP)      |
|   testdata/snapshots/{input,output}, testdata/test_cmd                                |
+---------------------------------------------------------------------------------------+

STORAGE / OUTPUT:
  *.scip protobuf index files  |  index.db SQLite (expt-convert)  |  snapshot text files
  JSON stats (stdout)  |  docs/scip.md (generated from proto)
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

**Overall:** Protobuf-schema-first, layered CLI architecture (schema -> generated + hand-written binding library -> command layer -> test harness).

**Key Characteristics:**
- One schema (`scip.proto`) is the single source of truth; all six language bindings and `docs/scip.md` are generated from it via `buf.gen.yaml`.
- The Go binding library is a separately versioned module (`github.com/scip-code/scip/bindings/go/scip`) consumed by external tools (e.g. Sourcegraph CLI) — treat its API as public.
- CLI uses urfave/cli v3 with one file per subcommand; each file exposes a `<name>Command() cli.Command` constructor registered in `cmd/scip/main.go:22` (`commands()`).
- Pure-core/testable-seam style: command files split thin CLI wiring from `*Main(...)` functions (e.g. `lintMain` -> `lintMainPure` in `cmd/scip/lint.go:33`).
- Golden-file (snapshot) testing throughout: `testutil.SnapshotTest` in `bindings/go/scip/testutil/snapshot_testing.go` with a global `-update-snapshots` flag.

## Layers

**Schema layer:**
- Purpose: Define the protocol contract: `Index`, `Metadata`, `ToolInfo`, `Document`, `Occurrence`, `Symbol`/`Package`/`Descriptor`/`Signature`, `SymbolInformation`, `Relationship`, and enums (`SymbolRole`, `SyntaxKind`, `Severity`, `Language`, `PositionEncoding`, `TextEncoding`).
- Location: `scip.proto`
- Contains: proto3 message/enum definitions with normative doc comments.
- Depends on: Nothing (leaf).
- Used by: All bindings via codegen; `docs/scip.md` via protoc-gen-doc.

**Binding library layer (Go):**
- Purpose: Rich utilities over the generated types: symbol string parse/format, canonicalization, sorting, streaming parse, position/range math, source-file helpers.
- Location: `bindings/go/scip/` (own Go module with `bindings/go/scip/go.mod`)
- Contains: Generated `scip.pb.go` plus hand-written files: `parse.go`, `symbol.go`, `symbol_parser.go`, `symbol_formatter.go`, `canonicalize.go`, `flatten.go`, `sort.go`, `sanitize.go`, `position.go`, `occurrence_range.go`, `source_file.go`, `identifier.go`, `symbol_role.go`, `symbol_table.go`.
- Depends on: `google.golang.org/protobuf`, `github.com/sourcegraph/beaut` (UTF-8-safe strings).
- Used by: The CLI (`cmd/scip/*`), reprolang indexer (`reprolang/repro/*`), external consumers (src-cli).

**testutil layer:**
- Purpose: Snapshot formatting and test-file runner shared between the library and the CLI's `snapshot`/`test` commands.
- Location: `bindings/go/scip/testutil/`
- Contains: `format.go` (`FormatSnapshots`), `test_runner.go` (`RunTests`), `snapshot_testing.go` (`SnapshotTest` harness).
- Depends on: Parent `scip` package, `github.com/hexops/gotextdiff`, `github.com/fatih/color`.
- Used by: `cmd/scip/snapshot.go`, `cmd/scip/test.go`, reprolang snapshot tests (`reprolang/repro/snapshot_test.go`).

**CLI layer:**
- Purpose: User-facing subcommands over `.scip` files.
- Location: `cmd/scip/` (module `github.com/scip-code/scip`)
- Contains: `main.go` (app assembly, flags helpers, version embed) plus one file per command.
- Depends on: Go bindings module (via `replace` directive in root `go.mod`), `urfave/cli/v3`, `zombiezen.com/go/sqlite` + `klauspost/compress/zstd` (convert), `k0kubun/pp` (print), `hhatto/gocloc` + `montanaflynn/stats` (stats).
- Used by: End users; CI release workflow builds binaries from `go build ./cmd/scip`.

**Test-harness layer (reprolang):**
- Purpose: A small tree-sitter-parsed language whose indexer emits SCIP, used to test CLI semantics (definitions, references, relationships, diagnostics) without a real language toolchain.
- Location: `reprolang/` (own Go module `github.com/scip-code/scip/reprolang`)
- Contains: `reprolang/grammar.js` (grammar source), `reprolang/grammar/` (cgo binding + generated `parser.c`, `grammar.json`, `node-types.json`), `reprolang/repro/` (indexer implementation), `reprolang/testdata/` (golden inputs/outputs).
- Depends on: `github.com/tree-sitter/go-tree-sitter` (cgo), Go bindings module.
- Used by: Its own tests and `cmd/scip` integration tests; explicitly not for external use.

**Generated non-Go bindings layer:**
- Purpose: Distribute the SCIP schema to other ecosystems.
- Location: `bindings/typescript/` (`scip_pb.ts`), `bindings/rust/` (`src/generated/scip.rs` + hand-written `src/symbol.rs`), `bindings/haskell/` (`src/Proto/Scip.hs`), `bindings/java/` (`org.scip_code.scip`), `bindings/kotlin/`.
- Contains: Fully generated code (Rust additionally has hand-written symbol formatting in `bindings/rust/src/symbol.rs`).
- Depends on: Nothing in-repo (generated in place).
- Used by: External indexers/consumers per ecosystem.

## Data Flow

### Primary Request Path (e.g. `scip snapshot index.scip`)

1. `main()` builds the urfave/cli app and dispatches to the subcommand action (`cmd/scip/main.go:15`, `cmd/scip/main.go:60`).
2. `readFromOption(fromPath)` loads bytes from a `.scip` file (or stdin when `-`) and `proto.Unmarshal`s them into `*scip.Index` (`cmd/scip/option_from.go:15`).
3. The command's `*Main` function processes the in-memory index — e.g. `snapshotMain` calls `testutil.FormatSnapshots` (`cmd/scip/snapshot.go:66`, `bindings/go/scip/testutil/format.go:15`).
4. Output is produced: snapshot files written under the `--to` directory (`cmd/scip/snapshot.go:77-92`), JSON/stats to stdout (`cmd/scip/print.go:65`, `cmd/scip/stats.go:47`), or lint errors joined and returned (`cmd/scip/lint.go:33`).

### Convert Flow (`scip expt-convert index.scip`)

1. `convertMain` loads the index via `readFromOption` and opens a SQLite connection with WAL/strict pragmas (`cmd/scip/convert.go:74`, `cmd/scip/convert.go:146`).
2. `Converter.Convert` canonicalizes each document (`scip.CanonicalizeDocument`), inserts `documents`, dedupes and inserts `global_symbols`, records `defn_enclosing_ranges` from definition occurrences (`cmd/scip/convert.go:258-318`).
3. Occurrences are grouped into ~200-line chunks (`chunkOccurrences`, `cmd/scip/convert.go:618`), each chunk protobuf-marshalled into a `scip.Document{Occurrences:...}` wrapper and zstd-compressed into the `chunks.occurrences` BLOB; symbol-role pairs populate `mentions` (`cmd/scip/convert.go:320-376`, `cmd/scip/convert.go:503`).
4. Indexes are created in `prepareIndexes` (`cmd/scip/convert.go:131`).

### reprolang Indexing Flow (test harness)

1. `repro.Index(projectRoot, packageName, sources, dependencies)` parses each `*.repro` source with tree-sitter (`reprolang/repro/indexer.go:12`, `reprolang/repro/parser.go:15`).
2. Two phases inside the indexer: parse all sources/dependencies, then name resolution through global/local scopes (`reprolang/repro/namer.go`, `reprolang/repro/ast.go:90` `identifier.resolveSymbol`).
3. Resolved symbols and relationships are converted to SCIP messages (`reprolang/repro/scip.go`) and returned as a `*scip.Index`, which snapshot tests diff against golden files (`bindings/go/scip/testutil/snapshot_testing.go`).

**State Management:**
- CLI commands are stateless per invocation; all state is local to the action function (e.g. `symbolTable` in `cmd/scip/lint.go:122`, `symbolToID`/`docPositions` maps in `cmd/scip/convert.go:265`).
- Library functions are pure transformations over protobuf messages; canonicalization mutates documents in place and returns them for convenience (`bindings/go/scip/canonicalize.go:7`).
- Module-level mutable state is limited to test hooks: `SkipLintSymbolParseTest` (`cmd/scip/lint.go:247`) and the `updateSnapshots` flag (`bindings/go/scip/testutil/snapshot_testing.go:21`).

## Key Abstractions

**`scip.Index` / `scip.Document`:**
- Purpose: Root protocol messages; an index holds metadata, documents (per-file occurrences + symbols), and external symbols.
- Examples: `scip.proto:26`, `scip.proto:76`, generated `bindings/go/scip/scip.pb.go`
- Pattern: Protobuf messages with hand-written extension methods in separate files (never edit `scip.pb.go`).

**Symbol string format (`scheme manager name version descriptor...`):**
- Purpose: The cross-repo identifier syntax, e.g. `scip python python-stdlib 3.11.4 module main/`.
- Examples: parser `bindings/go/scip/symbol_parser.go`, entry points `bindings/go/scip/symbol.go:25` (`ParseSymbol`, `ParseSymbolUTF8`), identifier charset rules `bindings/go/scip/identifier.go`, Rust equivalent `bindings/rust/src/symbol.rs`
- Pattern: Hand-rolled recursive-descent parsing over `beaut.UTF8String` with `ParseSymbolOptions` to control descriptor recording.

**`SymbolFormatter`:**
- Purpose: Configurable symbol rendering (verbose vs descriptor-only) used by snapshot output.
- Examples: `bindings/go/scip/symbol_formatter.go` (`VerboseSymbolFormatter`, `LenientVerboseSymbolFormatter`, `DescriptorOnlyFormatter`)
- Pattern: Struct of function fields (filter callbacks + `OnError`), not an interface — deliberately, to allow adding fields without breaking clients.

**`IndexVisitor` / `ParseStreaming`:**
- Purpose: Stream an `Index` payload at document granularity from an `io.Reader` without holding the whole message in memory.
- Examples: `bindings/go/scip/parse.go:11` (`IndexVisitor`), `bindings/go/scip/parse.go:27` (`ParseStreaming`), exercised under a memory cap by `bindings/go/scip/memtest/low_mem_test.go`
- Pattern: Manual protobuf wire-format decoding (varint tags/lengths); struct-of-functions instead of interface for forward compatibility. The proto file carries an inline reminder to update it when `Index` gains fields (`scip.proto:23`).

**`Range` / `Position`:**
- Purpose: Line/character math independent of the wire representation (deprecated `int32` arrays vs typed `SingleLineRange`/`MultiLineRange`).
- Examples: `bindings/go/scip/position.go`, `bindings/go/scip/occurrence_range.go` (`Occurrence.SourceRange()` prefers `typed_range` over the deprecated `range`)
- Pattern: Value types with `Compare`/`Less`/`Contains`/`Intersects`; validation via `NewRange`/`Range.Validate`.

**Canonicalization pipeline:**
- Purpose: Deterministic, well-formed index normalization for storage/diffing.
- Examples: `bindings/go/scip/canonicalize.go` (sort + merge + sanitize), `bindings/go/scip/flatten.go` (dedupe documents/symbols), `bindings/go/scip/sort.go` (binary-searchable ordering), `bindings/go/scip/sanitize.go` (UTF-8 fixing)
- Pattern: Composable pure functions; post-condition documented per function.

**`SourceFile`:**
- Purpose: Uniform absolute/relative path + text + lines representation for files fed to indexers or snapshotters.
- Examples: `bindings/go/scip/source_file.go` (`NewSourceFile`, `NewSourcesFromDirectory`)
- Pattern: Simple value struct factory.

**`Converter` (SQLite):**
- Purpose: Experimental relational projection of an index (documents, chunks, global_symbols, mentions, defn_enclosing_ranges).
- Examples: `cmd/scip/convert.go:237`
- Pattern: Prepared statements within one immediate transaction; zstd BLOB framing self-described by wrapping in a `scip.Document`/`scip.SymbolInformation` message.

## Entry Points

**`scip` CLI binary:**
- Location: `cmd/scip/main.go`
- Triggers: User invocation of the built binary (`go build ./cmd/scip`); release binaries from `.github/workflows/release.yaml`.
- Responsibilities: Assemble the cli app (`scipApp`), register the six subcommands (`commands()` at `cmd/scip/main.go:22`), embed `version.txt`, compute dev version suffix from Go build info (`gitSuffix`), and provide shared flag helpers (`fromFlag`, `commentSyntaxFlag`, `projectRootFlag`).

**Library entry (external consumers):**
- Location: `bindings/go/scip/` (importable module `github.com/scip-code/scip/bindings/go/scip`)
- Triggers: Go code in other repositories (Sourcegraph CLI, indexers).
- Responsibilities: All non-CLI functionality; keep this API stable and self-contained.

**reprolang indexer:**
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

**What happens:** Hand-editing `bindings/go/scip/scip.pb.go`, `bindings/typescript/scip_pb.ts`, `bindings/rust/src/generated/scip.rs`, `bindings/haskell/src/Proto/*.hs`, `bindings/java/**`, `bindings/kotlin/**`, `bindings/rust/src/generated/`, or `docs/scip.md`.
**Why it's wrong:** These are regenerated from `scip.proto` by `nix run .#proto-generate` (`buf.gen.yaml`); edits are silently lost on the next generation.
**Do this instead:** Change `scip.proto`, then regenerate. Hand-written Go helpers live in sibling files like `bindings/go/scip/symbol.go`; Rust helpers live in `bindings/rust/src/symbol.rs`.

### Adding a subcommand without registering it

**What happens:** Creating `cmd/scip/foo.go` with a `fooCommand()` constructor but not wiring it in.
**Why it's wrong:** `commands()` in `cmd/scip/main.go:22` is the single registry; unregistered commands are unreachable and unbuilt-in (also update `docs/CLI.md`).
**Do this instead:** Add the constructor to the list in `commands()` following the existing one-file-per-command pattern (`cmd/scip/lint.go:15`).

### Reading occurrences' deprecated `range` directly

**What happens:** Indexing into `occ.Range` (deprecated `[]int32`) without a length check.
**Why it's wrong:** Some indexers emit malformed/missing ranges; direct access crashes canonicalization and downstream code (see `RemoveIllegalOccurrences` rationale, `bindings/go/scip/canonicalize.go`).
**Do this instead:** Use `occ.SourceRange()` / `occ.EnclosingSourceRange()` (`bindings/go/scip/occurrence_range.go`), which prefer typed ranges and report missing ranges as `ok=false`.

### Loading an index ad hoc in a command

**What happens:** Each command opening/decoding `.scip` files itself.
**Why it's wrong:** Duplicates the `.scip`-extension check, stdin (`-`) handling, and error wrapping that users rely on.
**Do this instead:** Call `readFromOption(path)` from `cmd/scip/option_from.go:15`.

## Error Handling

**Strategy:** Return `error` up to the CLI; accumulate multiple diagnostics where a single verdict is unhelpful; typed error structs for lint output.

**Patterns:**
- Typed lint diagnostics: each lint finding is a dedicated struct implementing `error` with prefixed text (`error:`/`warning:`/`note:`) — see the taxonomy in `cmd/scip/lint.go:273-410` (`duplicateSymbolInfoWarning`, `nonCanonicalSymbolError`, `missingSymbolForOccurrenceError`, ...), aggregated via `errorSet` and joined with `stderrors.Join` (`cmd/scip/lint.go:38`).
- `fmt.Errorf` with `%w` wrapping plus context (path, document index) throughout `cmd/scip/convert.go`.
- `errors.Join` for deferred close/transaction errors (`cmd/scip/convert.go:111`, `bindings/go/scip/testutil/format.go:60`).
- Non-fatal degradation: `scip stats` logs and continues when LOC counting fails (`cmd/scip/stats.go` `countStatistics`).
- Formatter errors are routed through `SymbolFormatter.OnError` so lenient modes can drop them (`bindings/go/scip/symbol_formatter.go:12`).

## Cross-Cutting Concerns

**Logging:** `log` / `log/slog` standard-library logging for warnings (duplicate documents in `cmd/scip/convert.go:270`, missing project root in `bindings/go/scip/testutil/format.go:41`). No structured logging framework; CLI output intended for scripts goes through `cmd.Writer`/stdout.
**Validation:** Dedicated `scip lint` command (`cmd/scip/lint.go`) plus in-library validation (`Range.Validate` in `bindings/go/scip/position.go:87`, `SanitizeDocument` for UTF-8 in `bindings/go/scip/sanitize.go`, buf lint rules in `buf.yaml`).
**Authentication:** Not applicable — offline CLI and libraries; no network calls in the Go code. CI release publishing uses GitHub Actions secrets (`MAVEN_*`) documented in `docs/Development.md`.

---

*Architecture analysis: 2026-08-16*
