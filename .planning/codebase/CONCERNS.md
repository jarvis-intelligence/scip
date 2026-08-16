# Codebase Concerns

**Analysis Date:** 2026-08-16

Scope: full repo — `scip.proto`, `cmd/scip` (CLI), `bindings/go/scip` (+ `testutil`), `bindings/{rust,typescript,haskell,java,kotlin}`, `reprolang`, CI workflows. Two of the bugs below were reproduced against the current tree (marked VERIFIED).

## Tech Debt

**Hand-rolled symbol parser with unchecked invariants:**
- Issue: The allocation-optimized parser (`symbolParserV2`) relies on internal pre-conditions enforced only by an `assert` that is compiled out in release builds (`//go:build !asserts`). One violated invariant (the `peekNext` bounds guard) is already a live crash (see Known Bugs).
- Files: `bindings/go/scip/symbol_parser.go`, `bindings/go/scip/assertions.go`, `bindings/go/scip/assertions_noop.go`
- Impact: Any parser edge case becomes a runtime panic in every downstream consumer (scip-java, scip-python, scip-typescript, src-cli all import this package) rather than a returned error.
- Fix approach: Correct the `peekNext` guard; add Go native fuzzing (`func FuzzParseSymbol`) over the full symbol grammar; consider making `assert` always-on for parser internals (cost is negligible vs. protobuf decode).

**SQL statements re-prepared on every insert (`scip convert`):**
- Issue: `insertGlobalSymbols`, `insertChunk`, `insertDocument`, and the mentions/defn-range inserts call `c.conn.Prepare(...)` inside the per-row function, so an index with N symbols prepares N identical statements.
- Files: `cmd/scip/convert.go` (lines 323, 379, 447-451, 545-548, 568-571)
- Impact: Convert throughput degrades on large indexes; each `Prepare` is thrown away without `Finalize`, stressing the connection's statement cache.
- Fix approach: Prepare all statements once in `NewConverter`/`Convert` prologue and reuse; call `Reset`/`ClearBindings` in a defer per row.

**`errorSet.Unique()` is dead code — lint output not deduplicated:**
- Issue: `lintMain` joins the raw `errorSet.data` slice; the `Unique()` method that deduplicates identical messages is never called anywhere.
- Files: `cmd/scip/lint.go:38` (`stderrors.Join(lintMainPure(scipIndex).data...)`), `cmd/scip/lint.go:422-435`
- Impact: Indexes with many repeated violations produce massive duplicate lint output; the dedup machinery written for this purpose never runs.
- Fix approach: `stderrors.Join(lintMainPure(scipIndex).Unique()...)` — or delete `Unique()` if ordering matters.

**Global mutable test hook in production code path:**
- Issue: Package-level `var SkipLintSymbolParseTest` short-circuits `lintSymbolString` and is flipped from `lint_test.go`, coupling library behavior to test state.
- Files: `cmd/scip/lint.go:246-258`, `cmd/scip/lint_test.go:99`
- Impact: Any future concurrent test or CLI subcommand inherits whichever value a test last set; hides real parse errors in tests.
- Fix approach: Pass a lint-options struct through `lintMainPure` instead of mutating a global.

**In-place mutation as API contract:**
- Issue: `CanonicalizeDocument`, `SanitizeDocument`, `Flatten*`, `Sort*`, `RemoveIllegalOccurrences` (backing-array reuse via `occurrences[:0]`), and `FormatSnapshot` (sorts `document.Occurrences`) all mutate their inputs while being named like pure helpers.
- Files: `bindings/go/scip/canonicalize.go`, `bindings/go/scip/sanitize.go`, `bindings/go/scip/flatten.go:30-41`, `bindings/go/scip/sort.go`, `bindings/go/scip/testutil/format.go:84-88`
- Impact: Callers that canonicalize an index and then re-emit it silently reorder/merge the original; `Convert` in `cmd/scip/convert.go:276-277` writes the canonicalized doc back into the caller's slice.
- Fix approach: Document the mutation on each exported symbol (several already note it), or add `Clone`-based variants for external consumers.

