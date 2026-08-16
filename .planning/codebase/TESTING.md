# Testing Patterns

**Analysis Date:** 2026-08-16

## Test Framework

**Runner:**
- Go standard library `testing` package, Go 1.25.
- Three modules under `go.work` (root, `bindings/go/scip`, `reprolang`) — each has its own test suite; run them per-module or via the workspace.
- Config: no test config files; flags are defined in-test (`-update-snapshots`, `-update`, `-debug-snapshot-abspaths`).

**Assertion Library:**
- `github.com/stretchr/testify` v1.11.1 — almost exclusively `require.*` (fail-fast), rarely `assert.*`.
- `github.com/google/go-cmp/cmp` for structural diffs.
- `pgregory.net/rapid` v1.3.0 for property-based tests.
- `github.com/google/gofuzz` for fuzz-generated struct inputs.
- `github.com/hexops/autogold/v2` for inline golden values.
- `github.com/hexops/gotextdiff` + `myers` for unified snapshot diffs.

**Run Commands:**
```bash
go test ./...                                  # Run all tests (root module: cmd/scip)
go test ./bindings/go/scip/...                 # Library module tests
go test ./reprolang/...                        # Reprolang indexer + grammar tests
go test ./cmd/scip -update-snapshots           # Regenerate snapshot goldens (docs/Development.md)
go test ./reprolang/repro -update              # Update autogold values (test_cmd_test.go)
go test -tags asserts ./bindings/go/scip/...   # Run with internal assert() panics enabled
go vet ./...                                   # Vet
gofmt -d . && goimports -d .                   # Format check (CI: checks.nix 'formatting')
go test -cover ./bindings/go/scip/...          # Coverage (no target enforced)
nix build .#checks.x86_64-linux.go-bindings    # CI-equivalent: builds & tests with -tags asserts
```

## Test File Organization

**Location:**
- Co-located with source, same package (white-box tests): `bindings/go/scip/symbol_test.go` is `package scip`, `cmd/scip/lint_test.go` is `package main`.
- Shared harnesses live in a public sub-package `bindings/go/scip/testutil/` (which also has its own tests).
- Memory-limit tests are quarantined in `bindings/go/scip/memtest/` with a `DO_NOT_ADD_NEW_TEST_FILES_HERE` marker — `debug.SetMemoryLimit` must not run alongside other tests.

**Naming:**
- `<file>_test.go` mirroring the source file; test functions `TestXxx`, often prefixed `TestUnit...` for pure unit tests (`TestUnitComparePosition`, `TestUnitIntersect` in `bindings/go/scip/position_test.go`).

**Structure:**
```
bindings/go/scip/            # library tests (symbol_test.go, parse_test.go, ...)
bindings/go/scip/testutil/   # test harness package + its tests (format_test.go, test_runner_test.go)
bindings/go/scip/memtest/    # isolated memory/alloc tests (DO_NOT_ADD_NEW_TEST_FILES_HERE)
cmd/scip/                    # CLI tests (main_test.go, lint_test.go, convert_test.go, print_test.go)
reprolang/repro/             # indexer tests (snapshot_test.go, test_cmd_test.go)
reprolang/grammar/           # tree-sitter binding tests (binding_test.go)
reprolang/testdata/
  snapshots/input/<case>/    # snapshot test inputs (*.repro sources)
  snapshots/output/<case>/   # snapshot goldens (regen with -update-snapshots)
  test_cmd/<suite>/          # annotation tests; files named passes* / fails*
```

## Test Structure

**Suite Organization:**
Table-driven tests with inline case structs, executed via `t.Run` subtests named after the input (go):
```go
// bindings/go/scip/symbol_test.go
func TestParseSymbol(t *testing.T) {
	type test struct {
		Symbol   string
		Expected *Symbol
	}
	tests := []test{
		{Symbol: "local a", Expected: &Symbol{
			Scheme:      "local",
			Descriptors: []*Descriptor{{Name: "a", Suffix: Descriptor_Local}},
		}},
		// ... more cases
	}
	for _, test := range tests {
		t.Run(test.Symbol, func(t *testing.T) {
			obtained, err := ParseSymbol(test.Symbol)
			require.Nil(t, err)
			if diff := cmp.Diff(test.Expected.String(), obtained.String()); diff != "" {
				t.Fatalf("unexpected response (-want +got):\n%s", diff)
			}
		})
	}
}
```

