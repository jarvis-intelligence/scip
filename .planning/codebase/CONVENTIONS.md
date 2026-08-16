# Coding Conventions

**Analysis Date:** 2026-08-16

**Language:** Go 1.25 (multi-module workspace) with a Protobuf schema (`scip.proto`) and generated bindings for TypeScript, Rust, Haskell, Java, Kotlin.

**Workspace layout** (`go.work`): three Go modules — root `github.com/scip-code/scip` (the CLI in `cmd/scip`), `bindings/go/scip` (published library module with its own `go.mod`), and `reprolang` (test language, own `go.mod`). The root module `replace`es the bindings module with the local checkout.

## Naming Patterns

**Files:**
- Use `snake_case.go` file names, one primary concern per file: `symbol.go`, `symbol_parser.go`, `symbol_formatter.go`, `occurrence_range.go`, `canonicalize.go` (all in `bindings/go/scip/`).
- Co-locate tests as `<file>_test.go`: `symbol_test.go`, `sort_test.go`, `lint_test.go`.
- Generated protobuf code lives in `scip.pb.go` — never hand-edit; regenerate with `nix run .#proto-generate` (see `flake.nix`).
- Pair a build-tag'd file with a no-op counterpart: `assertions.go` (`//go:build asserts`) and `assertions_noop.go` (`//go:build !asserts`).

**Functions:**
- Exported functions are `PascalCase`; use verb-prefixed names that say what they do: `ParseSymbol`, `ParseSymbolUTF8With`, `ValidateSymbolUTF8`, `FormatSnapshots`, `RunTests`, `SortOccurrences`, `FindOccurrences`, `NewSourcesFromDirectory`.
- Constructors use `NewX`: `NewConverter`, `NewRangeUnchecked`, `newSymbolTable`, `newSet`. Note lowercase `newX` for internal constructors (`newSet[T comparable]()`, `newSymbolTable()`).
- Predicates use `IsX`: `IsGlobalSymbol`, `IsLocalSymbol`; `HasPrefix`-style helpers follow stdlib idioms.
- In `cmd/scip/`, follow the three-tier CLI pattern per subcommand: `xxxCommand() cli.Command` (flag wiring), `xxxMain(...)` (I/O orchestration), `xxxMainPure(...)` (testable core without I/O) — see `lintCommand`/`lintMain`/`lintMainPure` in `cmd/scip/lint.go`, and `convertCommand`/`convertMain` in `cmd/scip/convert.go`.
- Use `SourceX`-style getters on protobuf messages: `occ.SourceRange()` returns `(scip.Range, bool)`; `pos.IsSingleLine()`.

**Variables:**
- `camelCase`; short receivers are the first letter or short word of the type (`func (f *SymbolFormatter)`, `func (e errorSet)`, `func (s symbolAttributeTestCase)`).
- Group flags into a struct per command: `type testFlags struct { from string; commentSyntax string; ... }` in `cmd/scip/test.go`, `type snapshotFlags struct {...}` in `cmd/scip/snapshot.go`.
- Use `lowerCamel` constants; exported sentinel vars are allowed at package level (`var Reproducible = "" // set by ldflags in CI` in `cmd/scip/main.go`).

**Types:**
- Struct error/warning types named `xxxError` / `xxxWarning` implementing `error`: `emptyStringError`, `nonCanonicalSymbolError`, `duplicateSymbolInfoWarning` in `cmd/scip/lint.go`.
- String-typed enum kinds with typed constants: `type symbolAttributeKind string` with `definitionAttrKind symbolAttributeKind = "definition"` in `bindings/go/scip/testutil/test_runner.go`.
- Prefer type aliases for unwieldy composite types: `type stringMap = map[string][]string` and `type occurrenceMap = map[occurrenceKey]*scip.Occurrence` (in `cmd/scip/lint_test.go` and `cmd/scip/lint.go`), `type indexFunction = func(...)` in `testutil/snapshot_testing.go`.
- Configuration via plain exported-struct options (no functional options): `ParseSymbolOptions{IncludeDescriptors bool, RecordOutput *Symbol}` in `bindings/go/scip/symbol.go`; `SymbolFormatter` in `bindings/go/scip/symbol_formatter.go` uses struct-of-functions for pluggable behavior.
- Visitor via callback fields, not interfaces: `scip.IndexVisitor{VisitDocument: func(...)}` (see `bindings/go/scip/memtest/low_mem_test.go`).
- Generics are welcome for small utilities: `set[T comparable]`, `mySlice[T]`, `setToString[T comparable]` in `cmd/scip/lint.go`; `unwrap[T any]` in `reprolang/repro/test_cmd_test.go`.

## Code Style

**Formatting:**
- `gofmt` and `goimports` are the authoritative formatters; enforced by the Nix `formatting` check in `checks.nix`:
  ```
  gofmt -d . | tee /dev/stderr | diff /dev/null -
  goimports -d . | tee /dev/stderr | diff /dev/null -
  ```