**Deprecated dual range encoding kept in the schema:**
- Issue: `Occurrence` carries both the deprecated `repeated int32 range`/`enclosing_range` and the `typed_range`/`typed_enclosing_range` oneofs; every consumer must implement precedence rules ("typed wins") and producers must keep both forms equivalent if both set.
- Files: `scip.proto:680-745` (`range` field 1 and `enclosing_range` field 7 `[deprecated = true]`), `bindings/go/scip/occurrence_range.go`, `bindings/go/scip/canonicalize.go:49-56`
- Impact: Divergent handling across the six language bindings silently produces different navigation results; `scip.proto:214` also aliases `Package = Namespace = 1` (`allow_alias`), another interop trap.
- Fix approach: Keep emitting migration guidance; add a `scip lint` check that flags occurrences carrying both forms with non-equivalent values.

**Outstanding TODO markers:**
- Issue: `_ = 0 // TODO - warn?` silently swallows a document-language mismatch during flattening; `TODO: Enable exhaustive in CI` on the parser error enum; `TODO - better data` in sort tests; `TODO: Replace with cmp.Compare` (now possible — repo is on Go 1.25).
- Files: `bindings/go/scip/flatten.go:14-16`, `bindings/go/scip/symbol_parser.go:317`, `bindings/go/scip/sort_test.go:107`, `bindings/go/scip/position_test.go:10`
- Impact: Conflicting-language documents merge without any signal; enum switch exhaustiveness is unverified.
- Fix approach: Wire the flatten warning into the lint error set; adopt `golang.org/x/tools/go/analysis` exhaustive or the `exhaustive` linter in CI; use `cmp.Compare`/`slices` helpers.

**Library package registers a command-line flag:**
- Issue: `testutil` registers `flag.Bool("update-snapshots", ...)` at package init. Any binary importing `testutil` (reprolang does, via its test helper usage) grows a surprise flag, and importing `testutil` into a non-test binary panics on flag redefinition.
- Files: `bindings/go/scip/testutil/snapshot_testing.go:18`
- Impact: Surprising coupling for downstream indexers that use `testutil` in their own CLIs.
- Fix approach: Gate the flag behind an exported `RegisterFlags(*flag.FlagSet)` or an `init()` restricted to test binaries via a build tag.

## Known Bugs

**`scip.ParseSymbol` panics on symbols ending with a multi-byte rune (VERIFIED):**
- Symptoms: `runtime error: index out of range` from the exported API for inputs like `"a b c d fooΩ"`, `"test . pkg . barΩ"`, `"a b c d \`foo\`Ω"`. Reproduced against this tree on 2026-08-16. Because `scip lint` and `scip snapshot` call `ParseSymbol`/`Format` on every occurrence symbol, one malformed symbol in an index crashes the entire CLI instead of being reported as a lint error.
- Files: `bindings/go/scip/symbol_parser.go:374-379` (`peekNext` guards with `z.byteIndex+1 < len(...)` but indexes `z.byteIndex + int(z.bytesToNextRune)`), reached from `parseDescriptor`'s unconditional `advanceRune` at `symbol_parser.go:431`; CLI crash paths `cmd/scip/lint.go:253` and `bindings/go/scip/symbol_formatter.go:57-63`.
- Trigger: Any global symbol string whose final rune is multi-byte UTF-8 and not a valid descriptor suffix (real indexers can emit such names in string-literal/meta descriptors).
- Workaround: None for callers; the symbol must be trimmed/re-encoded before parsing.
- Fix: Change the guard to `z.byteIndex+int(z.bytesToNextRune) < len(z.SymbolString)`; return `unrecognizedDescriptorError` instead of panicking; add fuzz + regression tests.