**Patterns:**
- Setup pattern: build fixtures in helper factories (`makeIndex`, `testIndex1`) or load `testdata/` directories; use `t.TempDir()` for scratch files (`cmd/scip/convert_test.go`, `bindings/go/scip/memtest/low_mem_test.go`).
- Teardown pattern: `defer os.RemoveAll(...)` / `defer func() { require.NoError(t, db.Close()) }()`; env-var cleanup via `t.Cleanup(func() { os.Unsetenv("NO_COLOR") })` (`reprolang/repro/test_cmd_test.go`); restore swapped globals in `TestMain` or deferred closures (`os.Stdout` swap in `cmd/scip/main_test.go`).
- Assertion pattern: `require.NoError(t, err)` for preconditions; `cmp.Diff` with `-want +got` annotation for structures; `require.Truef/Equalf` with a format message describing the invariant (`require.Truef(t, r1.Intersects(r2), "%+v.Intersects(%+v)", r1, r2)`).
- Generic unwrap helper to flatten error handling in tests (`reprolang/repro/test_cmd_test.go`):
  ```go
  func unwrap[T any](v T, err error) func(*testing.T) T {
      return func(t *testing.T) T {
          require.NoError(t, err)
          return v
      }
  }
  cwd := unwrap(os.Getwd())(t)
  ```
- `TestMain` mutates globals before the run: `func TestMain(m *testing.M) { Reproducible = "true"; os.Exit(m.Run()) }` (`cmd/scip/main_test.go`).

## Mocking

**Framework:** None. The codebase avoids mocks in favor of fakes, in-memory buffers, and pure cores.

**Patterns:**
- **Pure-core extraction**: CLI logic is split so `lintMain` (I/O) delegates to `lintMainPure(*scip.Index) errorSet` which tests call directly — no filesystem, no mocks (`cmd/scip/lint.go`, tested in `cmd/scip/lint_test.go`).
- **Capture output in a `bytes.Buffer`** instead of mocking writers: `testutil.RunTests(..., &passOutput)` then compare the buffer (`reprolang/repro/test_cmd_test.go`).
- **Redirect os.Stdout** when exercising real CLI help output (`cmd/scip/main_test.go`):
  ```go
  r, w, err := os.Pipe()
  require.Nil(t, err)
  origStdout := os.Stdout
  os.Stdout = w
  defer func() { os.Stdout = origStdout }()
  ```
- **Callback-style visitors** stand in for collaborator interfaces: `scip.IndexVisitor{VisitDocument: func(_ context.Context, d *scip.Document) error {...}}` (`bindings/go/scip/memtest/low_mem_test.go`).
- **Mini-DSL factories** for protobuf fixtures rather than builders/mocks — `parseSymbolInfo("a~b#rd")` compactly encodes symbols+relationships and `makeIndex(extSyms, docSyms, docOccs)` assembles an `*scip.Index` (`cmd/scip/lint_test.go`).

**What to Mock:**
- Only process boundaries you cannot construct cheaply (stdout pipes, temp files). Prefer injecting an `io.Writer` or an `indexFunction` callback (`testutil.SnapshotTest(t, dir, func(inputDir, outputDir string, sources []*scip.SourceFile) []*scip.SourceFile {...})`).

**What NOT to Mock:**
- The `scip` library itself, protobuf marshal/unmarshal, or the SQLite layer — `TestConvert_SmokeTest` uses a real (in-memory/temp-file) SQLite database via `zombiezen.com/go/sqlite` (`cmd/scip/convert_test.go`).

## Fixtures and Factories

