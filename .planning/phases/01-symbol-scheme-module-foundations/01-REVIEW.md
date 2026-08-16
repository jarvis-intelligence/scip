---
phase: 01-symbol-scheme-module-foundations
reviewed: 2026-08-16T10:56:20Z
depth: deep
files_reviewed: 26
files_reviewed_list:
  - .github/workflows/ci.yaml
  - bindings/go/scip/symbol_fuzz_test.go
  - bindings/go/scip/symbol_parser.go
  - bindings/go/scip/symbol_test.go
  - checks.nix
  - go.work
  - swift/README.md
  - swift/cmd/scip-swift/main.go
  - swift/go.mod
  - swift/internal/symbol/namer.go
  - swift/internal/symbol/namer_test.go
  - swift/internal/symbol/roundtrip_test.go
  - swift/internal/symbol/scheme.go
  - swift/spikes/lsp_probe/main.go
  - swift/spikes/perf/generate/main.go
  - swift/spikes/fixture/Package.swift
  - swift/spikes/fixture/Sources/CapabilityFixture/Core.swift
  - swift/spikes/fixture/Sources/CapabilityFixture/Extensions.swift
  - swift/spikes/fixture/Sources/CapabilityFixture/Generics.swift
  - swift/spikes/fixture/Sources/CapabilityFixture/Unicode.swift
  - bindings/go/scip/go.mod
  - bindings/go/scip/go.sum
  - go.work.sum
  - swift/go.sum
  - swift/spikes/fixture/Package.swift (context)
  - flake.nix (context, wiring verification)
findings:
  critical: 2
  warning: 4
  info: 6
  total: 12
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-08-16T10:56:20Z
**Depth:** deep
**Files Reviewed:** 26
**Status:** issues_found

## Summary

Deep review of the Phase 1 diff (base `1523fe4`): the `peekNext` parser fix plus
regression/fuzz coverage, the new `swift/` module (frozen scheme + namer +
golden/rapid tests), the SourceKit-LSP spike driver and perf generator, and the
CI/Nix wiring.

Verified locally (read-only commands): `GOWORK=off go build ./...` and
`GOWORK=off go test -tags asserts -count=1` pass for both `bindings/go/scip` and
`swift`; workspace-mode `go build ./...` from the repo root passes; `go vet` is
clean. The `peekNext` guard fix is correct: for well-formed UTF-8 the next-rune
start is only dereferenced when strictly inside the string, and the public
`ParseSymbol` entry validates UTF-8 via `beaut.NewUTF8String` before the parser
runs, so the panic class is closed at the public API. The namer's output is
round-trip-validated against `ParseSymbol` before return, and the golden table,
README, and `scheme.go` doc are mutually consistent (kind-number comments match
`scip.proto` enum values).

Two critical findings: (1) the new Nix `swift` check lists package `.` as a
build target, but the `swift/` module root has no Go files, so
`go build .`/`go test .` fail there — the check cannot pass on any Nix host
(verified: `GOWORK=off go build .` in `swift/` exits 1 with "no Go files"); the
mirrored `go-bindings` attribute works only because that module root *does*
have Go files. (2) `lsp_probe`'s `fail()` calls `os.Exit(1)`, skipping every
deferred cleanup — on any post-start failure (readiness-gate timeout, harvest
error, references-pass server death) the spawned `sourcekit-lsp` process is
orphaned and the buffered tail of the JSONL evidence file (including the
diagnostic record written immediately before the failure) is never flushed.

Warnings cover a `LocalSymbol` ordinal/sanitization collision hazard frozen into
the scheme, an unvalidated `-mode` flag that silently runs the perf path, a
silent wrong-position probe in `findUse`, and ignored write errors on the
evidence file. Info items are dead code, missing upfront container-name
validation, negative-index handling, duplicated identifier-charset logic across
modules, the pre-existing `ParseSymbolUTF8` invalid-UTF-8 panic surface, and a
redundant directory-skip condition.

## Critical Issues