**`scip stats` never counts lines of code when the project root exists locally (VERIFIED):**
- Symptoms: `scip stats` logs `Couldn't count lines of code: stat : no such file or directory` and reports `"linesOfCode": 0` for an index whose `Metadata.ProjectRoot` points at an existing local directory. Reproduced against this tree on 2026-08-16. The `else` branch that should set `localSource = root.Path` is missing, so `localSource` stays empty whenever `os.Stat(root.Path)` succeeds.
- Files: `cmd/scip/stats.go:160-174` (`countLinesOfCode`), non-fatal fallback at `cmd/scip/stats.go:108-118`
- Trigger: Run `scip stats` on any index with a locally-valid `project_root` and no `--project-root` flag.
- Workaround: Pass `--project-root=<folder>` explicitly (that branch works).
- Fix: Add `else { localSource = root.Path }`, or restructure to `localSource = customProjectRoot` / fallback logic; add a unit test for `countLinesOfCode` (currently none exists).

**`NO_COLOR` handling inverted relative to the no-color.org spec:**
- Symptoms: With `NO_COLOR=1` color output stays enabled; with `NO_COLOR=0/false/off` it is disabled — the value is treated as a truthy flag, but the spec says any non-empty value must disable color.
- Files: `cmd/scip/print.go:43-52`
- Trigger: `NO_COLOR=1 scip print index.scip` still emits ANSI codes.
- Workaround: Pipe through an external filter.
- Fix: `if _, found := os.LookupEnv("NO_COLOR"); found { colorOutput = false }` (keeping the empty-string exemption if desired).

**`duplicateSymbolInfoWarning.Error()` message branches swapped:**
- Symptoms: For external symbols (empty path) lint prints `found repeated SymbolInformation for 'X' in ''`; for document symbols it prints `found repeated SymbolInformation for external symbol 'X'` — the two messages are exchanged.
- Files: `cmd/scip/lint.go:275-286`
- Trigger: Any `scip lint` run reporting duplicate symbol information.
- Workaround: None (misleading diagnostic only).
- Fix: Swap the two format strings.

**Wrong variable in stats error message:**
- Symptoms: JSON marshal failure prints `failed to marshall into JSON map[]` — it interpolates `output` (an always-empty map) instead of the stats object.
- Files: `cmd/scip/stats.go:57-60`
- Trigger: A `json.MarshalIndent` failure (extremely large/unencodable stats).
- Fix: Interpolate `indexStats`; fix typo "marshall" while there.

**`Stats.Percentiles` field missing a json tag:**
- Symptoms: Output contains mixed casing — top-level keys are camelCase but the nested object is `"Percentiles": { "50": ... }`.
- Files: `cmd/scip/stats.go:65-78`
- Trigger: Every `scip stats` invocation; breaks consumers expecting `percentiles`.
- Fix: Add `json:"percentiles"` (note: output-format change — announce in release notes).

**`defer scipFile.Close()` registered before the open-error check:**
- Symptoms: If `os.Open` fails, `Close` is invoked on a nil `*os.File` (returns `ErrInvalid`, ignored) — benign today but a latent nil-deref pattern; `Close` errors are also always swallowed.
- Files: `cmd/scip/option_from.go:23-27`
- Fix: Move the defer after the error check and capture the close error.

**`scip test`/`scip snapshot` crash on malformed test-file comments (panic, not error):**
- Symptoms: A test file comment line whose first non-marker word is not one of the five known kinds (e.g. `// ^Hello world`) hits `panic("Unknown symbolAttributeKind: Hello")`; a `_` marker on a non-`synthetic_definition` kind panics too.
- Files: `bindings/go/scip/testutil/test_runner.go:206`, `bindings/go/scip/testutil/test_runner.go:438-439`
- Trigger: Run `scip test` over a directory containing an ordinary comment that begins with `^`/`_` followed by an unknown word.
- Workaround: Edit the test file.
- Fix: Return an error naming the file and line instead of panicking.

**`FormatSnapshots` dereferences `index.Metadata` without a nil check:**
- Symptoms: `scip snapshot` on a metadata-less index panics with a nil-pointer dereference (`stats` guards this, snapshot does not).
- Files: `bindings/go/scip/testutil/format.go:25`
- Fix: Mirror the `index.Metadata == nil` check used in `cmd/scip/stats.go:49-51`.

## Security Considerations

