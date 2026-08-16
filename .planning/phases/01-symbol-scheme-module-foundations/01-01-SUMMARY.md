---
phase: 01-symbol-scheme-module-foundations
plan: 01
subsystem: symbol-scheme
tags: [go, scip, swift, symbol-formatting, fuzzing, nix, ci]

# Dependency graph
requires:
  - phase: 00-brownfield-init
    provides: SCIP Go bindings (ParseSymbol/VerboseSymbolFormatter), reprolang module precedent, Nix/CI check matrix
provides:
  - Go module github.com/scip-code/scip/swift wired into go.work / ci.yaml / checks.nix
  - Tracer namer API (Scheme, ManagerSwiftPM, SymbolInput, Container, DeclKind, Symbol(SymbolInput) (string, error)) for plans 01-02/01-03 to expand
  - scip.ParseSymbol multi-byte-trailing-rune panic fixed (returns unrecognizedDescriptorError)
  - FuzzParseSymbol fuzz target + TestParseSymbolRuneSoupNeverPanics rapid property in bindings/go/scip
  - checks.x86_64-linux.swift buildGoModule attribute with computed vendorHash
affects: [01-02-namer-spec, 01-03-sourcekit-lsp-spikes, phase-2-indexer]

actuals:
  tokens: 5155
  tasks: 3
  commits: 6

tech-stack:
  added: []
  patterns:
    - "Build *scip.Symbol protobuf values and format via scip.VerboseSymbolFormatter — never Sprintf descriptor suffixes (namer validate-before-return with ParseSymbol)"
    - "RED/GREEN gate commits per behavior-adding task (MVP+TDD)"
    - "One buildGoModule check attribute per Go module with GOWORK=off and relative replace"

key-files:
  created:
    - swift/go.mod
    - swift/go.sum
    - swift/cmd/scip-swift/main.go
    - swift/internal/symbol/namer.go
    - swift/internal/symbol/namer_test.go
    - swift/README.md
    - bindings/go/scip/symbol_fuzz_test.go
  modified:
    - go.work
    - go.work.sum
    - bindings/go/scip/symbol_parser.go
    - bindings/go/scip/symbol_test.go
    - .github/workflows/ci.yaml
    - checks.nix

key-decisions:
  - "Tracer namer_test.go carries a small rapid round-trip property so pgregory.net/rapid stays imported — go mod tidy drops unused requires and the plan mandates rapid pinned now for zero 01-02 go.mod churn"
  - "go.work entry normalized to 'swift' (go 1.26.5's 'go work use' writes './swift'; repo precedent and the plan's expected form use 'swift')"
  - "checks.nix vendorHash computed as NAR-SHA256 of the 'GOWORK=off go mod vendor' tree via a pipeline first validated against the known go-bindings hash (exact match) because nix is not installed on the executing host"
  - "Fuzz soak counterexample (empty-field round-trip asymmetry) treated as a pre-existing out-of-scope defect: documented in the fuzz doc comment + deferred-items.md, discovered corpus entry NOT committed so plain go test and -tags asserts stay green"
  - "Reminder: open an upstream scip-code/scip issue with the three repro inputs + the one-line peekNext fix after the phase lands (fix is upstream-worthy; do not block on it)"

patterns-established:
  - "Namer validate-before-use: every Symbol() output is ParseSymbol-checked and wrapped as fmt.Errorf(\"namer produced unparseable symbol %q: %w\", s, err)"
  - "New-module wiring triple in the same change: ci.yaml paths-filter + tidy loop + nix-update attr alongside the module manifest"

requirements-completed: [SYM-01]