### CR-01: Nix `swift` check builds package `.` which has no Go files — check fails on any Nix host

**File:** `checks.nix:146-149`
**Issue:** The new `swift` buildGoModule attribute sets
`subPackages = [ "." "internal/symbol" ]` with `modRoot = "./swift"`. The `swift/`
module root contains only `go.mod`/`go.sum` (all Go code lives under `cmd/`,
`internal/`, `spikes/`), so `go build .` / `go test .` from the module root fails
with `no Go files in .../swift`. Verified on this host: `cd swift && GOWORK=off
go build .` exits 1 with exactly that error (while `go build ./...` passes
because the pattern skips package-less directories). The shape was mirrored from
the `go-bindings` attribute, where `.` works because `bindings/go/scip` root has
~20 non-test Go files. Consequences: `nix build .#checks.x86_64-linux.swift`
(and `nix flake check`, which the new `swift/README.md` advertises) fails; the
phase acceptance criterion was never executed on a Nix host ("nix is not
installed on the executing host", 01-01-SUMMARY), so this was not caught. Note
the CI `fix` job's `nix-update` pass can auto-correct a stale `vendorHash` but
cannot repair a bad `subPackages` list — every CI run on this tree fails.
**Fix:**
```nix
    subPackages = [
      "cmd/scip-swift"
      "internal/symbol"
    ];
```
(Optionally add `"spikes/lsp_probe"` and `"spikes/perf/generate"` if the check
should also compile the spike drivers; all are in `go.sum`/vendor scope already.)

### CR-02: `lsp_probe` failure paths orphan the LSP server and lose buffered evidence (`os.Exit` skips all defers)

**File:** `swift/spikes/lsp_probe/main.go:974-977` (fail), call sites at `768-779`, `837`, `857`, `864`, `866`, `890`
**Issue:** `fail(err)` calls `os.Exit(1)`. Go defers do not run on `os.Exit`, so
after `d.start()` succeeds, every failure path — `initialize` error, readiness
gate timeout (a 30-minute budget, after which `readinessGate` writes a
`gate`-error evidence record and then calls `fail`), any `didOpen`/
`documentSymbol` error, or `referencesPass` server death — (a) leaks the spawned
`sourcekit-lsp` child (the `defer` at `main.go:826-835` that closes the conn,
waits, and kills the process never runs), and (b) discards everything still in
the `bufio.Writer` (the `defer` at `main.go:798-801` never runs). The bufio
buffer is 4096 bytes, so a run that fails before ~4 KiB of records accumulate —
including the very first `meta` record, and always the failure-diagnostic record
written moments before `fail()` — produces an empty or tail-truncated evidence
file. For a driver whose sole deliverable is the JSONL evidence (multi-hour perf
runs), this is silent data loss plus a leaked multi-hundred-MB server process.
**Fix:** Restructure so there is a single exit path (defers honored), e.g.:
```go
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "lsp_probe: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// ... everything currently in main, with `fail(err)` -> `return err`
	// and the existing defers (evidence flush, conn close + kill) in place.
}
```
(Keep `flag.Usage`/`os.Exit(2)` for the pure flag-validation path at the top of
`main`, which precedes any resource acquisition.)

## Warnings

### WR-01: `LocalSymbol` sanitization + ordinal concatenation can collide two distinct locals onto one symbol

**File:** `swift/internal/symbol/namer.go:151-161` (and `sanitizeLocalID` at `166-179`)
**Issue:** The id is `sanitizeLocalID(sourceName)` plus `"_N"` when ordinal > 0.
Because `_` is itself in the simple-identifier set, two different declarations in
the same document can produce the identical local symbol:
`LocalSymbol("count_2", 0)` and `LocalSymbol("count", 2)` both render
`local count_2`; likewise `LocalSymbol("🚀", 0)` and `LocalSymbol("π", 0)` both
render `local _` (golden test row `{"🚀", 0, "local _"}` confirms the collapse).
Since locals are document-scoped, a collision merges two variables' references
— the exact navigation-trust failure the scheme's frozen rules exist to prevent.
Nothing in `namer.go` or `scheme.go` constrains how callers must assign ordinals
(per raw source name vs per sanitized id), so Phase 2 can easily instantiate the
collision. This is the moment the rule gets frozen; it should be made
collision-proof or explicitly specified now.
**Fix:** Either (a) make the sanitized id canonical and the ordinal mandatory,
with a separator excluded from the sanitized charset, e.g. sanitize to
`[A-Za-z0-9+-]` (dropping `_`) and always append `_N`:
`LocalSymbol("count", 2) -> "local count_2"` vs `LocalSymbol("count_2", 0) ->
"local count+2"`-style distinct spellings; or at minimum (b) document in
`scheme.go`'s frozen rules that ordinal assignment MUST be grouped by sanitized
id, not raw source name, and add a golden-table row locking the disambiguation
contract.

### WR-02: Unvalidated `-mode` flag silently runs the perf path with capability settings

**File:** `swift/spikes/lsp_probe/main.go:781-792` and `870`
**Issue:** `-mode` accepts any string. A typo (`-mode=capabilitiy`) skips the
`capability` branch at line 870 and falls into the `else` (perf/references-pass)
branch, while `trim` (line 797) and the probe-symbol default (line 784, `Shape`
vs `Gen000Widget`) both key off `== "perf"` — so the run silently does the perf
harvest with capability-style untrimmed payloads and a possibly-wrong probe
symbol. Evidence harvested this way is mislabeled/undiffable.
**Fix:** Validate at startup:
```go
if *flagMode != "capability" && *flagMode != "perf" {
	fmt.Fprintf(os.Stderr, "lsp_probe: -mode must be 'capability' or 'perf', got %q\n", *flagMode)
	flag.Usage()
	os.Exit(2)
}
```

### WR-03: `findUse` ignores a missing identifier inside the snippet and probes `byteOff-1`

**File:** `swift/spikes/lsp_probe/main.go:329-341`
**Issue:** `ii := strings.Index(snippet, ident)` is unchecked; if `ident` is not
a substring of `snippet`, `ii == -1` and `byteOff = si + ii` points one byte
*before* the snippet — the driver then sends `prepareCallHierarchy`/
`prepareTypeHierarchy` at an arbitrary wrong position and records the resulting
(empty or wrong) response as evidence, rather than reporting the misconfiguration.
(It is also recomputed inside the per-line loop although constant.) Current call
sites pair correct strings, but this driver exists to record trustworthy
evidence about hierarchy support.
**Fix:**
```go
func findUse(text, snippet, ident string) (lspPosition, bool) {
	ii := strings.Index(snippet, ident)
	if ii < 0 {
		return lspPosition{}, false
	}
	...
}
```

### WR-04: Evidence-file write errors are silently dropped

**File:** `swift/spikes/lsp_probe/main.go:94-95` and `798-801`
**Issue:** `e.w.Write(b)` / `e.w.WriteByte('\n')` return values are discarded,
and the deferred `ev.w.Flush()` error is likewise ignored. `bufio.Writer` is
sticky-error: after the first IO failure (full disk, closed file), every
subsequent record is silently dropped and the process still exits 0 with a
truncated JSONL file — the spike's conclusions would be drawn from partial data
with no signal.
**Fix:** Track a sticky error on the `evidence` struct (`e.err`), check it after
`Flush` in the shutdown path, and fail loudly:
```go
defer func() {
	if err := ev.w.Flush(); err != nil { /* report */ }
	...
}()
```

## Info

### IN-01: Dead code in lsp_probe (unused fields and return value)

**File:** `swift/spikes/lsp_probe/main.go:73` (`evidence.f` never read), `:217` (`stderrRing.tail` never read), `:663` (`harvestFiles` always returns `0` as its second result, discarded by the caller at `:864`)
**Issue:** Three pieces of dead code accumulated during spike evolution.
**Fix:** Delete `evidence.f`, delete `stderrRing.tail` (or use it instead of the
growing `buf`), and change `harvestFiles` to return `([]defInfo, error)`.

### IN-02: Empty `Container` names are not validated upfront

**File:** `swift/internal/symbol/namer.go:62-68` and `:96-110`
**Issue:** `Symbol` rejects empty `Module`/`Name` with dedicated sentinel errors,
but an empty `ContainerPath[i].Name` flows into `newDescriptor` and only surfaces
much later as the generic `namer produced unparseable symbol %q: ...` (the output
validation at `:86` catches it, so no malformed string escapes — diagnostics are
just opaque and the failure site is far from the cause).
**Fix:** Validate container names in the same block as the `ErrEmptyModule` /
`ErrEmptyName` checks (e.g. `ErrEmptyContainerName`), or fold into a single
input-validation loop over `in.ContainerPath`.

### IN-03: Negative `OverloadIndex` / `ordinal` are silently coerced to "no disambiguator"

**File:** `swift/internal/symbol/namer.go:127` (`if overloadIndex > 0`) and `:153` (`if ordinal > 0`)
**Issue:** A caller passing `-1` (e.g. an off-by-one in Phase 2's overload
grouping) silently renders index-0 spelling, under-disambiguating genuine
overloads — a determinism bug in the making. The zero-vs-positive contract is
documented; negative is neither documented nor rejected.
**Fix:** Reject negatives (`return "", fmt.Errorf(...)`) or document that
callers must pass `>= 0`.

### IN-04: Identifier-charset logic duplicated between the bindings and the swift module

**File:** `swift/internal/symbol/namer.go:184-187` vs `bindings/go/scip/identifier.go:3-5`
**Issue:** `isSimpleIdentifierCharacter` is re-implemented privately in the
swift module (currently byte-identical to the bindings version). If either copy
drifts, escaping (formatter, bindings copy) and local-id sanitization (namer
copy) disagree — exactly the class of divergence the round-trip properties are
meant to guard, but the rapid tests would only catch it after the fact.
**Fix:** Acceptable to keep (unexported, stable grammar), but add a
cross-referencing comment on both copies ("must stay in sync with
bindings/go/scip/identifier.go") and a golden row pinning a boundary character
set — or export the predicate from the bindings module and reuse it.

### IN-05: Pre-existing: `ParseSymbolUTF8*` can still panic on invalid UTF-8 (documented precondition)

**File:** `bindings/go/scip/symbol_parser.go:281-296` (`findRuneAtIndex` reads up to 4 bytes from a lead byte)
**Issue:** Not introduced by this phase: `ParseSymbol` validates via
`beaut.NewUTF8String` (verified: `utf8.ValidString` check), so the fixed guard
closes the panic for the public path; but `ParseSymbolUTF8`/`ParseSymbolUTF8With`
callers feeding malformed UTF-8 (e.g. a truncated trailing rune like `"a\xc3"`)
still index out of range in `findRuneAtIndex`. The precondition is documented on
the function; the fuzz doc's "never panics" claim holds only for the
`ParseSymbol` entry.
**Fix (optional hardening):** clamp reads in `findRuneAtIndex` (return
`utf8.RuneError, 1` when continuation bytes fall outside the string), or note
the precondition on `ParseSymbolUTF8`'s doc comment more prominently.

### IN-06: Redundant/precedence-heavy skip condition in `swiftSources`

**File:** `swift/spikes/lsp_probe/main.go:735`
**Issue:** `info.Name() == ".build" || strings.HasPrefix(info.Name(), ".") && info.Name() != "."`
— the `.build` disjunct is subsumed by the dot-prefix conjunct (`.build` starts
with `.`), and the `||`/`&&` mix reads as if `.build` were special-cased
differently than it is.
**Fix:** Simplify to `strings.HasPrefix(info.Name(), ".") && info.Name() != "."`.

---

_Reviewed: 2026-08-16T10:56:20Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: deep_