**Destructive `os.RemoveAll` on a CLI-supplied path:**
- Risk: `scip snapshot` begins by recursively deleting the `--to` directory (default `scip-snapshot/`). A typo (`--to .` or an absolute path) irreversibly deletes arbitrary trees; there is no confirmation or dry-run.
- Files: `cmd/scip/snapshot.go:68` (also `bindings/go/scip/testutil/snapshot_testing.go:30` under `-update-snapshots`)
- Current mitigation: None beyond documentation; the flag has no path validation.
- Recommendations: Refuse to remove paths that are empty, `.`, `/`, or equal to `$HOME`/cwd; or write to a temp dir and swap atomically.

**Path traversal through `Document.RelativePath` from untrusted indexes:**
- Risk: SCIP indexes are untrusted input. `snapshot` writes to `filepath.Join(output, snapshot.RelativePath)` after `MkdirAll` — a relative path like `../../.zshenv` writes outside the output directory. `scip test` reads `filepath.Join(directory, document.RelativePath)` — arbitrary local file read. `FormatSnapshot` likewise reads from `projectRoot + RelativePath`.
- Files: `cmd/scip/snapshot.go:84-92`, `bindings/go/scip/testutil/test_runner.go:57-69`, `bindings/go/scip/testutil/format.go:42`, `bindings/go/scip/source_file.go:29-50`
- Current mitigation: None; `RelativePath` is used as-is.
- Recommendations: Validate that cleaned joined paths remain within the intended root (`filepath.Rel` + prefix check); treat indexes downloaded from CI artifacts as hostile.

**Streaming parser trusts length prefixes before bounding allocation:**
- Risk: `ParseStreaming` allocates `make([]byte, dataLen)` from a varint length before reading any bytes — a corrupt or malicious 10-byte varint can request a multi-GB allocation (memory-exhaustion DoS). On 32-bit builds `int(dataLenUint)` can also go negative.
- Files: `bindings/go/scip/parse.go:58-71`
- Current mitigation: `io.ReadAtLeast` then fails, but only after the allocation.
- Recommendations: Grow `dataBuf` incrementally while streaming reads, or cap `dataLen` (e.g. 1 GiB) with a clear error.

**Secrets handling in release automation:**
- Risk: Standard registry tokens (`HACKAGE_TOKEN`, `CRATES_TOKEN`, Maven creds, GPG key) are used in workflows; token strings are interpolated into shell (`cargo publish --token '...'`) which can leak on error output in some setups.
- Files: `.github/workflows/release.yaml:95-140` (crates token on the command line), Maven publishing job
- Current mitigation: `permissions:` blocks are scoped read/contents-write as appropriate; workflow only triggers on `version.txt` changes on main.
- Recommendations: Prefer environment-variable injection for the crates token (`--token` reading `env:`) to keep it out of argv.

**SQLite converter:**
- Risk: Low — all statements are parameterized; no user SQL interpolation.
- Files: `cmd/scip/convert.go`
- Current mitigation: Prepared statements, `PRAGMA strict`.
- Recommendations: None needed beyond fixing the `panic`/`log.Fatal` paths listed under Tech Debt.

## Performance Bottlenecks

**Whole-index in-memory reads for every CLI command:**
- Problem: `readFromOption` does `io.ReadAll` + `proto.Unmarshal` of the entire index for lint/print/stats/snapshot/test/convert, even though a streaming parser exists.
- Files: `cmd/scip/option_from.go:16-42`, `bindings/go/scip/parse.go:29-122` (`IndexVisitor.ParseStreaming`, used only by `bindings/go/scip/memtest/low_mem_test.go`)
- Cause: Streaming path not wired into the CLI; per-command needs differ (lint needs global symbol tables anyway, but convert/stats/print could stream).
- Improvement path: Route `print`/`stats`/`convert` through `ParseStreaming`; keep lint on the full load but document the memory profile. The memtest proves 1000x128KB documents parse under a tight memory limit.

**Statement-per-insert plus per-doc `Prepare` in convert:**
- Problem: see Tech Debt entry; also `insertOccurrenceData` prepares the mentions statement once per document but `insertGlobalSymbols` prepares per symbol.
- Files: `cmd/scip/convert.go:323, 447`
- Cause: Prepared statements created inside loops.
- Improvement path: Hoist preparation to converter construction; batch mentions with multi-VALUES inserts.