- Tabs for Go (gofmt default); no `.editorconfig` needed.
- Non-Go files: `prettier --check '**/*{ts,js(on)?,md,yml}'` with `.prettierrc` = `{ "semi": false, "trailingComma": "es5", "singleQuote": true, "useTabs": false, "arrowParens": "avoid" }`; `buf format --diff --exit-code scip.proto` for protobuf; `nixfmt --check *.nix` for Nix files.
- No golangci-lint configuration exists (`Not detected`). Lint enforcement is: gofmt/goimports diffs, `buf lint` (DEFAULT rules with exceptions for `PACKAGE_VERSION_SUFFIX`, `ENUM_VALUE_PREFIX`, `ENUM_VALUE_UPPER_SNAKE_CASE`, `ENUM_ZERO_VALUE_SUFFIX` in `buf.yaml`), `action-validator` for workflows, and version-consistency asserts across `cmd/scip/version.txt` / `bindings/*` manifests in `checks.nix`.

**Linting:**
- Protobuf breaking-change detection: `breaking: use: [FILE]` in `buf.yaml` — schema changes must be backward compatible at the file level.
- CI (`.github/workflows/ci.yaml`) matrix-builds every attribute of `checks` and `packages` from `flake.nix`; the `packages` job runs `nix run .#<pkg>` then `git diff --exit-code` so generated code (e.g. `proto-generate`) can never drift.

**Key style rules observed (prescriptive):**
- Use Go 1.21+ stdlib (`slices`, `sort` replaced by `slices.Sort`, `slices.Contains`, `slices.Equal`) — see `bindings/go/scip/testutil/test_runner.go`.
- Add a section comment banner for long files: `// --- Main types ---`, `// --- All possible errors ---`, `// --- Miscellaneous utility types ---` in `cmd/scip/lint.go`; `// --------------------------------- Utils ---------------------------------` in `test_runner.go`.
- Use build tags for expensive invariants: `assert()` panics under `-tags asserts`, is a no-op otherwise (`bindings/go/scip/assertions.go` / `assertions_noop.go`). Nix builds compile tests with `buildTags = [ "asserts" ]` (`checks.nix`).
- Embed static assets with `//go:embed version.txt` (`cmd/scip/main.go`); inject build metadata via ldflags `-X main.Reproducible=true` (`flake.nix`).

## Import Organization

**Order** (three groups, blank-line separated — goimports-compatible):
1. Standard library: `"context"`, `"fmt"`, `"os"`, ...
2. Third-party: `"github.com/urfave/cli/v3"`, `"github.com/stretchr/testify/require"`, `"google.golang.org/protobuf/proto"`
3. First-party: `"github.com/scip-code/scip/bindings/go/scip"` (always last, own group)

Example from `cmd/scip/lint.go`:
```go
import (
	"context"
	stderrors "errors"
	"fmt"
	"sort"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/scip-code/scip/bindings/go/scip"
)
```
- Alias the stdlib `errors` package as `stderrors` when a file also uses another errors-heavy import or to be explicit (`cmd/scip/lint.go`); otherwise import `errors` unaliased (`cmd/scip/convert.go`).
- Alias rarely: `fuzz "github.com/google/gofuzz"` in `bindings/go/scip/parse_test.go`.

**Path Aliases:**
- None; the bindings module path is the canonical import path `github.com/scip-code/scip/bindings/go/scip` (and `.../testutil`).

## Error Handling

**Patterns:**
- Return `error` as the last value; never ignore errors in library code.
- **Typed struct errors for user-facing diagnostics** — each lint finding is its own struct so tests can assert on exact messages (`cmd/scip/lint.go`):
  ```go
  type emptyStringError struct {
      what    string
      context string
  }

  func (e emptyStringError) Error() string {
      return fmt.Sprintf("error: found empty %s in %s", e.what, e.context)
  }
  ```
  Prefix rendered messages with `error:`, `warning:`, or `note:` to classify severity (see `duplicateSymbolInfoWarning`, `note` in `cmd/scip/lint.go`).
- **Aggregate** many errors with a custom container and `errors.Join`: `lintMain` does `return stderrors.Join(lintMainPure(scipIndex).data...)` after collecting into `errorSet` (`cmd/scip/lint.go`).
- **Wrap with context** using `fmt.Errorf("...: %w", err)` — see `readFromOption` in `cmd/scip/option_from.go`:
  ```go
  scipBytes, err := io.ReadAll(scipReader)
  if err != nil {
      return nil, fmt.Errorf("failed to read SCIP index at path %s: %w", fromPath, err)
  }
  ```
- **Validate arguments early** at the CLI boundary: `if indexPath == "" { return stderrors.New("missing argument for path to SCIP index") }` (`cmd/scip/lint.go`, `cmd/scip/convert.go`).
- **Panic only for programmer errors / unreachable states**: `panic(fmt.Sprintf("Unknown symbolAttributeKind: %s", str))` in `testutil/test_runner.go`; `log.Fatal` reserved for CLI startup and truly fatal I/O (`cmd/scip/main.go`, `cmd/scip/convert.go`).
- Prefer `, ok` / `, bool` second returns over errors for "no result" cases: `sel, ok := parseRangeSelection(...)`, `pos, ok := occ.SourceRange()`.

