# Project Research Summary

**Project:** scip-swift — hybrid Swift SCIP indexer in the scip monorepo (SourceKit-LSP primary + tree-sitter fallback; macOS arm64 + x86_64; consumer is jarvis MCP)
**Domain:** Developer tooling — source-code indexing / code intelligence (SCIP emission)
**Researched:** 2026-08-16
**Confidence:** HIGH overall (two flagged MEDIUM decision areas: semantic-path performance at scale, Xcode-scheme orchestration)

## Executive Summary

No Swift SCIP indexer exists today — Swift is absent from the Sourcegraph indexer family — and the precise-navigation lane for Swift is occupied only by live-editor tooling (SourceKit-LSP, Xcode's locked-in index). scip-swift's niche is exactly the open ground: a portable, offline, compiler-grade `.scip` file that answers all five jarvis MCP tools (goToDefinition, findReferences, callHierarchy, typeHierarchy, documentSymbols) with no server process. Research unanimously recommends building it as a **new Go module in this repo** (`swift/`, following the verified reprolang precedent: own `go.mod` with a `replace` to `bindings/go/scip`, vendored tree-sitter grammar via cgo, testutil golden snapshots, `scip lint`/`scip snapshot` subprocess validation gates), because the repo's entire quality chain — Protobuf emission, canonicalization, snapshot harness, CI, Nix, release matrix — is Go, and the semantic engine is consumed as a subprocess anyway.

The researchers diverged on the semantic source and this synthesis makes the call: **drive SourceKit-LSP as a subprocess over LSP/stdio for v1** (STACK.md + ARCHITECTURE.md), using its documented `textDocument/symbolInfo` USR extension as the symbol-identity backbone — **not** direct compiler index-store consumption (FEATURES.md's preference). The index store is genuinely the better *bulk harvester* (roles/relations come free, `swift build`/`xcodebuild` produce it incrementally as a side effect, scip-clang and Periphery are the precedents), but libIndexStore/indexstore-db have no maintained Go bindings: consuming them means either hand-written cgo against a toolchain-shipped dylib or a Swift helper process — and Swift-in-Nix is broken (nixpkgs Swift is 5.10-era; Darwin gap is a multi-year open issue), which would force exactly the CI exception the project constraints forbid. SourceKit-LSP is itself backed by the compiler index, background indexing is default-on since Swift 6.1, and it is a *runtime-only* dependency on every target Mac. Crucially, FEATURES.md's semantic vocabulary (USR identity, role/relation mapping, the extension-attribution rule) is **source-independent and adopted wholesale** — so if the LSP harvest proves too slow at scale (the one real risk, per Pitfall 2), switching the data source to an index-store sidecar in v2 preserves the entire emission and symbol-layer design.

Top risks, all with designed mitigations: querying a cold index yields silently-wrong-but-lint-clean output (readiness gate + clean-runner CI check); per-symbol LSP round-trip storms on large repos (harvest discipline: one documentSymbol pass + per-unique-definition references, bounded concurrency, perf-budget fixture, index-store escape hatch); UTF-16→UTF-8 position corruption and a verified `ParseSymbol` Unicode panic in this repo's own bindings (fix the panic first; normalize positions at the boundary; emoji fixtures from day one); silent degradation to tree-sitter (mode stamped in `ToolInfo`, `--strict-semantic` flag); and macOS CI reality (repo CI is Linux-only today; Intel runners disappear ~Fall 2027 — build both arches natively now via `macos-15` + `macos-15-intel`, document the cliff).

## Key Findings

### Recommended Stack

Go is the implementation language, rejecting the scip-* "write it in the target language" convention. SourceKit-LSP is consumed over LSP/stdio (language-agnostic), the repo's validated patterns (reprolang module layout, cgo grammar vendoring, go-tree-sitter v0.25.0 already pinned, testutil snapshots) are directly reusable, SCIP emission is solved by the in-repo `bindings/go/scip` Protobuf module, and the Nix/check/release machinery stays untouched. Swift remains a runtime prerequisite (discovered via `xcrun -f sourcekit-lsp`), never a build-time one.

**Core technologies:**
- **Go 1.25** — indexer implementation language; new in-repo module `swift/` (4th go.work entry), reprolang pattern
- **SourceKit-LSP subprocess** (require Swift ≥ 6.1; local ground truth 6.2.4 / Xcode 26.3) — primary semantics: definitions, references, document symbols, call/type hierarchies, plus the documented `textDocument/symbolInfo` (USR identity), `sourcekit/workspace/symbolNames`, `sourcekit/isIndexing` extensions
- **go-tree-sitter v0.25.0 + vendored tree-sitter-swift grammar 0.7.3** (alex-pinkus; parser self-generated `--abi 14`, committed like reprolang) — fallback path; pure syntax, runs anywhere including Linux/Nix CI
- **`bindings/go/scip`** (in-repo, protobuf-go 1.36.x) — emit, canonicalize, sort; never shell out for Protobuf
- **sourcegraph/jsonrpc2 v0.2.2** + ~10 hand-written LSP request structs — thin LSP client (~300 LoC); go.lsp.dev is stale
- **xcode-build-server v1.3.0** (runtime, Homebrew) — BSP bridge for the `--xcode-scheme` path; orchestration details are a phase-1 spike
- **Autogold v2 + testify** (already in repo) — golden snapshot and assertion layer

### Expected Features

**Must have (table stakes — v1 contract):**
- Definitions + references with read/write roles — the navigation core; USR→SCIP symbol translation is the real work
- Swift SCIP symbol scheme — hardest table-stakes item: backtick-escaped operators, overload disambiguators (`foo(+1).`), `init` as Constructor, **extension members scoped under the extended type** (`Module/ExtendedType#member`), `local <id>` for function-locals; decide and freeze before any emitter code
- Protocol conformances → `is_implementation` relationships; class overrides — powers typeHierarchy implementations and reference grouping
- Document symbols; call + type hierarchy edges from relations — mapping work, not graph algorithms
- SwiftPM project indexing (primary fixture path); Xcode `--scheme` indexing (jarvis README promise; Periphery UX precedent)
- `scip lint`-clean, deterministic, snapshot-tested output; tree-sitter degraded fallback (mode-signaled); macOS arm64 + x86_64 binaries

**Should have (differentiators):**
- Offline static index with no live server — the core strategic advantage vs every LSP-based option
- Cross-target/cross-module resolution in one index; test-target `Test` role marking; import statements as occurrences
- Doc comments via syntax overlay at definition sites (v1.x); external symbol stubs via USR demangling (v1.x); module/target filtering
- Note: FEATURES.md's `--skip-build`/store-reuse differentiator presumes the index-store path; under the LSP verdict its v1 approximation is warm background indexing + CI caching of `.build/`, with true store reuse deferred to a possible v2 index-store backend

**Defer (v2+) / anti-features (do not build):**
- Generic specialization resolution, full ObjC/C/C++ indexing (recommend scip-clang composition), live daemon/re-index, rename refactoring, custom semantic analysis on tree-sitter, `scip.proto` schema changes, Linux support, search/embeddings — each researched and explicitly rejected with rationale in FEATURES.md

### Architecture Approach

One Go CLI, two indexing engines substituting per-file (never merged — merging name-based and USR-based identity breaks lint and navigation): semantic path (SourceKit-LSP oracle) primary; tree-sitter path activates globally (no toolchain / non-buildable) or per-file (query failure). Both feed one shared `Record` type and one assembly layer.

**Major components:**
1. **`internal/backend/`** — detect SwiftPM vs xcodeproj vs bare tree; prime the build (readiness precondition)
2. **`internal/lsp/`** — spawn/manage SourceKit-LSP; initialize handshake; position-encoding negotiation; `symbolInfo`/`definition`/`references`/`documentSymbol`/hierarchies; readiness gating via `sourcekit/isIndexing` with watchdog
3. **`internal/symbol/`** — USR → SCIP symbol mapper (shared "namer" used by both paths; disambiguators, escaping, extension attribution)
4. **`internal/fallback/` + `grammar/`** — tree-sitter-swift vendored cgo; degraded defs/doc-symbols/intra-file refs
5. **`internal/emit/`** — UTF-16→UTF-8 position normalization, dedup, `SymbolInformation` completeness, canonicalize + sort → `*scip.Index`
6. **Validation harness** — `testutil` goldens + `scip` CLI as subprocess (cmd/scip is package main; reprolang's `test_cmd_test.go` is the template)

### Critical Pitfalls

1. **Querying a cold index** — empty/stale-but-lint-clean results; background indexing is SwiftPM-only, 2–3x build time cold, separate from `swift build`, no reliable done-notification. Avoid: explicit readiness gate from the first semantic commit, probe known symbols, clean-runner CI check, enforce Swift ≥ 6.1.
2. **LSP round-trip storms** — O(files × symbols) queries; LSP is an editor protocol, not a batch indexer (why scip-clang drives ASTs, not clangd). Avoid: documentSymbol pass + one `references` per unique definition, bounded concurrency, watchdog/respawn, perf-budget fixture; the index-store sidecar is the documented escape hatch.
3. **UTF-16/UTF-8 position mismatch + verified `ParseSymbol` Unicode panic** — Swift's Unicode identifiers make both certain, not edge cases; `scip lint` panics on multi-byte symbol endings (CONCERNS.md-verified). Avoid: fix the one-line `peekNext` guard before mass emission (upstream-worthy PR), normalize positions at the LSP boundary, always emit `UTF8CodeUnitOffsetFromLineStart`, emoji fixtures from day one.
4. **Symbol-scheme instability / dual-path drift** — same entity named differently per path or per release breaks jarvis dedup. Avoid: scheme spec frozen in phase 1, one shared namer, dual-path symbol-parity golden test.
5. **Silent degradation to fallback** — confidently-wrong navigation erodes trust. Avoid: mode stamped in `ToolInfo`/metadata, CLI warning, `--strict-semantic` flag, published accuracy delta.
6. **macOS CI reality** — repo CI is Linux-only today; Intel runners retire ~Fall 2027. Avoid: new pinned-Xcode macOS arm64 job for semantic tests, Linux runs fallback/emission everywhere, native `macos-15-intel` builds now (CGO_ENABLED=1; the scip CLI's CGO_ENABLED=0 cross-compile trick doesn't apply), documented deprecation cliff.

## Implications for Roadmap

Based on research, suggested phase structure (mirrors both ARCHITECTURE.md's dependency-ordered build order and PITFALLS.md's pitfall-to-phase mapping):

### Phase 1: Foundations, Symbol Scheme, and Spike
**Rationale:** The riskiest integration surface (new module, cgo grammar, workspace/CI/Nix wiring) and the hardest correctness problem (symbol identity) come first, so later phases never block on identity churn. The semantic-path decision gets empirically validated before it is load-bearing.
**Delivers:** `swift/` module skeleton wired into `go.work`, `ci.yaml` paths-filter, `checks.nix` atomically; `Record` type + emit/assembly + `scip lint`/snapshot subprocess gates; **the ParseSymbol Unicode panic fix + fuzz test** (before first emitted index); position-encoding contract; Swift symbol-scheme spec (extension-attribution rule, disambiguators, escaping) + shared namer with `scip.ParseSymbol` round-trip property tests; canonical `#if` configuration policy; SourceKit-LSP capability-inventory spike (protocol witnesses, retroactive extensions, overloads, generics, ObjC bridging — record what actually comes back) + LSP-harvest performance spike against a perf-budget fixture.
**Addresses:** symbol scheme (P1 feature), determinism foundations.
**Avoids:** Pitfalls 4, 5, 11, 13; de-risks Pitfalls 2 and 10 (the spike is where the LSP-vs-index-store verdict gets empirical evidence).

### Phase 2: Tree-sitter Fallback End-to-End
**Rationale:** The fallback validates the entire emission pipeline deterministically, on Linux/Nix CI, with zero Xcode dependency — everything later reuses `emit/` unchanged. It also delivers the "always produces something" guarantee jarvis needs.
**Delivers:** Vendored alex-pinkus grammar 0.7.3 (parser generated `--abi 14`, committed), fallback indexer (doc symbols, defs, intra-file refs), mode signaling in `ToolInfo` + `--strict-semantic`, ERROR/MISSING-node accounting, degraded-mode golden snapshots.
**Uses:** go-tree-sitter v0.25.0, reprolang's `generate-tree-sitter-parser.sh` pattern, testutil.
**Implements:** fallback + grammar + emit components.
**Avoids:** Pitfalls 6, 7; enforces 3 and 5 (emoji fixtures, symbol parity) continuously.

### Phase 3: Semantic Core on SwiftPM Projects
**Rationale:** The core value delivery (defs/refs/doc symbols in jarvis). SwiftPM is SourceKit-LSP's first-class workspace type — background indexing works there — deferring all build-server complexity.
**Delivers:** LSP client (spawn/initialize/position negotiation, readiness gate, watchdog/respawn, bounded concurrency), `documentSymbol` + `symbolInfo` + `definition` + `references` harvest, USR mapper wiring, SDK/dependency occurrence filtering (project files only, minimal external stubs), per-file degradation to fallback.
**Uses:** jsonrpc2 v0.2.2, SourceKit-LSP extensions.
**Implements:** backend/swiftpm + lsp + semantic runner.
**Avoids:** Pitfalls 1, 2 (discipline + budget), 8 (SwiftPM only), 9 (SDK filtering), 12 (deterministic-vs-recorded fixture split, pinned Xcode runners); 15 mitigated (emit-as-you-go rather than whole-index accumulation).

### Phase 4: Hierarchies and Relationships
**Rationale:** Pure superset of Phase 3's plumbing — verified-supported capabilities that additionally require a fully built index; relation mapping work, not computation.
**Delivers:** `prepareCallHierarchy`/incoming/outgoing and `prepareTypeHierarchy`/supertypes/subtypes → occurrence roles + `SymbolInformation.relationships`; protocol conformance `is_implementation` links (witness methods per the scip.proto TypeScript-example semantics); `isDynamic`/`receiverUsrs` handling for protocol dispatch; extension attribution verified against observed behavior from the Phase-1 inventory.
**Addresses:** callHierarchy, typeHierarchy, protocol-grouping features.
**Avoids:** Pitfall 10 (emission rules from spike observations, conservative relationships, documented limitations).

### Phase 5: Xcode `--scheme` Backend + Dual-Arch CI and Release
**Rationale:** Last because it is the least-documented integration (spike-gated: xcode-build-server BSP vs synthesized compile_commands.json) and only affects backend selection; SwiftPM already covers the majority of indexable repos. CI/release land here because they need a working indexer to ship.
**Delivers:** `--scheme` flag end-to-end (xcodebuild priming, store/DerivedData discovery, shared-scheme detection with actionable errors, timeout-guarded cached discovery), `testdata/projects/` xcodeproj fixture, macOS arm64 + x86_64 CI matrix (pinned Xcode), release artifacts `scip-swift-darwin-{arm64,amd64}.tar.gz` version-synced to `cmd/scip/version.txt`, `ToolInfo.version` derived from version.txt.
**Uses:** xcode-build-server v1.3.0, existing release.yaml matrix extension.
**Avoids:** Pitfalls 8, 14; 13 enforced (full `nix flake check` green).

### Phase 6: Hardening and jarvis End-to-End Validation
**Rationale:** The milestone's done-criteria — real-project proof — belongs last, on top of a feature-complete indexer.
**Delivers:** Large-corpus perf/memory budget tests (heap-capped CI job; index-bytes-per-LOC assertion), clean-runner reproducibility check, jarvis e2e (real project → `jarvis index` → MCP query assertions across all five tools, protocol-heavy fixture included), jarvis README/docs updates (both-arch claim), full "Looks Done But Isn't" checklist sweep.
**Avoids:** Pitfalls 12, 14, 15 verification; every checklist item from PITFALLS.md.

### Phase Ordering Rationale

- **Emission before semantics:** the fallback (Phase 2) proves records→assembly→canonicalization→goldens→lint with zero toolchain dependency, so semantic work is never blocked on pipeline bugs (ARCHITECTURE.md Pattern/build-order 1).
- **Identity before LSP:** the symbol mapper (Phase 1) is the shared vocabulary both paths speak; doing it first means LSP integration never churns on identity design (build-order 2).
- **SwiftPM before Xcode:** first-class vs spike-gated workspace support; defers the least-documented integration and its MEDIUM-confidence decisions (build-order 5).
- **Hierarchies after defs/refs:** they need a fully built index and are relation-mapping over Phase 3's plumbing (build-order 4).
- **Every pitfall has a prevention phase and a verification phase** — mapped explicitly in PITFALLS.md; Phase 1 absorbs the four design-decision pitfalls (4, 5, 11, 13) so nothing expensive is retrofitted.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1:** the SourceKit-LSP capability-inventory spike and the LSP-harvest perf spike are the two highest-value unknowns; the semantic-path verdict (LSP for v1) should be re-confirmed against spike evidence with explicit criteria for invoking the v2 index-store escape hatch. USR grammar stability across Xcode majors also needs a test strategy.
- **Phase 3:** readiness-gating semantics (background-indexing behavior, `sourcekit/isIndexing` reliability), UTF-8 position-encoding negotiation support (unverified in SourceKit-LSP), role fidelity (Read/Write bits) from `references` results.
- **Phase 5:** xcode-scheme orchestration (xcode-build-server vs compile_commands synthesis — MEDIUM confidence, spike-gated); GitHub macOS/Intel runner landscape near the ~Fall 2027 cliff.

Phases with standard patterns (skip research-phase):
- **Phase 2:** direct copybook from reprolang (module layout, grammar vendoring, snapshot wiring) — verified in-repo precedent.
- **Phase 4:** relation mapping over established LSP plumbing; rules pre-specified by FEATURES.md's semantics cheat sheet and the Phase-1 inventory.
- **Phase 6:** standard e2e/budget CI patterns; the checklist is already enumerated in PITFALLS.md.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Verified against local toolchain (Xcode 26.3, Swift 6.2.4), repo pins, and current upstream releases; per-item sources cited in STACK.md |
| Features | HIGH | Schema claims verified against local `scip.proto`; index-store model verified against `indexstore.h`; behavioral claims (LSP freshness) tagged MEDIUM where forum-sourced |
| Architecture | HIGH | Repo-internal facts read from source; SourceKit-LSP extension surface from official contributor docs; behavioral details (position negotiation, background-indexing timing) MEDIUM and flagged |
| Pitfalls | HIGH | Repo-specific pitfalls verified by inspection (CONCERNS.md, ci.yaml, lint.go); external pitfalls source-cited |

**Overall confidence:** HIGH — with two named MEDIUM decision areas (semantic-path performance at scale; Xcode-scheme orchestration), both gated behind Phase-1/5 spikes rather than assumptions.

### Gaps to Address

- **LSP harvest performance at scale:** no measured evidence yet that documentSymbol + per-unique-def references meets a sane budget on 500+ file repos. Handle: Phase-1 perf spike with fixture budget; pre-agreed criteria for switching v2 to an index-store sidecar (Swift helper or cgo libIndexStore) — the symbol/emission layers survive the switch unchanged.
- **Xcode-scheme orchestration:** xcode-build-server BSP vs synthesized compile_commands.json undecided (MEDIUM). Handle: Phase-5 gate, informed by a small spike; both routes documented in STACK/ARCHITECTURE.
- **SourceKit-LSP UTF-8 negotiation & role fidelity:** unverified whether the server accepts `utf-8` positionEncoding (assume UTF-16, keep the translation layer) and whether `references` results carry enough read/write fidelity for full `SymbolRole` bits. Handle: Phase-3 validation, coarse roles acceptable in v1.
- **USR stability across Xcode majors:** semi-documented grammar. Handle: treat USRs as opaque keys, versioned mapper, golden tests per Xcode major.
- **Call-hierarchy maturity for dynamic dispatch:** limited/immature per one MEDIUM source. Handle: covered by Phase-1 capability inventory; `isDynamic`/`receiverUsrs` fallback design already sketched.
- **testutil flag registration:** `-update-snapshots` registers at init (CONCERNS) — may bite the swift module's own test binary. Handle: private `flag.FlagSet` or fix in Phase 1.
- **jarvis consumer contract details:** exact tool surface and `--scheme` wording to be re-verified from the jarvis-index repo during Phase 6 e2e.

## Sources

### Primary (HIGH confidence)
- Local repository (read 2026-08-16): `scip.proto` (symbol grammar, `Language.Swift = 2`, Swift-specific Kinds, PositionEncoding, Relationship semantics), `go.work`, `reprolang/` (module layout, grammar vendoring, namer), `bindings/go/scip/` + `testutil/`, `cmd/scip/lint.go`, `.github/workflows/{ci,release}.yaml`, `flake.nix`, `cmd/scip/version.txt`, `.planning/codebase/CONCERNS.md`
- Local toolchain ground truth: Xcode 26.3, Apple Swift 6.2.4, `sourcekit-lsp --help` (workspace types `swiftPM|compilationDatabase|buildServer`)
- SourceKit-LSP official docs — repo README, Contributor Documentation/LSP Extensions.md (`symbolInfo`, `workspace/symbolNames`, `isIndexing`), background-indexing documentation (Swift 6.1+ default, SwiftPM-only, 2–3x cold cost)
- swiftlang/llvm-project `indexstore.h` — authoritative index-store data model (kinds, roles, relations)
- LSP 3.17 specification — position encodings, hierarchy requests
- alex-pinkus/tree-sitter-swift (0.7.3, 2026-06-01) and tree-sitter/go-tree-sitter v0.25.0 — verified via GitHub API/pkg.go.dev
- SolaWing/xcode-build-server v1.3.0 + Homebrew formula — existence/role HIGH, orchestration MEDIUM
- GitHub runner changelogs/issues — macos-13 retirement (Dec 2025), `macos-15-intel` transitional label, Intel cliff ~Fall 2027
- Periphery, scip-clang Design.md, indexstore-db, MobileNativeFoundation/swift-index-store — CLI/architecture precedents for compile-driven indexing

### Secondary (MEDIUM confidence)
- Swift Forums threads — background indexing usage/limits (#79497), hierarchies require a built project (#77058), Xcode-project LSP setup pain (#86000), pre-generating index DBs (#80870)
- sourcekit-lsp issues — #730 (no native xcodeproj), #1269/#1271 (background-indexing workspace limits), #1837 (indexing slowness), #1626, #2693
- nixpkgs Swift documentation + issues #37437/#37473 — Swift-on-Nix lag and Darwin gap
- swiftlang sourcekit-lsp Swift Package Index feature matrix — call/type hierarchy support
- alexey1312/swift-index — approximate-search MCP competitor (contrasts the precise lane)

### Tertiary (LOW confidence / needs validation)
- SourceKit-LSP call-hierarchy maturity for protocol/dynamic dispatch — single-source observation, validate in Phase-1 spike
- UTF-8 positionEncoding negotiation support in SourceKit-LSP — unverified; assume UTF-16
- Blog/anecdotal reports (Peter Kos background-indexing post, dimillian xcodebuild slowness) — consistent with primary sources but individually LOW

---
*Research completed: 2026-08-16*
*Ready for roadmap: yes*