coverage:
  - id: D1
    description: "swift/ Go module with tracer namer producing and identity-round-tripping 'scip-swift swiftpm MyApp . Shape#area().' through ParseSymbol/VerboseSymbolFormatter"
    requirement: SYM-01
    verification:
      - kind: unit
        ref: swift/internal/symbol/namer_test.go#TestSymbolMethodOnStructRoundTrips
        status: pass
      - kind: unit
        ref: swift/internal/symbol/namer_test.go#TestSymbolRoundTripProperty
        status: pass
      - kind: unit
        ref: command:go test ./swift/... (repo root, workspace mode) and GOWORK=off go test ./... (swift/)
        status: pass
    human_judgment: false
  - id: D2
    description: "ParseSymbol returns errors instead of panicking for trailing multi-byte runes; FuzzParseSymbol seed corpus and rapid rune-soup property green; all pre-existing bindings tests (incl. -tags asserts and ./memtest) stay green"
    requirement: SYM-01
    verification:
      - kind: unit
        ref: bindings/go/scip/symbol_test.go#TestParseSymbolError
        status: pass
      - kind: unit
        ref: bindings/go/scip/symbol_fuzz_test.go#FuzzParseSymbol
        status: pass
      - kind: unit
        ref: bindings/go/scip/symbol_fuzz_test.go#TestParseSymbolRuneSoupNeverPanics
        status: pass
      - kind: integration
        ref: command:cd bindings/go/scip && go test ./... && go test -tags asserts ./...
        status: pass
    human_judgment: false
  - id: D3
    description: "Repo wiring: ci.yaml paths-filter/tidy-loop/nix-update entries, checks.nix swift buildGoModule attribute, prettier-clean swift/README.md, flake.nix untouched"
    requirement: SYM-01
    verification:
      - kind: other
        ref: command:grep checks for swift/go.mod, tidy loop line, checks.x86_64-linux.swift, modRoot (all PASS)
        status: pass
    human_judgment: true
    rationale: "nix is not installed on the executing host, so 'nix eval .#checks.x86_64-linux', 'nix build .#checks.x86_64-linux.swift', 'nix flake check', nixfmt, and prettier --check were NOT run. The vendorHash was derived via a NAR-hash pipeline validated against the known go-bindings hash, but the actual nix build must be confirmed on a nix-enabled host (CI)."

# Metrics
duration: 17min
completed: 2026-08-16
status: complete
---

# Phase 1 Plan 01: Symbol Scheme & Module Foundations — Walking Skeleton Summary

**New `github.com/scip-code/scip/swift` Go module whose tracer namer round-trips `scip-swift swiftpm MyApp . Shape#area().` through ParseSymbol/VerboseSymbolFormatter, the pre-existing ParseSymbol trailing-multi-byte-rune panic fixed to return errors with fuzz + rapid regression coverage, and CI/Nix wiring for the module.**

## Performance

- **Duration:** 17 min
- **Started:** 2026-08-16T09:03:30Z
- **Completed:** 2026-08-16T09:20:56Z
- **Tasks:** 3
- **Files modified:** 13

## Accomplishments

- Standing up `swift/` as the fourth workspace module: manifest mirrors reprolang (go 1.25.0, relative replace to `../bindings/go/scip`), go.sum generated by `GOWORK=off go mod tidy`, builds and tests green from both repo root (workspace mode) and `swift/` (GOWORK=off)
- Walking-skeleton slice proven: `Symbol(SymbolInput{Module: "MyApp", Container: Shape/struct, Name: "area", Kind: method})` returns exactly `scip-swift swiftpm MyApp . Shape#area().`, which parses and re-formats as the identity — module wiring + symbol-identity pipeline verified end-to-end on day one
- Fixed the phase-blocking `scip.ParseSymbol` out-of-bounds panic: `peekNext`'s guard now tests `byteIndex+int(bytesToNextRune)` (the offset actually read); trailing multi-byte runes return `unrecognizedDescriptorError`, never a panic and never a partial parse
- Added `FuzzParseSymbol` (stdlib `testing.F`, 27 seeds) and `TestParseSymbolRuneSoupNeverPanics` (rapid rune-soup generator biased to multi-byte tails); 60s local soak explored at ~5.4k execs/sec
- Wired the module into the CI fix job (paths-filter, tidy loop, nix-update attr) and added the `checks.x86_64-linux.swift` buildGoModule attribute with a computed-and-validated vendorHash; no new CI job (list/checks matrix auto-discovers); flake.nix untouched

## Task Commits

Each task was committed atomically (TDD tasks carry RED before GREEN):

1. **Task 1 RED: failing tracer test + module manifest** - `6e460c0` (test)
2. **Task 1 GREEN: tracer namer + cmd stub** - `9652b2f` (feat)
3. **Task 1 WIRE: go work use ./swift** - `1f9c3ac` (chore)
4. **Task 2 RED: regression inputs + fuzz corpus, panic reproduced** - `9bae1ec` (test)
5. **Task 2 GREEN: peekNext guard fix** - `566fcf9` (feat)
6. **Task 3: ci.yaml + checks.nix + README wiring** - `4d08068` (chore)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified

- `swift/go.mod`, `swift/go.sum` — module manifest (github.com/scip-code/scip/swift) + lockfile
- `swift/internal/symbol/namer.go` — tracer namer: consts, SymbolInput/Container/DeclKind, `Symbol(SymbolInput) (string, error)` building `*scip.Symbol` and validating via ParseSymbol
- `swift/internal/symbol/namer_test.go` — golden + identity round-trip test, rapid round-trip property
- `swift/cmd/scip-swift/main.go` — stub main (CLI surface is Phase 2+)
- `swift/README.md` — module docs, Running tests, runtime-only prerequisites
- `bindings/go/scip/symbol_parser.go` — exactly one guard line changed (peekNext)
- `bindings/go/scip/symbol_test.go` — three panic-repro inputs appended to TestParseSymbolError
- `bindings/go/scip/symbol_fuzz_test.go` — FuzzParseSymbol + TestParseSymbolRuneSoupNeverPanics
- `go.work`, `go.work.sum` — swift added to the use block (committed together)
- `.github/workflows/ci.yaml` — paths-filter + tidy loop + nix-update attr
- `checks.nix` — `swift` buildGoModule attribute

## Decisions Made

See key-decisions in frontmatter. Highlights: the rapid import kept alive by a real property test (tidy drops unused deps); vendorHash derived via a NAR-SHA256 pipeline validated against the known go-bindings hash because nix is absent on this host; the fuzz-soak counterexample recorded as a pre-existing defect rather than silently weakening the property.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added a rapid round-trip property to namer_test.go**
- **Found during:** Task 1 (RED)
- **Issue:** The plan requires `pgregory.net/rapid v1.3.0` in swift/go.mod now ("so plan 01-02 adds no go.mod churn") AND requires go.sum be produced by `go mod tidy` — but tidy drops requires nothing imports, and the plan's tracer tests import only testify + scip
- **Fix:** Added `TestSymbolRoundTripProperty` (small rapid generator set) to namer_test.go so rapid is genuinely imported; tidy then retains it and the CI tidy loop stays drift-free
- **Files modified:** swift/internal/symbol/namer_test.go
- **Verification:** go.mod contains the exact required rapid string; `GOWORK=off go mod tidy` is a no-op afterwards
- **Committed in:** 6e460c0, 9652b2f

**2. [Rule 1 - Bug] Two test-compile corrections during GREEN**
- **Found during:** Task 1 (GREEN)
- **Issue:** (a) RESEARCH's example used `rapid.SampledFrom("a", "b", "c")` but v1.3.0's signature is `SampledFrom[S ~[]E, E any](slice S)`; (b) `scip.Scheme` type does not exist in the bindings — `Symbol.Scheme` is a plain `string`
- **Fix:** Slice-argument SampledFrom calls; plain string equality for the scheme assertion
- **Files modified:** swift/internal/symbol/namer_test.go
- **Verification:** tests compile and pass
- **Committed in:** 6e460c0, 9652b2f

**3. [Rule 3 - Blocking] Nix gates unrunnable on host — vendorHash derived and cross-validated instead**
- **Found during:** Task 3
- **Issue:** nix is not installed on the executing host, so the acceptance criteria `nix build .#checks.x86_64-linux.swift` (vendorHash source), `nix eval`, `nix flake check`, `nixfmt`, and `prettier --check` could not run
- **Fix:** Computed the vendorHash as the NAR-SHA256 of the `GOWORK=off go mod vendor` tree using a purpose-built hasher FIRST validated against the known go-bindings vendorHash (exact match: `sha256-9uPDs/MoITP8Q92HQEMoRUB7Dz30n242z4cSnxjiPCk=`); mirrored the proven go-bindings attribute shape for nixfmt compliance; wrote the README to `.prettierrc` rules; validated ci.yaml as YAML. Recorded as unrun-verify in `.planning/WINDOWS.md` and deferred-items.md #2
- **Verification:** hash pipeline reproduces the known-good value bit-for-bit; go-level gates all green
- **Committed in:** 4d08068