**Test Data:**
- Protobuf fixture factory with symbol-string DSL (go):
  ```go
  // cmd/scip/lint_test.go
  var placeholderRange = []int32{0, 0, 0}
  const placeholderRole int32 = 0

  func makeIndex(extSyms []string, docSyms stringMap, docOccs stringMap) *scip.Index {
      // parseSymbolInfo("x~a#r") => SymbolInformation{Symbol: "x",
      //   Relationships: [{Symbol: "a", IsReference: true}]}
      ...
      return &scip.Index{Documents: docs, ExternalSymbols: scipExtSyms}
  }
  ```
- Inline expected values as `autogold.Expect("✓ passes.repro (3 assertions)\n")` for command output (`reprolang/repro/test_cmd_test.go`).
- Snapshot goldens are real `.repro` source files under `reprolang/testdata/snapshots/output/`, generated from `testdata/snapshots/input/` counterparts.
- Annotation test files use Sublime-style caret markers in comments, interpreted relative to the code line above (from `reprolang/testdata/test_cmd/roles/passes.repro`):
  ```
  definition hello().
  #            ^ definition reprolang repro_manager roles 1.0.0 passes.repro/hello().
  ```
  Marker semantics (documented in `docs/test_file_format.md`): `^` ignores length, `^^^` (2+) enforces length, `<-` anchors to the first comment character, `___` is reserved for `synthetic_definition`, `>` continues multiline diagnostic messages, and a `.` token in the expected symbol acts as a wildcard.

**Location:**
- `reprolang/testdata/` (snapshot input/output, `test_cmd` suites). Directory names are test names; input dirs are siblings acting as cross-package dependencies (`reprolang/repro/snapshot_test.go` indexes every sibling dir as a `Dependency`).
- CLI goldens live in `docs/CLI.md` — see `TestCLIReferenceInSync` in `cmd/scip/main_test.go`.

## Coverage

**Requirements:** None enforced. No coverage thresholds in CI.

**View Coverage:**
```bash
go test -cover ./bindings/go/scip/...
go test -coverprofile=cover.out ./cmd/scip/ && go tool cover -html=cover.out
```

**CI execution:** tests run through Nix (`checks.nix`): `go-bindings` builds subPackages `.`, `memtest`, `testutil` with `buildTags = [ "asserts" ]` (buildGoModule runs `go test` in its check phase); `reprolang` and the root `packages.scip` (cmd/scip) modules are built/tested the same way with `GOWORK=off` and vendored deps. Formatting (gofmt/goimports/buf/prettier/nixfmt) is a separate check. No golangci-lint.

## Test Types

**Unit Tests:**
- Dominant style. Pure-function tables (`TestUnitComparePosition`), parser round-trips (`checkRoundtrip` in `bindings/go/scip/parse_test.go`), error-path enumeration (`TestParseSymbolError` iterates malformed symbol strings with `require.NotPanics` + error presence checks).
- Property-based tests with rapid for algebraic invariants (`bindings/go/scip/position_test.go`):
  ```go
  func genRange() *rapid.Generator[Range] {
      return rapid.Custom(func(t *rapid.T) Range {
          posGen := genPosition()
          start := posGen.Draw(t, "start")
          end := posGen.Draw(t, "end")
          if start.Compare(end) > 0 { start, end = end, start }
          return Range{Start: start, End: end}
      })
  }

  func TestIntersects(t *testing.T) {
      rapid.Check(t, func(t *rapid.T) {
          r1 := genRange().Draw(t, "r1")
          r2 := genRange().Draw(t, "r2")
          if r1.Intersects(r2) {
              if !(r1.Contains(r2.Start) || r2.Contains(r1.Start)) {
                  t.Errorf("%+v overlaps with %+v but neither contains the others start position", r1, r2)
              }
          }
      })
  }
  ```
- Fuzz-style loop with gofuzz (100 random `Index` values, skipping unexported/ignored fields) then round-trip (`TestFuzz`, `TestDocumentsOnly` in `bindings/go/scip/parse_test.go`):
  ```go
  pat := regexp.MustCompile("^(state|sizeCache|unknownFields|...)$")
  f := fuzz.New().NumElements(0, 2).SkipFieldsWithPattern(pat)
  for i := 0; i < 100; i++ {
      index := Index{}
      f.Fuzz(&index)
      checkRoundtrip(t, &index)
  }
  ```