**O(n²) documentation dedup:**
- Problem: `combineDocumentation` uses `stringSliceContains` (linear scan per element) — quadratic on symbols with many doc strings.
- Files: `bindings/go/scip/flatten.go:110-130`
- Improvement path: Deduplicate with a `map[string]struct{}` per merge.

**`scip stats` serializes every document just to measure size:**
- Problem: `proto.Marshal(document)` per document purely to record byte size; for huge indexes this doubles CPU and allocates the full encoded buffer per document.
- Files: `cmd/scip/stats.go:128-140`
- Improvement path: Use `proto.Size(document)` (no allocation, same number).

**Single-threaded everything:**
- Problem: Lint, convert, snapshot are all single-goroutine; lint builds full symbol/occurrence maps before reporting.
- Files: `cmd/scip/lint.go:41-104`, `cmd/scip/convert.go:258-318`
- Improvement path: Parallelize per-document passes; the occurrence map in lint could shard by document.

**Non-deterministic document iteration in convert:**
- Problem: `Convert`'s second pass iterates `docPositions` (a map), so insertion order of chunks/mentions varies run to run — same input, different DB byte layout (hurts reproducibility testing and diffing).
- Files: `cmd/scip/convert.go:303-313`
- Improvement path: Sort document positions by relative path (or original index) before the second pass.

## Fragile Areas

**Symbol parser internals:**
- Files: `bindings/go/scip/symbol_parser.go`
- Why fragile: Manual rune-index arithmetic with multiple documented pre-conditions; assertions compiled out in release; one confirmed OOB bug.
- Safe modification: Add a regression test per fix; keep the fuzz corpus growing; avoid touching `stringWriter`/`descriptorsWriter` slot-reuse logic without the allocation test in `bindings/go/scip/memtest/low_mem_test.go` green.
- Test coverage: Good on happy paths (`symbol_test.go`, `symbol_formatter_test.go`); no fuzzing; edge cases around multi-byte runes untested (proven by the shipped panic).

**Test-file parser in `testutil`:**
- Files: `bindings/go/scip/testutil/test_runner.go:222-480`
- Why fragile: Comment-line grammar is heuristic (marker runs, `<-`, `<n>:<c>` suffixes, `>` continuation lines); unknown tokens panic; a harmless comment can crash a test run; marker semantics are coupled to snapshot formatting via `markerLength`/`renderLine` in `format.go`.
- Safe modification: Change parser and formatter together (they share `syntheticDefinitionsByParent` to avoid drift); add golden files under `reprolang/testdata/test_cmd`.
- Test coverage: 7 tests in `test_runner_test.go` + autogold snapshots, but no coverage of malformed-input paths.

**Dual range encoding (`typed_range` vs deprecated `range`):**
- Files: `bindings/go/scip/occurrence_range.go`, `scip.proto:680-745`
- Why fragile: Every consumer (4 CLI subcommands, external indexers, Rust/TS/Haskell bindings) must apply the same precedence; malformed legacy lengths are silently treated as "missing" (`SourceRange` returns false).
- Safe modification: Any change must update `SourceRange`, `EnclosingSourceRange`, `CanonicalizeOccurrence`, and the proto comment contract simultaneously.
- Test coverage: `occurrence_range_test.go` covers the matrix well.

**CLI flag/global state:**
- Files: `cmd/scip/main.go:35` (`Reproducible` global set from `TestMain`), `cmd/scip/lint.go:247`
- Why fragile: Tests mutate production globals (`Reproducible = "true"` in `main_test.go`), making test order significant.
- Safe modification: Keep `TestMain` as the only writer.
- Test coverage: `TestCLIReferenceInSync` guards docs drift for `--help`.

**`SourceFile.RangeText` unchecked indexing:**
- Files: `bindings/go/scip/source_file.go:70-85`
- Why fragile: Panics with index-out-of-range when a range references lines/columns beyond the file (stale index vs edited source) — common when indexes are generated on another machine.
- Safe modification: Clamp line/character indices before slicing.
- Test coverage: `source_file_test.go` covers valid ranges only.

