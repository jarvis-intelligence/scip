---
phase: 01-symbol-scheme-module-foundations
verified: 2026-08-16T11:08:42Z
status: human_needed
score: 22/23 must-haves verified
behavior_unverified: 0
overrides_applied: 0
behavior_unverified_items: []
human_verification:
  - test: "Run the Nix gates on a nix-enabled host (or observe the first CI run): nix build .#checks.x86_64-linux.swift, nix flake check, nix develop --command prettier --check swift/README.md swift/spikes/2026-08-sourcekit-lsp-findings.md"
    expected: "vendorHash sha256-Vz6S8i6udqSIykS2UmJdgZU07wCUKfuWOcytXMx1Jis= is accepted (no hash-mismatch print), the swift check builds green (subPackages now lists only real package dirs), flake check exits 0, prettier passes"
    why_human: "nix is NOT installed on this host (verified: command -v nix absent). The vendorHash was derived via a NAR-SHA256 pipeline cross-validated against the known go-bindings hash (WINDOWS.md #1/#4/#6), and the CR-01 subPackages defect was fixed in be27bd3 — but the actual nix build/flake-check/prettier executions cannot be run here. ROADMAP success criterion 4's nix leg is UNVERIFIABLE on this host by assumption, not passed."
  - test: "Human review of the flagged prohibition: dual-path identity (semantic vs fallback must never emit two different strings for one declaration)"
    expected: "Confirm the mitigation is acceptable for now: the rule is normative in swift/internal/symbol/scheme.go (Dual-path identity) and restated in swift/README.md, and a single shared namer is the only string-producing path in the codebase today; enforcement lands with the dual-path symbol-parity goldens already planned for Phase 3 (03-02)"
    why_human: "Judgment-tier prohibition (ADR-550 D4 autonomous route): non-authoritative LLM-judge verdict 'no violation observable — only one indexing path exists in Phase 1; both paths are contractually required to use the shared namer'. unverified-prohibition — human review recommended. It can only be test-enforced once the fallback (Phase 2) and semantic (Phase 3) paths both exist."
---

# Phase 1: Symbol Scheme & Module Foundations Verification Report

**Phase Goal:** Freeze the Swift SCIP symbol scheme as spec + shared namer (the vocabulary both indexing paths speak), stand up the `swift/` Go module wired into the repo's workspace/CI/Nix, fix the `ParseSymbol` Unicode panic, and record spike evidence validating the SourceKit-LSP semantic path for v1
**Verified:** 2026-08-16T11:08:42Z
**Status:** human_needed
**Re-verification:** No — initial verification (no previous VERIFICATION.md found)

**Advisory note (mode):** ROADMAP.md marks Phase 1 `Mode: mvp`, but the goal is not a User Story (`As a … I want to … so that …`), so the MVP User-Flow-Coverage table cannot be built. Standard goal-backward verification was applied instead. Roadmap metadata inconsistency for the orchestrator to reconcile (not a gap in the delivered work).

## Goal Achievement

### Observable Truths