**4. [Rule 1 - Bug] Pre-existing round-trip asymmetry found by fuzz soak (out of scope — documented, not fixed)**
- **Found during:** Task 2 (60s soak)
- **Issue:** `" 0 0 0 0!"` (empty field spelled literally rather than as `.`) parses but re-formats as `". 0 0 0 0!"` — `writeEscapedPackage` maps `""` → `"."`. Pre-existing and independent of the guard fix (pure-ASCII input; guards identical for single-byte runes)
- **Fix:** Documented in the FuzzParseSymbol doc comment and deferred-items.md #1; the discovered corpus entry was NOT committed so plain `go test` / `-tags asserts` stay green; WINDOWS.md deviation entry recorded
- **Files modified:** bindings/go/scip/symbol_fuzz_test.go (doc comment only)
- **Verification:** seed corpus green under plain and asserts builds
- **Committed in:** 566fcf9

**5. [Rule 3 - Blocking] go.work path normalization**
- **Found during:** Task 1 (WIRE)
- **Issue:** local go 1.26.5's `go work use ./swift` records `./swift` in the use block; the plan's expected form and repo precedent use `swift`
- **Fix:** normalized the use-block entry to `swift`
- **Verification:** workspace build/test green from root
- **Committed in:** 1f9c3ac

---

**Total deviations:** 5 auto-fixed (2 blocking, 2 bug-class, 1 out-of-scope discovery documented per scope boundary)
**Impact on plan:** All fixes necessary to satisfy the plan's own acceptance criteria or to keep CI green. No scope creep; the one discovered pre-existing defect was logged, not fixed.

## TDD Gate Compliance

Both behavior-adding tasks (`tdd="true"`) carry RED-before-GREEN gate commits:

| Task | RED commit | RED state evidence | GREEN commit |
|------|-----------|--------------------|--------------|
| Task 1 (tracer) | `6e460c0` test(01-01) | `go test ./...` failed to compile: `undefined: Symbol, SymbolInput, Container, DeclKind*` | `9652b2f` feat(01-01) |
| Task 2 (panic fix) | `9bae1ec` test(01-01) | TestParseSymbolError FAIL: panic `index out of range [13] with length 13` on "a b c d fooΩ"; rapid FAIL on "Ω"; fuzz seed#20 FAIL | `566fcf9` feat(01-01) |

No `feat(01-01)` commit precedes its `test(01-01)` counterpart. Task 3 is config/docs wiring (no `tdd="true"`), exempt.

## Issues Encountered

- `go mod tidy` on an empty module strips the require block — resolved by writing the importing test file before tidying (the RED order the plan prescribes anyway)
- `go test ./...` from the workspace root matches only the root module's packages under go 1.26.5; full coverage required explicit `./bindings/go/scip/...`, `./reprolang/...`, `./swift/...` patterns (all green, plus per-module GOWORK=off runs)
- Local-only 60s fuzz soak outcome: corpus explores (11 workers, ~5.4k execs/sec, 5 new interesting inputs) and found the pre-existing empty-field asymmetry above — the soak's stated purpose (confirm exploration, record outcome) was met

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `swift/internal/symbol` is the home for plan 01-02's full scheme (scheme.go constants, mapping-table golden tests, roundtrip_test.go) — SymbolInput already carries the unread fields (IsSystemModule, SwiftToolchainVersion) so the struct shape does not change
- The ParseSymbol blocker from STATE.md is cleared: mass symbol emission and Unicode Swift names are unblocked repo-wide (`scip lint`/`scip snapshot` call ParseSymbol per occurrence)
- Open follow-ups (tracked in deferred-items.md + WINDOWS.md): first CI run on a nix-enabled host must confirm `nix flake check` green; upstream scip-code/scip issue with the repro + one-line fix; empty-field round-trip asymmetry needs its own TDD task if canonical strictness is wanted

## Self-Check: PASSED

- Created files exist on disk: swift/go.mod, swift/go.sum, swift/cmd/scip-swift/main.go, swift/internal/symbol/namer.go, swift/internal/symbol/namer_test.go, swift/README.md, bindings/go/scip/symbol_fuzz_test.go — all FOUND
- Commits exist on branch gsd/v1.0-milestone: 6e460c0, 9652b2f, 1f9c3ac, 9bae1ec, 566fcf9, 4d08068 — all FOUND
- TDD gate sequence in git log: test(01-01) → feat(01-01) × 2 — VERIFIED

---
*Phase: 01-symbol-scheme-module-foundations*
*Completed: 2026-08-16*