**Generated bindings across six ecosystems:**
- Files: `bindings/go/scip/scip.pb.go`, `bindings/typescript/scip_pb.ts`, `bindings/rust/src/generated/scip.rs`, `bindings/haskell/src/Proto/Scip.hs`, `bindings/java`, `bindings/kotlin`
- Why fragile: Committed generated code (3.3k–10.5k lines per binding) can drift from `scip.proto` if someone edits outputs by hand or skips `buf generate`.
- Safe modification: Never hand-edit; regenerate via the Nix flake.
- Test coverage: CI `packages` job (`git diff --exit-code` after `nix run`) plus `checks.*-bindings` attributes in `checks.nix` enforce regeneration parity — good.

## Scaling Limits

**In-memory index size:**
- Current capacity: Comfortable for indexes up to ~1-2 GB of heap (whole-index commands); Chromium-class indexes (multi-GB) will stress or OOM `lint`/`snapshot`.
- Limit: `io.ReadAll` + full `proto.Unmarshal` in `cmd/scip/option_from.go:31-41`.
- Scaling path: Stream documents via `IndexVisitor.ParseStreaming` (already proven in `bindings/go/scip/memtest/low_mem_test.go`); make lint's global tables disk-backed if needed.

**int32 counters in `scip stats`:**
- Current capacity: `Stats.Mean/Sum/Max` and `indexStatistics.Occurrences/Definitions` are int32; overflow at ~2.1B occurrences — reachable for very large monorepos (Sum of occurrences across documents).
- Limit: Silent wraparound produces negative counts.
- Scaling path: Switch to int64 (JSON output change).

**Convert single transaction:**
- Current capacity: One `ImmediateTransaction` covers the entire conversion; SQLite WAL grows with the whole index.
- Limit: Multi-GB indexes hold the transaction open for the full run; a crash discards everything.
- Scaling path: Commit per document batch; index creation is already deferred to `prepareIndexes`.

**CI matrix:**
- Current capacity: Linux x86_64 only (`nix build .#checks.x86_64-linux...` in `.github/workflows/ci.yaml:139-160`); release binaries cover more platforms.
- Limit: No aarch64-linux, no Windows, no macOS checks in main CI.
- Scaling path: Add `checks.aarch64-linux` (cheap with Nix) and a Windows job for path-handling code (`filepath.Join` semantics differ).

## Dependencies at Risk

**`github.com/sourcegraph/beaut` (pre-release pseudo-version):**
- Risk: `v0.0.0-20240611...` — no tagged releases, single-vendor maintenance; core of the hot symbol-parsing path.
- Impact: Symbol parsing API churn breaks this repo and all downstream indexers.
- Migration plan: The used surface is small (`UTF8String`); vendor the needed functions or pin via go.mod and schedule periodic reviews.

**`github.com/hexops/gotextdiff` + `hexops/autogold` (reprolang only):**
- Risk: Hexops repos are archived/unmaintained; `autogold` is test-only so blast radius is limited to `reprolang` snapshots.
- Migration plan: Move to `github.com/google/go-cmp` diffs or `patch-diff-match-split`; keep out of the main module (already confined to `reprolang/go.mod`).

**Rust `protobuf = "=3.7.2"` exact pin:**
- Risk: Must match the codegen version (comment in `bindings/rust/Cargo.toml`); exact pin forces every downstream crate to the same protobuf major/minor, conflicting with other protobuf users in the same dependency graph.
- Migration plan: None urgent (intentional); track upstream releases and bump pin + regenerate together.

**Two SQLite stacks in the main module:**
- Risk: `zombiezen.com/go/sqlite` (used) and `modernc.org/sqlite` (indirect via zombiezen→ncruces) both appear in `go.mod`; version skew between them causes subtle bugs.
- Migration plan: Acceptable as-is; keep an eye on zombiezen release notes when bumping.

**TypeScript `devDependencies: typescript ^7.0.0` and `@bufbuild/protobuf ^2.11.0`:**
- Risk: TS 7 is the native-port line — build tooling may behave differently across environments; bufbuild major 2 pins the runtime API shape of `scip_pb.ts`.
- Migration plan: Keep `buf generate` as the single source (CI enforces); bump both together.

