---
phase: 01-symbol-scheme-module-foundations
plan: 02
subsystem: symbol-scheme
tags: [go, scip, swift, symbol-formatting, property-testing, tdd, rapid]

# Dependency graph
requires:
  - phase: 01-symbol-scheme-module-foundations/01-01
    provides: swift/ Go module with tracer namer (SymbolInput/Container/DeclKind/Symbol), fixed scip.ParseSymbol multi-byte panic, rapid/testify pinned in swift/go.mod
provides:
  - Frozen scip-swift symbol scheme as executable spec — golden table asserting every RESEARCH mapping row 1-29 (26 rows where a string is defined + tracer row + empty-version system module)
  - Full shared namer API — Symbol(SymbolInput) (string, error) over the complete SymbolInput space (system modules, all 22 DeclKind families, overload disambiguators, empty-input error sentinels) and LocalSymbol(sourceName, ordinal) with simple-identifier sanitization
  - scheme.go — consts Scheme/ManagerSwiftPM/ManagerSystem, DeclKind (22 values), ScipKind() frozen mapping to scip.SymbolInformation.Kind, normative package doc (grammar, determinism, canonical #if policy, dual-path identity, SYM-02 extension attribution, Term-family limitation, no-symbol rows 30-31)
  - rapid round-trip proofs — TestNamerRoundTrip (emoji/operators/backtick/multi-byte module names), TestLocalSymbolRoundTrip, TestNamerPure (incl. parallel determinism)
  - swift/README.md "Symbol scheme" section restating the frozen rules
affects: [01-03-sourcekit-lsp-spikes, phase-2-indexer, phase-3-semantic-path, phase-4-relationships]

actuals:
  tokens: 9525 # chars/4 over the realized swift/ diff (estimate was 45000, confidence: low)
  tasks: 3
  commits: 3

tech-stack:
  added: [] # go-cmp v0.7.0 promoted from transitive to direct require; already in go.sum via the bindings module
  patterns:
    - "Golden table keyed t.Run(test.Expected, ...) — one row per mapping-table row; duplicate strings (getter vs zero-arg method) are legal and distinguished by Kind"
    - "Descriptor suffixes chosen in exactly one switch (newDescriptor/appendDescriptors); OverloadIndex only ever lands on Method-family descriptors"
    - "Local ids: sanitize content (non-simple runes -> _) then build the local-form *scip.Symbol and let the formatter render 'local <id>'"

key-files:
  created:
    - swift/internal/symbol/scheme.go
    - swift/internal/symbol/roundtrip_test.go
  modified:
    - swift/internal/symbol/namer.go
    - swift/internal/symbol/namer_test.go
    - swift/README.md
    - swift/go.mod

key-decisions:
  - "DeclKind constants keep the DeclKind-prefixed names (DeclKindStruct, DeclKindGetter, ...) established by the 01-01 tracer — the plan lists family names; the prefix preserves the tracer test unchanged and Go naming convention"
  - "Container descriptors use the container's own Kind through the same suffix switch — a Func container renders f(). (row 27 parameter case), not f#, so parameters nest as f().(x)"
  - "Destructor (deinit) maps to SymbolInformation_Method (no dedicated kind) and EnumCase to SymbolInformation_EnumMember, per the RESEARCH mapping table"
  - "IsSystemModule with an empty SwiftToolchainVersion passes Version \"\" straight through — the formatter's \"\"->\".\" placeholder rendering is the deterministic documented behavior (golden row 'scip-swift swift Swift . Swift/')"
  - "LocalSymbol ordinal formatted with fmt.Sprintf(\"%s_%d\") — same content-level class as the \"+N\" disambiguator; no symbol-grammar assembly anywhere"
  - "rapid failfiles (testdata/rapid/*.fail) are transient RED-state repro artifacts — deleted, not committed; they only regenerate when a property fails"

patterns-established:
  - "Error sentinels ErrEmptyModule/ErrEmptyName returned before any descriptor construction — malformed input errors instead of emitting"
  - "Golden-table + property pair: every scheme rule gets one exact-string row AND survives the random input space through ParseSymbol/FormatSymbol identity"

requirements-completed: [SYM-01, SYM-02]

coverage:
  - id: D1
    description: "Golden table executable spec: every RESEARCH mapping row 1-29 asserted as an exact symbol string (module, system module, struct/enum/protocol/typealias, nested type, func, method, overloads 0/1/2, both operator forms, init(+0/+1), deinit, getter, escaped setter, property/let/global-var terms, subscript(+0/+1), enum case, extension member, retroactive owner-module attribution with (+1) collision, protocol method, witness/override paths, type parameter, parameter, macro), plus empty-Module/empty-Name error cases and LocalSymbol sanitization goldens"
    requirement: SYM-01
    verification:
      - kind: unit
        ref: swift/internal/symbol/namer_test.go#TestSymbolGoldenTable
        status: pass
      - kind: unit
        ref: swift/internal/symbol/namer_test.go#TestSymbolInputErrors
        status: pass
      - kind: unit
        ref: swift/internal/symbol/namer_test.go#TestLocalSymbolGolden
        status: pass
    human_judgment: false
  - id: D2
    description: "Round-trip property proofs: rapid-generated inputs (modules incl 🄼odule, containers 0-3 deep, operator/emoji/backtick/multi-byte names, all 22 kinds, overloads 0-5, system-module headers) satisfy Symbol -> ParseSymbol -> FormatSymbol identity with Scheme == scip-swift; locals round-trip as IsLocalSymbol; Symbol is pure under repeated and parallel calls"
    requirement: SYM-01
    verification:
      - kind: unit
        ref: swift/internal/symbol/roundtrip_test.go#TestNamerRoundTrip
        status: pass
      - kind: unit
        ref: swift/internal/symbol/roundtrip_test.go#TestLocalSymbolRoundTrip
        status: pass
      - kind: unit
        ref: swift/internal/symbol/roundtrip_test.go#TestNamerPure
        status: pass
      - kind: integration
        ref: command:cd swift && GOWORK=off go test ./... ; repo root go test ./swift/...
        status: pass
    human_judgment: false
  - id: D3
    description: "scheme.go frozen spec: consts, DeclKind set, ScipKind mapping to scip.SymbolInformation.Kind, and the normative doc (determinism, canonical #if policy, dual-path identity, SYM-02 attribution, Term-family limitation, rows 30-31)"
    requirement: SYM-02
    verification:
      - kind: other
        ref: command:grep ManagerSwiftPM = "swiftpm" / Scheme / ManagerSystem in scheme.go; Sprintf + Descriptor_Package sweeps clean; gofmt clean
        status: pass
    human_judgment: true
    rationale: "ScipKind is frozen data with no behavioral consumer until Phase 2/3; the mapping was verified by inspection against the generated constants in bindings/go/scip/scip.pb.go, not by an executing test (none was mandated by the plan's behavior block)."
  - id: D4
    description: "swift/README.md 'Symbol scheme' section restating header forms, extension attribution, determinism, canonical #if policy, dual-path identity, and the Term-family limitation"
    requirement: SYM-01
    verification:
      - kind: other
        ref: command:grep -cF '## Symbol scheme' swift/README.md == 1
        status: pass
    human_judgment: true
    rationale: "nix develop is unavailable on the executing host so 'nix develop --command prettier --check swift/README.md' was NOT run; the section was hand-written to the .prettierrc rules (no semicolons, single quotes, es5 trailing commas). Recorded as unrun-verify in .planning/WINDOWS.md; first CI run must confirm."

# Metrics
duration: 11min
completed: 2026-08-16
status: complete
---

# Phase 1 Plan 02: Symbol Scheme & Module Foundations — Frozen Scheme + Shared Namer Summary

**Frozen scip-swift symbol scheme as an executable spec: a 26-row golden table over every RESEARCH mapping row, a full namer (system modules, 22 DeclKind families, overload disambiguators, error sentinels, sanitized locals) building every string through \*scip.Symbol + VerboseSymbolFormatter, and rapid round-trip/determinism properties covering emoji, operators and backticked names**

## Performance

- **Duration:** 11 min
- **Started:** 2026-08-16T09:31:33Z
- **Completed:** 2026-08-16T09:42:06Z (execution gates; SUMMARY close-out follows)
- **Tasks:** 3
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments

- Golden table now pins the exact symbol string for every mapping row: modules and system modules (with toolchain version and the empty-version "." placeholder), all type kinds, nested types, funcs/methods, overload disambiguators (+1/+2), both operator spellings (`+` raw because it IS an identifier character, `` `==` `` backtick-escaped by the formatter), init/deinit, getter/setter accessors (`` `area=` `` escaped form included), terms, subscripts, enum cases, extension members, retroactive owner-module attribution with the (+1) method-family collision, protocol requirements, witness/override own paths, type parameters `[T]`, parameters `f().(x)`, macros, plus the 01-01 tracer row
- The namer covers the full SymbolInput space with validate-before-return unchanged: system modules use ManagerSystem + SwiftToolchainVersion; empty Module/Name return the ErrEmptyModule/ErrEmptyName sentinels; overload indices land only on Method-family descriptors (the only suffix the formatter renders disambiguators for)
- LocalSymbol sanitizes any source name (emoji, CJK, backticks, spaces) onto the simple-identifier charset with "\_N" ordinals and renders through the local-form Symbol — Unicode never appears raw in a local id
- Rapid properties prove the encoding edge (parse->format identity over generated inputs including "🄼odule" module names, "back\`\`tick" names, operators), the local-form round-trip, and purity under repeated + 8-way parallel calls
- scheme.go carries the normative frozen doc: grammar summary, determinism rule, canonical #if policy (macOS arm64 / host Swift version / DEBUG; fallback picks the same single branch), dual-path identity rule, SYM-02 extension attribution, the flagged Term-family retroactive-collision limitation, and the rows that emit nothing (property-wrapper storage accessors, self/Self); README restates them

## Task Commits (TDD: RED -> GREEN -> REFACTOR)

Each gate is its own atomic commit:

1. **Task 1 RED — golden table + round-trip properties + type-only scaffolding** - `cf53512` (test)
2. **Task 2 GREEN — frozen scheme.go + full namer implementation** - `abdc3d6` (feat)
3. **Task 3 REFACTOR — descriptor helpers + README scheme section + discipline sweep** - `9a5130d` (refactor)

**Plan metadata:** (docs commit — see below)

## RED / GREEN / REFACTOR detail

- **RED (cf53512):** namer_test.go grew TestSymbolGoldenTable (26 rows), TestSymbolInputErrors, TestLocalSymbolGolden; roundtrip_test.go added genName/genContainer/genDeclKind/genInput + the three properties. To keep the package compiling, scheme.go declared the full DeclKind constant set (constants only) and namer.go gained the LocalSymbol signature returning "". The suite failed behaviorally as required: 24/26 golden rows wrong (unsupported-kind errors from the tracer's default branch), system-module row wrong manager, empty-Module emitted a symbol instead of erroring, locals returned "". The rows that passed (Shape#, Shape#area()., resize overloads, tracer row) are exactly the tracer's struct+method path.
- **GREEN (abdc3d6):** scheme.go gained the consts, the ScipKind switch, and the normative package doc (package comment moved here from namer.go to stay single); namer.go implemented the container/name descriptor switches, package headers per module kind, error sentinels, LocalSymbol + sanitizeLocalID + the local charset mirror. All tests green from swift/ (GOWORK=off) and repo root.
- **REFACTOR (9a5130d):** the duplicated container/name suffix switches collapsed into appendDescriptors + newDescriptor (the single place suffixes are chosen), with the overload-index-only-on-Method rule documented at that site; README gained the Symbol scheme section; sweeps confirmed no Sprintf outside the "+N"/"\_N" ordinals and no Descriptor_Package anywhere under swift/internal/symbol/.

## TDD Gate Compliance

| Task | RED commit | RED state evidence | GREEN commit | REFACTOR commit |
|------|-----------|--------------------|--------------|-----------------|
| Task 1 (golden + properties) | `cf53512` test(01-02) | `GOWORK=off go test ./internal/symbol/...` FAIL: TestSymbolGoldenTable 24 subtests, TestSymbolInputErrors, TestLocalSymbolGolden, TestNamerRoundTrip/TestLocalSymbolRoundTrip/TestNamerPure — all behavioral ("namer cannot map unsupported DeclKind", `local symbol "" must be local`, wrong strings), zero compile errors | `abdc3d6` feat(01-02) | `9a5130d` refactor(01-02) |

Gate sequence in git log: test(01-02) -> feat(01-02) -> refactor(01-02). No feat precedes its test.

## Files Created/Modified

- `swift/internal/symbol/scheme.go` — created: Scheme/ManagerSwiftPM/ManagerSystem consts, DeclKind (22 values) + ScipKind() frozen mapping, normative package doc (rules + limitation + no-symbol rows)
- `swift/internal/symbol/namer.go` — expanded from tracer to full space: system-module packages, appendDescriptors/newDescriptor helpers, ErrEmptyModule/ErrEmptyName, LocalSymbol + sanitizeLocalID
- `swift/internal/symbol/namer_test.go` — golden table + error cases + LocalSymbol goldens; tracer tests retained
- `swift/internal/symbol/roundtrip_test.go` — created: generators biased to grammar-hostile names; TestNamerRoundTrip, TestLocalSymbolRoundTrip, TestNamerPure
- `swift/README.md` — "Symbol scheme" section
- `swift/go.mod` — go-cmp promoted to direct require

## Decisions Made

See key-decisions in frontmatter. Highlights: DeclKind-prefixed constant names (tracer continuity); container kinds flow through the same suffix switch so Func containers render `f().` and parameters nest as `f().(x)`; Destructor->Method and EnumCase->EnumMember per the mapping table; the empty system-module version relies on the formatter's "." placeholder (golden-pinned).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] go-cmp promoted to a direct require in swift/go.mod**
- **Found during:** Task 1 (RED)
- **Issue:** The plan mandates the bindings table-test shape "require + cmp.Diff on strings" (PATTERNS.md item 6) while stating "no go.mod changes" — importing `github.com/google/go-cmp/cmp` makes it a direct dependency
- **Fix:** Imported cmp for the table diff; `GOWORK=off go mod tidy` promoted go-cmp v0.7.0 to the direct require block. It was already in swift/go.sum via the bindings module's require of the same version — zero new dependency resolution, no go.sum churn
- **Files modified:** swift/go.mod
- **Verification:** tidy is a no-op afterwards; suite compiles and runs
- **Committed in:** cf53512

**2. [Rule 1 - Bug] rapid failfiles generated by the failing RED run**
- **Found during:** Task 1 (RED)
- **Issue:** rapid v1.3.0 writes `testdata/rapid/<Test>/<Test>-<ts>.fail` repro files when a property fails, leaving untracked runtime artifacts in the tree
- **Fix:** Deleted after confirming the RED failure output was captured; they do not regenerate once green (and any reappearance signals a failing property)
- **Files modified:** (transient, not committed)
- **Verification:** clean `git status` after every gate commit
- **Committed in:** n/a

**3. [Rule 3 - Blocking] Nix/prettier gate unrunnable on host — hand-conformance instead**
- **Found during:** Task 3
- **Issue:** `nix develop --command prettier --check swift/README.md` cannot run (nix not installed on this host; 01-01 precedent)
- **Fix:** The README section was written to the `.prettierrc` rules (semi false, single quotes, es5 trailing commas, prose wrap matching the existing prettier-clean file); gofmt DID run and is clean; recorded as unrun-verify in `.planning/WINDOWS.md`
- **Verification:** gofmt -l swift/ empty; structural greps pass
- **Committed in:** 9a5130d

---

**Total deviations:** 3 auto-fixed (2 blocking, 1 transient-artifact cleanup)
**Impact on plan:** All necessary to satisfy the plan's own mandated test shape and to keep the tree clean. No scope creep.

## Issues Encountered

None beyond the deviations above. The flagged Term-family limitation required no work — it is documented, not fixed, exactly as the plan prescribes (no schema changes).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `swift/internal/symbol` is the frozen vocabulary for both indexing paths: Phase 2's fallback and Phase 3's semantic path consume SymbolInput/Container/DeclKind + ScipKind; the golden table and properties are the regression contract for any namer change
- The dual-path identity and canonical-#if rules are now normative documentation the Phase 2/3 plans must implement against
- Rows 30-31 (property-wrapper storage accessors, self/Self) intentionally emit no symbol; macros (row 29) are named but their syntax-tree recognition is a Phase 2 concern flagged in RESEARCH
- Unrun gates carried from 01-01 remain open in WINDOWS.md (nix flake check on a nix-enabled host; prettier check now also covers the new README section)

## Self-Check: PASSED

- Created files exist on disk: swift/internal/symbol/scheme.go, swift/internal/symbol/roundtrip_test.go — FOUND
- Modified files exist: swift/internal/symbol/namer.go, namer_test.go, swift/README.md, swift/go.mod — FOUND
- Commits exist on gsd/v1.0-milestone: cf53512 (test), abdc3d6 (feat), 9a5130d (refactor) — FOUND
- TDD gate sequence in git log: test(01-02) -> feat(01-02) -> refactor(01-02) — VERIFIED
- Plan-level verification: swift/ GOWORK=off `go test ./...` green; repo-root `go test ./swift/...` green; full multi-module sweep (root, bindings plain + `-tags asserts`, reprolang, swift) green; gofmt clean; Descriptor_Package absent; Sprintf sweep = "+N" and "\_N" only
- Unrun: prettier --check via nix (host lacks nix) — recorded in WINDOWS.md

---
*Phase: 01-symbol-scheme-module-foundations*
*Completed: 2026-08-16*