Merged from 5 ROADMAP success criteria + 21 PLAN-frontmatter truths (deduplicated; roadmap wording kept where plans restate it).

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | SC1: Every Swift symbol category has a defined frozen SCIP symbol string; namer output round-trips through ParseSymbol/FormatSymbol (property tests incl. emoji/multi-byte) | ✓ VERIFIED | `go test -count=1 -v ./internal/symbol/...` (GOWORK=off): TestSymbolGoldenTable PASS with 37 subtests covering modules, system modules (6.2.4 + empty-version `.`), struct/enum/protocol/typealias, nested types, funcs/methods, overloads +0/+1/+2, operators (`+` raw, `` `==` `` escaped), init +0/+1, deinit, getter, escaped setter `` `area=` ``, terms, subscripts +0/+1, enum case, extension member, retroactive +(+1) collision, protocol method, witness/override, `[T]`, `(x)`, `Preview!`; TestNamerRoundTrip/TestLocalSymbolRoundTrip/TestNamerPure PASS (generators include 🚀, π, F⃗, 🄼odule, back``tick, `<->`, `==`) |
| 2 | SC2: `extension Foo` member (incl. cross-file, retroactive) named under the extended type's path by the shared namer | ✓ VERIFIED | Golden rows `scip-swift swiftpm MyMod . Shape#area2().` (extension member), `scip-swift swiftpm App . Shape#spike().` + `…spike(+1).` (retroactive: owner module App, second-module collision); scheme.go "Extension attribution (SYM-02)" normative section; spike evidence row 2/3 shows LSP USRs recover the same attribution |
| 3 | SC3: ParseSymbol no longer panics on trailing multi-byte runes; fuzz test proves it; all existing module tests stay green | ✓ VERIFIED | symbol_parser.go:375 guard is exactly `if z.byteIndex+int(z.bytesToNextRune) < len(z.SymbolString) {`; `git show 566fcf9` diff = one line; the three repro inputs verbatim in TestParseSymbolError (symbol_test.go:151-153) under require.NotPanics + err-non-nil; FuzzParseSymbol (27 seeds incl. `🚀`/`π` escaped forms) and TestParseSymbolRuneSoupNeverPanics PASS; full bindings suite fresh: `go test -count=1 ./...` AND `go test -tags asserts -count=1 ./...` both green (incl. memtest) |
| 4 | SC4: swift/ module builds + tests pass through go.work, paths-filtered CI job, and nix flake check; nothing existing breaks | ? UNCERTAIN (nix leg) — go.work + CI legs VERIFIED | go.work `use` block lists `swift`; from repo root `go build ./...` and `go test ./bindings/go/scip/... ./reprolang/... ./swift/...` all ok (workspace mode); root module standalone `GOWORK=off go test ./...` ok; swift/ `GOWORK=off go test ./...` ok. CI wiring: ci.yaml:42-43 (paths-filter swift/go.mod+go.sum), :80 (tidy loop `for dir in bindings/go/scip . reprolang swift; do`), :96 (nix-update attr checks.x86_64-linux.swift). CR-01 fix confirmed: checks.nix swift subPackages = `[ "cmd/scip-swift" "internal/symbol" ]` (both real package dirs). `nix flake check` UNVERIFIABLE — nix not installed on this host (see Human Verification #1) |
| 5 | SC5: Spike findings recorded with measured evidence: capability inventory + harvest performance + explicit go/no-go verdict | ✓ VERIFIED | swift/spikes/2026-08-sourcekit-lsp-findings.md: metadata header, 5 thresholds (flagged assumed), 14-row capability inventory each with JSONL excerpt, 9-metric cold/warm tables, `## Verdict` = **GO** with per-threshold PASS bullets, escape-hatch paragraph, How-to-re-run. Raw evidence cross-checked: perf summary records match the doc exactly (cold wall 242383ms=4m02.4s, TTR 3268ms, p50 35.273/p95 126.905ms, 13506 reqs, 0 crashes, 13001 symbols/35500 occurrences, RSS 62064KB; warm 240368ms, TTR 1025ms, p95 142.746ms, RSS 51456KB) |
| 6 | 01-01 tracer: passing test in swift/internal/symbol emits `scip-swift swiftpm MyApp . Shape#area().` and round-trips as identity | ✓ VERIFIED | TestSymbolMethodOnStructRoundTrips PASS (fresh run); TestSymbolRoundTripProperty PASS |
| 7 | 01-01: swift/go.mod shape (module path, go 1.25.0, relative replace, pinned deps) + dual-entry-point tests | ✓ VERIFIED | swift/go.mod contains all required strings + jsonrpc2 v0.2.2 (01-03) + go-cmp v0.7.0 (01-02, documented deviation); both entry points green (workspace + GOWORK=off) |
| 8 | 01-01: repo wiring triple (paths-filter, tidy loop, nix-update attr) + checks.nix swift attribute | ✓ VERIFIED | grep evidence in truth 4; checks.nix swift buildGoModule with pname scip-swift, modRoot ./swift, env.GOWORK off, real-format vendorHash |
| 9 | 01-02: golden table asserts every mapping-table row (rows 1-29 where a string is defined) | ✓ VERIFIED | See truth 1 — 37 subtests, one per row family incl. all spec-mandated strings |
| 10 | 01-02: round-trip edge (emoji/operators/backtick/multi-byte-tail) identity, no Unicode normalization | ✓ VERIFIED | TestNamerRoundTrip PASS; genNames/genLocalNames/genInput biased exactly as specified |
| 11 | 01-02: disambiguator boundaries (0 renders none, N>0 renders (+N); accessor/method ordering group) | ✓ VERIFIED | Golden rows resize()/resize(+1)/resize(+2), init()/init(+1), subscript()/subscript(+1), getter row shares `Shape#area().` with zero-arg method (Kind-distinguished) |
| 12 | 01-02: empty Module or empty Name errors, emits nothing; empty ContainerPath = top-level | ✓ VERIFIED | TestSymbolInputErrors PASS (empty_module, empty_name); ErrEmptyModule/ErrEmptyName returned before descriptor construction (namer.go:63-68); top-level rows (parse()., config.) have no containers |
| 13 | 01-02: determinism — pure function, identical output every call and under parallel calls | ✓ VERIFIED | TestNamerPure: repeated call + 8-goroutine parallel calls all byte-identical, PASS |
| 14 | 01-02: LocalSymbol sanitization + ordinal + local-form round-trip (IsLocalSymbol) | ✓ VERIFIED | TestLocalSymbolGolden: `local i`, `local count_2`, `local _` (🚀 sanitized), `local x_1`; TestLocalSymbolRoundTrip PASS |
| 15 | 01-02: DeclKind→ScipKind mapping frozen in scheme.go with the specified values | ✓ VERIFIED | All 21 mappings verified against generated scip.pb.go: Module=29, Struct=49, Class=7, Enum=11, Protocol=42, TypeAlias=55, Function=17, Method=26, Operator=34, Constructor=9, Destructor→Method(26, documented), Getter=18, Setter=45, Property=41, Constant=8, Variable=61, Subscript=47, EnumMember=12, ProtocolMethod=68, TypeParameter=58, Parameter=37, Macro=25 — exact match |
| 16 | 01-02: scheme.go doc states the three frozen-spec extras; README restates them | ✓ VERIFIED | scheme.go package doc: Determinism, Canonical #if policy, Dual-path identity (+ SYM-02 attribution, Term-family limitation, rows 30-31 emit nothing); README `## Symbol scheme` restates all of them |
| 17 | 01-03: runnable probe driver (jsonrpc2 v0.2.2 over stdio, xcrun discovery, handshake, readiness gate, JSONL with latency) | ✓ VERIFIED | swift/spikes/lsp_probe/main.go (~1000 lines); `GOWORK=off go build ./... && go vet ./spikes/...` green; discovery via xcrun with PATH fallback; meta/init/ready/harvest/hierarchy/summary phases in evidence with latency_ms |
| 18 | 01-03: capability inventory with observed evidence for every hard case, never from a cold index | ✓ VERIFIED | 14 inventory rows each citing a JSONL excerpt (overload USRs distinct, cross-file + retroactive attribution, generics, witness + existential isDynamic/receiverUsrs, operators, accessors single-symbol, Unicode/UTF-16, override, call/type-hierarchy, isIndexing -32601, 60/60 USR coverage); evidence-capability.jsonl phase order ready(idx 3-6) precedes harvest(idx 7) — gate held; isIndexing -32601 recorded verbatim |
| 19 | 01-03: harvest perf measured on ~500-file fixture, all nine metrics, prescribed discipline | ✓ VERIFIED | Deterministic generator (seeded) + cold/warm evidence files (14012 records each); summary records carry all nine metrics; discipline documented (documentSymbol per file + references per unique def, concurrency 4) |
| 20 | 01-03: findings doc complete (thresholds, inventory, numbers, verdict, escape hatch, re-run) | ✓ VERIFIED | See truth 5 |
| 21 | 01-03: verdict recorded as decision in STATE.md referencing the findings doc | ✓ VERIFIED | STATE.md line 81: "[Phase 1 / 01-03]: SourceKit-LSP semantic path verdict: GO — … see swift/spikes/2026-08-sourcekit-lsp-findings.md" |
| 22 | 01-03 prohibition: spike not promoted into internal/, no CLI wiring | ✓ VERIFIED (enforced) | `grep -rn "spikes/" swift/internal/ swift/cmd/` → 0 matches; spikes are package main run via go run only |
| 23 | 01-01/01-03: Nix gate survives go.mod changes (vendorHash refreshed same-commit; nix flake check green) | ? UNCERTAIN | Commit 763183d lands go.mod+go.sum+checks.nix together (git verified); vendorHash derived via validated NAR pipeline (WINDOWS.md #4); stale 01-02 hash incidentally fixed (WINDOWS.md #6). `nix build`/`nix flake check` UNVERIFIABLE on this host — Human Verification #1 |

**Score:** 22/23 truths verified (0 present-but-behavior-unverified; 1 UNCERTAIN → human)

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| swift/go.mod + go.sum | module manifest, relative replace, pinned deps | ✓ VERIFIED | All required strings present; jsonrpc2 v0.2.2 + go-cmp additions documented |
| swift/internal/symbol/namer.go | shared namer, validate-before-return | ✓ VERIFIED | ParseSymbol validation with `namer produced unparseable symbol %q: %w` wrap; Sprintf only at `+%d` disambiguator and `%s_%d` ordinal (allowed uses); no Descriptor_Package |
| swift/internal/symbol/scheme.go | frozen constants + DeclKind + ScipKind + normative doc | ✓ VERIFIED | Read in full; 22 DeclKind values; mapping matches scip.pb.go exactly |
| swift/internal/symbol/namer_test.go + roundtrip_test.go | golden table + rapid properties | ✓ VERIFIED | 37 golden subtests + 4 local goldens + 2 error cases + 3 properties, all PASS fresh |
| swift/README.md | Symbol scheme + Running tests + runtime-only prerequisites | ✓ VERIFIED | Sections verified; prettier check itself is nix-gated (human item) |
| bindings/go/scip/symbol_fuzz_test.go | FuzzParseSymbol + rune-soup property | ✓ VERIFIED | 27 seeds; no-crash + identity property; rapid import present |
| swift/spikes/lsp_probe + fixture + perf generator + findings doc + evidence | spike deliverables | ✓ VERIFIED | All exist; driver builds/vets green; evidence committed (capability) / git-ignored (perf, policy documented) |
| .planning/STATE.md | spike verdict decision | ✓ VERIFIED | GO decision entry present |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| swift/internal/symbol/namer.go | bindings/go/scip | scip.Symbol, VerboseSymbolFormatter, ParseSymbol via relative replace | ✓ WIRED (imports + call sites verified; tests exercise the pipeline) |
| go.work use block | swift/ module | workspace builds | ✓ WIRED (root workspace build+test green) |
| checks.nix swift attribute | flake checks | CI list/checks matrix auto-discovery | ✓ WIRED structurally (attr exists, subPackages real dirs); build green only provable on nix host |
| ci.yaml paths-filter/tidy/nix-update | swift/ manifests | same-commit wiring | ✓ WIRED (lines 42-43, 80, 96) |
| scheme.go ScipKind | scip.SymbolInformation Kind constants | single kind-mapping source | ✓ WIRED (21/21 constant names exist with matching numeric values) |
| STATE.md decision | findings doc path | explicit reference | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Panic-fix regression (error not panic) | `cd bindings/go/scip && GOWORK=off go test -count=1 -run 'TestParseSymbolError\|FuzzParseSymbol\|TestParseSymbolRuneSoupNeverPanics' -v ./...` | 3/3 PASS | ✓ PASS |
| Frozen-spec golden table | `cd swift && GOWORK=off go test -count=1 -run TestSymbolGoldenTable` | PASS (37 subtests) | ✓ PASS |
| Full workspace (nothing breaks) | repo root: `go build ./... && go test ./bindings/go/scip/... ./reprolang/... ./swift/...` (fresh via per-module -count=1 runs) | all ok incl. memtest plain + `-tags asserts` | ✓ PASS |
| swift module standalone | `cd swift && GOWORK=off go test -count=1 ./...` + `go build ./... && go vet ./spikes/...` | ok | ✓ PASS |
| Spike evidence integrity | python3 parse of evidence JSONL summary records vs findings-doc tables | exact numeric match | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| SYM-01 | 01-01, 01-02, 01-03 | Stable SCIP symbol strings per category, round-tripping through bindings parse/format | ✓ SATISFIED | Frozen scheme.go + golden table + round-trip properties; ParseSymbol panic fixed with regression/fuzz coverage; spike confirms LSP USR attribution feeds the same scheme |
| SYM-02 | 01-02 | Extension members attributed to the extended type's symbol path (cross-file + retroactive) | ✓ SATISFIED | Golden rows + normative SYM-02 rule + README; spike inventory rows 2-3 show the semantic side recovers attribution from USRs |

Orphaned requirements: none — REQUIREMENTS.md traceability maps exactly SYM-01 and SYM-02 to Phase 1; both are claimed by plans and verified. (SYM-03..FBQ-04 map to Phases 2-6.)

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none — no TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER in any phase-modified file; gofmt -l clean on swift/ and bindings/go/scip) | | | | |

Review-fix verification (01-REVIEW.md, fixed in be27bd3 — confirmed in code):
- CR-01 checks.nix subPackages → now `["cmd/scip-swift" "internal/symbol"]` with explanatory comment ✓
- CR-02 lsp_probe fail-path cleanup → `driver.fail` flushes evidence, closes conn, waits/kills server before exit; `failBeforeServer` reserved for pre-resource path ✓
- WR-02 -mode validation → `capability|perf` guard with exit 2 ✓

Remaining advisory (non-blocking, from 01-REVIEW.md): WR-01 (LocalSymbol sanitize+ordinal collision hazard — Phase 2 must group ordinals by sanitized id or the collision merges two locals; recommend addressing when fallback indexing lands), WR-03 (findUse unchecked Index), WR-04 (evidence write errors dropped), IN-01..06. None block the phase goal.

### Human Verification Required

### 1. Nix gates on a nix-enabled host (ROADMAP SC4 nix leg)

**Test:** Run `nix build .#checks.x86_64-linux.swift`, `nix flake check`, and `nix develop --command prettier --check swift/README.md swift/spikes/2026-08-sourcekit-lsp-findings.md` on a host with nix (or observe the first CI run).
**Expected:** vendorHash accepted (no `got: sha256-…` mismatch print), swift check builds green, flake check exits 0, prettier passes.
**Why human:** nix is not installed on this host (verified). The vendorHash was derived by a NAR-SHA256 pipeline cross-validated bit-for-bit against the known go-bindings hash, and the one defect that made the check unpassable (CR-01 subPackages) is fixed — but "nix flake check green" remains an assumption until executed. Recorded as WINDOWS.md #1/#3/#4/#5/#6.

### 2. Flagged prohibition: dual-path identity (unverified-prohibition)

**Test:** Review the dual-path-identity mitigation: normative rule in scheme.go/README + single shared namer as the only string producer.
**Expected:** Accept as adequate for Phase 1; enforcement arrives with the Phase-3 dual-path symbol-parity goldens (03-02) and Phase-2 fallback conformance.
**Why human:** Judgment-tier prohibition; no test can enforce it until both indexing paths exist. Autonomous verdict is non-authoritative: no violation observable today.

### Gaps Summary

No truth FAILED; no artifact MISSING/STUB; no key link NOT_WIRED; no debt-marker blockers. The single UNCERTAIN item is the nix leg of success criterion 4 (`nix flake check` + derived vendorHash), which is structurally wired and defect-fixed but unexecutable on this verification host — recorded above as human verification, consistent with the orchestrator's instruction to treat it as an explicit assumption rather than pass/fail. Two minor advisory notes: (a) ROADMAP marks the phase `Mode: mvp` with a non-User-Story goal — standard goal-backward verification was applied; (b) the working tree carries uncommitted changes to `.planning/WINDOWS.md` and `go.work.sum` for the orchestrator to commit alongside this report.

---

_Verified: 2026-08-16T11:08:42Z_
_Verifier: the agent (gsd-verifier)_