**`github.com/go-enry/go-oniguruma` (indirect, via gocloc):**
- Risk: C-binding (oniguruma) — cgo in the stats path; breaks cross-compilation of the CLI when CGO is disabled.
- Migration plan: Not directly controllable; if cross-compilation pain appears, `scip stats` LOC counting could switch to a pure-Go counter.

## Missing Critical Features

**No fuzzing for the hand-written parsers:**
- Problem: `ParseSymbol` and the test-file grammar are exactly the kind of code fuzzing exists for; the shipped OOB panic would have been caught on day one.
- Blocks: Confidence in robustness for arbitrary/malicious indexes.

**No reader/query tooling for the SQLite conversion:**
- Problem: `expt-convert` writes a DB schema but there is no `scip query`/lookup command to consume it (only "use the SQLite CLI").
- Blocks: The experimental feature cannot be evaluated end-to-end without hand-written SQL.

**No `--from -` support documented for all commands / extension strictness:**
- Problem: `readFromOption` refuses files without a `.scip` extension (`cmd/scip/option_from.go:20-22`) even though stdin bypasses it; awkward for pipelines renaming artifacts.
- Blocks: CI pipelines that produce `index.pb`-style artifacts.

**Lint cannot skip warning classes:**
- Problem: The command description suggests `grep -v` as the filtering mechanism; there are no severity flags, and dedup (`Unique`) is unwired.
- Blocks: Adoption in CI where only errors (not warnings) should fail builds.

## Test Coverage Gaps

**`cmd/scip/stats.go` — zero direct tests:**
- What's not tested: `countStatistics`, `countLinesOfCode`, `NewStats` (empty-slice behavior: `stats.Max([])` errors are swallowed and NaN→int32 conversion yields platform-dependent values).
- Files: `cmd/scip/stats.go`
- Risk: The verified lines-of-code bug shipped; JSON schema drift (`Percentiles` casing) unnoticed.
- Priority: High

**Symbol parser adversarial inputs:**
- What's not tested: Multi-byte runes at buffer edges, backtick escaping edge cases, fuzz corpus.
- Files: `bindings/go/scip/symbol_parser.go`, `bindings/go/scip/symbol_test.go`
- Risk: Panics reachable from `scip lint`/`scip snapshot` on third-party indexes (verified).
- Priority: High

**testutil malformed-input handling:**
- What's not tested: Unknown attribute kinds, `_` misuse, comments that look like assertions; `parseRangeSelection`/`parseMultilineSuffix` unit behavior.
- Files: `bindings/go/scip/testutil/test_runner.go`
- Risk: Panic-instead-of-error behavior on user-authored test files.
- Priority: Medium

**`scip print` color/NO_COLOR behavior:**
- What's not tested: The inverted NO_COLOR logic.
- Files: `cmd/scip/print.go:43-52`, `cmd/scip/print_test.go`
- Risk: Spec violation shipped (verified by inspection).
- Priority: Medium

**Convert robustness:**
- What's not tested: Duplicate relative paths warning path, `chunkOccurrences` boundary sizes (0/1/200/201 occurrences, same-line chunk extension), `log.Fatal` branch in `insertGlobalSymbols`, occurrence referencing a symbol never seen in `doc.Symbols` (synthetic insert path at `cmd/scip/convert.go:344-349`).
- Files: `cmd/scip/convert.go`, `cmd/scip/convert_test.go` (single happy-path smoke test)
- Risk: Panics/fatals on real-world messy indexes.
- Priority: Medium

**Rust/TypeScript/Haskell/Java/Kotlin bindings:**
- What's not tested: Rust has solid `symbol.rs` unit tests, but TS/Haskell/JVM bindings have no tests in this repo (TS is typecheck-only per `package.json` scripts; Haskell/JVM rely on protoc correctness).
- Files: `bindings/typescript/`, `bindings/haskell/`, `bindings/java/`, `bindings/kotlin/`
- Risk: Regressions in generated-code pipelines surface only in downstream consumers.
- Priority: Low (generated code; CI regen checks mitigate)

---

*Concerns audit: 2026-08-16*