## Logging

**Framework:** stdlib split three ways — `log` / `log.Fatal` for fatal CLI errors, `log/slog` for structured warnings, `fmt.Fprintf(cmd.Writer, ...)` / `fmt.Println` for user-facing output. `github.com/fatih/color` for colored pass/fail output in the test runner. No third-party logging framework.

**Patterns:**
- User-facing CLI output goes through the cli command's writer or stdout: `fmt.Fprintf(cmd.Writer, "Successfully converted ...", outputPath)` in `cmd/scip/convert.go`; `fmt.Println("done: " + output)` in `cmd/scip/snapshot.go`.
- Structured warnings use `slog` with key-value attrs (`cmd/scip/convert.go`):
  ```go
  slog.Warn("found multiple documents with identical relative path; ignoring duplicates",
      slog.String("path", doc.RelativePath),
      slog.Int("firstIndex", pos.Index),
      slog.Int("duplicateIndex", i))
  ```
- Test-runner results use colored check/cross markers: `red.Fprintf(output, "✗ %s\n", ...)` / `green.Fprintf(output, "✓ %s (%d assertions)\n", ...)` in `testutil/test_runner.go`; tests force `os.Setenv("NO_COLOR", "1")` with `t.Cleanup` to unpollute golden output (`reprolang/repro/test_cmd_test.go`).
- Errors returned from `cli.Command.Action` are printed by `main` via `log.Fatal(err)` — prefer returning errors over printing-and-continuing.

## Comments

**When to Comment:**
- Every exported identifier gets a godoc comment starting with the identifier name (`bindings/go/scip/symbol.go`):
  ```go
  // ParseSymbol parses an SCIP string into the Symbol message.
  //
  // Prefer using ParseSymbolUTF8 for strings already known to
  // be valid UTF-8 encoded strings. ...
  ```
- Add a `CAUTION:` line for sharp edges: `// CAUTION: Does not perform full validation of the symbol string's contents.`
- Explain *why*, not what, for non-obvious passes: `// This has to run in a second pass so that all symbols are first populated.` (`cmd/scip/lint.go`).
- Document parsing DSLs with a grammar block comment (see `parseSymbolInfo` in `cmd/scip/lint_test.go`).
- Use `TODO:` comments for known debt: `// TODO: Replace with cmp.Compare go 1.21 or newer.` (`bindings/go/scip/position_test.go`).
- Mark file/directory-level policy with sentinel comments/files: `bindings/go/scip/memtest/DO_NOT_ADD_NEW_TEST_FILES_HERE`.

**Doc comments:**
- Multi-paragraph godoc with a blank comment line between paragraphs; reference related APIs by name (`ParseSymbol` ↔ `ParseSymbolUTF8` ↔ `ParseSymbolUTF8With`).
- Struct fields get line comments when non-obvious: `// path may be empty for external symbols` (`cmd/scip/lint.go`), `// IncludeDescriptors indicates whether ...` (`bindings/go/scip/symbol.go`).

## Function Design

**Size:** Keep functions small and decompose parsing pipelines into named helpers (`parseRangeSelection` → `parseTestCase` → `parseMultilineSuffix` in `testutil/test_runner.go`). Long switch-heavy bodies are acceptable when flat (e.g. `RunTests`).

**Parameters:** Prefer a small number of params; bundle configuration into options/flags structs (`ParseSymbolOptions`, `testFlags`, `snapshotFlags`). Group related scalars into a struct rather than passing 5+ positional args (exception: `RunTests(directory, fileFilters, checkDocuments, index, commentSyntax, output)`).

**Return Values:** `(result, error)` everywhere; `(value, bool)` for lookups (`scipOccurrenceKey`, `parseTestCase`); return the concrete `*scip.Index` / `[]*scip.SourceFile` rather than interfaces. Pure cores return domain values (`lintMainPure(scipIndex *scip.Index) errorSet`) so tests skip the filesystem.

## Module Design

**Exports:**
- The library package `scip` (`bindings/go/scip`) exposes protobuf types plus helpers as free functions on the package (`scip.ParseSymbol`, `scip.NewRangeUnchecked`); avoid methods on generated types except thin `Source*` accessors.
- `testutil` (`bindings/go/scip/testutil`) exports the reusable test harnesses: `RunTests`, `SnapshotTest`, `SnapshotTestDirectories`, `FormatSnapshots` — intended for external indexer authors too.
- CLI package `main` (`cmd/scip`) exports nothing; command list assembled in `commands()` (`cmd/scip/main.go`).

**Barrel Files:**
- Not used (idiomatic Go — import the specific package).

**Generated code:**
- All bindings under `bindings/{typescript,rust,haskell,java,kotlin}` and `bindings/go/scip/scip.pb.go` are generated from `scip.proto` via `buf generate` (`buf.gen.yaml`); regenerate rather than edit. Hand-written Go additions go in sibling files (`symbol.go`, `sort.go`, ...), never in `scip.pb.go`.

---

*Convention analysis: 2026-08-16*