- Resource tests: memory ceiling via `debug.SetMemoryLimit` asserting streaming parse does not OOM; allocation-free assertions comparing `runtime.MemStats.TotalAlloc` before/after (`bindings/go/scip/memtest/low_mem_test.go`). Keep such tests in the isolated `memtest` package only.

**Integration Tests:**
- CLI end-to-end: build the real `scipApp()`, marshal an index to a temp `.scip` file, run `app.Run(context.Background(), []string{"scip", "print", "--json", file})`, parse the JSON output back (`TestJSONPrinting`, `cmd/scip/print_test.go`).
- SQLite conversion smoke test against a real database with per-table check subtests (`TestConvert_SmokeTest`, `cmd/scip/convert_test.go`).
- Docs-sync test: `--help` output of every command must appear verbatim in `docs/CLI.md` (`TestCLIReferenceInSync`, `cmd/scip/main_test.go`).
- Indexer integration: `TestSCIPSnapshots` runs the full reprolang indexer over every `testdata/snapshots/input/<case>` directory, formats snapshots, and diffs against `testdata/snapshots/output` goldens (`reprolang/repro/snapshot_test.go` + `bindings/go/scip/testutil/snapshot_testing.go`).

**E2E Tests:**
- The annotation-based `scip test` runner is itself E2E: `reprolang/repro/test_cmd_test.go` indexes each `testdata/test_cmd/<suite>` directory and runs `testutil.RunTests` against files named `passes*` (must succeed, output matched with autogold) and `fails*` (must error, failure output matched). Adding a testdata directory without a `TestCase` entry (or vice versa) fails the test with "Missing entry"/"Stale entry" messages.

## Common Patterns

**Async Testing:**
- Not much concurrency; where visitors are async-shaped, callback structs are used synchronously (`IndexVisitor`). Use `context.Background()` when invoking `app.Run` / `ParseStreaming` in tests.

**Error Testing:**
- Expected-error-set comparison: run `lintMainPure`, render all errors via `.Error()` into a `set[string]`, assert each expected typed error's message is present (`TestErrors`, `cmd/scip/lint_test.go`):
  ```go
  for _, testCase := range testCases {
      errMsgSet := errorMessages(lintMainPure(testCase.index))
      for _, expectErr := range testCase.expectedErrors {
          if !errMsgSet.Contains(expectErr.Error()) {
              t.Errorf("test %s failure: expected\n%s\nbut it wasn't present in error set:\n%s", ...)
          }
      }
  }
  ```
- Non-panic + error presence for parser error paths: `require.NotPanics(t, func() { if _, err := ParseSymbol(s); err == nil { t.Errorf(...) } })` (`bindings/go/scip/symbol_test.go`).
- Panic assertions for programmer-error invariants: `require.Panics(t, func() { parseTestCase("// ___ reference local child", ...) })` (`bindings/go/scip/testutil/test_runner_test.go`).

**Snapshot Testing:**
- Harness: `testutil.SnapshotTest(t, baseDir, indexFunction)` walks `baseDir/snapshots/input/*`, runs the indexer callback, and diffs formatted output against `snapshots/output/*` using Myers edits rendered as a unified diff; `-update-snapshots` deletes and rewrites goldens (`bindings/go/scip/testutil/snapshot_testing.go`).
- Custom `SymbolFormatter` (`scip.DescriptorOnlyFormatter` with an `IncludePackageName` filter) keeps goldens short and version-stable (`reprolang/repro/snapshot_test.go`).

**Adding new SCIP-semantics tests (prescriptive flow, from `docs/Development.md`):**
1. Add/extend a `.repro` input (+ expected annotations or output golden).
2. Implement the functionality.
3. Regenerate: `go test ./cmd/scip -update-snapshots` (or `go test ./reprolang/repro -update`).
4. Review the diff; commit inputs and goldens together.

---

*Testing analysis: 2026-08-16*
