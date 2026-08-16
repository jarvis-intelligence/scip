# Roadmap: scip-swift

## Overview

scip-swift grows from an empty fork position to a shipped Swift indexer in six phases that de-risk in dependency order. First, the hardest correctness problem — Swift symbol identity — is frozen as a spec and shared namer, alongside the new `swift/` Go module wired into the repo's workspace/CI/Nix and a spike that validates the SourceKit-LSP semantic path with evidence. Second, the tree-sitter fallback path proves the entire emission pipeline (records → assembly → canonicalization → goldens → `scip lint`) deterministically on Linux CI with zero toolchain dependency, delivering the "always produces something" guarantee. Third — the core value delivery — the semantic path drives SourceKit-LSP over SwiftPM projects to produce accurate definitions, references, document symbols, imports, and test-role marking. Fourth, hierarchy and relationship edges (conformances, overrides, call/type hierarchies) are layered on as relation mapping over that plumbing. Fifth, the Xcode `--scheme` backend extends coverage to xcodeproj workspaces and one-index multi-target assembly. Finally, dual-arch release artifacts ship and the milestone's done-criterion is proven: jarvis MCP tools answer structural queries on a real Swift project indexed by scip-swift.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Symbol Scheme & Module Foundations** - Freeze the Swift SCIP symbol scheme (shared namer, extension attribution, ParseSymbol round-trip), stand up the `swift/` module in workspace/CI/Nix, and validate the SourceKit-LSP path via spikes
- [ ] **Phase 2: Fallback Indexer & Emission Pipeline** - Tree-sitter fallback indexes Swift trees end-to-end: deterministic, `scip lint`-clean, degraded-mode-signaled output with golden snapshot fixtures passing on CI
- [ ] **Phase 3: Semantic Indexing on SwiftPM** - SourceKit-LSP semantic path delivers accurate definitions/references, document symbols, import occurrences, and test-role marking on SwiftPM packages
- [ ] **Phase 4: Hierarchies & Relationships** - Protocol conformances, overrides, and call/type hierarchy edges emitted as SCIP relationships powering callHierarchy and typeHierarchy
- [ ] **Phase 5: Xcode Project Support** - `--scheme` backend indexes Xcode projects; one index covers all targets/modules with per-module symbol disambiguation
- [ ] **Phase 6: Release & End-to-End Validation** - Dual-arch macOS release artifacts ship and jarvis MCP answers all five structural queries on a real Swift project indexed by scip-swift

## Phase Details

### Phase 1: Symbol Scheme & Module Foundations
**Goal**: Freeze the Swift SCIP symbol scheme as spec + shared namer (the vocabulary both indexing paths speak), stand up the `swift/` Go module wired into the repo's workspace/CI/Nix, fix the `ParseSymbol` Unicode panic, and record spike evidence validating the SourceKit-LSP semantic path for v1
**Mode:** mvp
**Depends on**: Nothing (first phase)
**Requirements**: [SYM-01, SYM-02]
**Success Criteria** (what must be TRUE):
  1. Every Swift symbol category — SwiftPM modules, types, overloaded functions, backtick-escaped operators, `init`/accessors, extension members, function locals — has a defined SCIP symbol string in a frozen spec, and the shared namer's output round-trips through `scip.ParseSymbol`/`FormatSymbol` (property tests pass, including emoji/multi-byte identifiers)
  2. A member declared in `extension Foo` — including in a different file from the type — is named under the extended type's symbol path (`<module>/Foo#member`) by the shared namer, so no later phase mis-attributes extension members
  3. `scip.ParseSymbol` no longer panics on symbol strings ending in multi-byte runes; a fuzz test in `bindings/go/scip` proves it and all existing module tests stay green
  4. The new `swift/` module builds and its tests pass through `go.work`, the paths-filtered CI job, and `nix flake check` — nothing in the existing repo (Go workspace, bindings codegen, reprolang tests) breaks
  5. Spike findings are recorded with measured evidence: a SourceKit-LSP capability inventory (overloads, retroactive extensions, generics, witnesses) and harvest performance on a fixture repo, with an explicit go/no-go verdict on the LSP semantic path for v1
**Plans**: 3 plans

Plans:
- [ ] 01-01: swift/ module skeleton (go.work entry, ci.yaml paths-filter, checks.nix) + ParseSymbol multi-byte panic fix with fuzz test
- [ ] 01-02: Swift SCIP symbol-scheme spec + shared namer with `scip.ParseSymbol` round-trip property tests (overload disambiguators, operator escaping, init/accessor kinds, extension attribution, locals)
- [ ] 01-03: SourceKit-LSP capability-inventory spike + LSP-harvest performance spike against a perf-budget fixture (verdict evidence for the v1 semantic path)

### Phase 2: Fallback Indexer & Emission Pipeline
**Goal**: The tree-sitter fallback path indexes Swift files end-to-end — definitions, document symbols, intra-file references — through the full emission pipeline, producing deterministic, `scip lint`-clean, degraded-mode-signaled `.scip` output with golden snapshot fixtures passing on CI (Linux, no Xcode)
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: [SYM-03, FBQ-01, FBQ-02]
**Success Criteria** (what must be TRUE):
  1. With no Swift toolchain or build available, scip-swift still indexes a Swift tree via tree-sitter: definitions, document symbols, and intra-file references appear in the emitted `.scip` index, explicitly flagged as degraded (mode in `ToolInfo`/metadata + CLI warning) and never silently merged with semantic output; `--strict-semantic` fails loudly instead
  2. The emitted index passes `scip lint` with zero errors, and two consecutive runs over the same tree produce byte-identical output (canonical ordering, relative paths, UTF-8 positions — emoji fixtures included)
  3. Golden snapshot fixtures covering extensions, protocols, generics, operators, accessors, `#if` blocks, and test targets pass `scip snapshot` on CI, including Linux runners with no Xcode
**Plans**: 2 plans

Plans:
- [ ] 02-01: Vendor tree-sitter-swift grammar (committed generated parser) + fallback indexer (document symbols, definitions, intra-file references) with degraded-mode signaling and `--strict-semantic`
- [ ] 02-02: Deterministic emission (canonicalize + sort, UTF-16→UTF-8 position normalization, dedup) + golden snapshot harness + `scip lint`/`scip snapshot` subprocess CI gates

### Phase 3: Semantic Indexing on SwiftPM
**Goal**: The semantic path drives SourceKit-LSP over SwiftPM projects to deliver the navigation core — precise definitions and references with roles, document symbols, import occurrences, and test-target marking — with readiness gating so a cold index never yields silently-wrong output
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: [PROJ-01, NAV-01, NAV-02, NAV-03, SYM-04]
**Success Criteria** (what must be TRUE):
  1. Running scip-swift on a SwiftPM package (`Package.swift`, build + semantic harvest) produces an index where goToDefinition lands on the correct declaration and findReferences returns all uses — for types, funcs, methods, vars, enum cases, accessors, and params — with read/write roles distinguishing accesses
  2. documentSymbols returns a correct per-file outline: types and members nested via symbol paths, locals carried via `enclosing_symbol`
  3. Every `import Foo` statement emits an occurrence with the `Import` role resolving to the module's symbol
  4. Occurrences in test targets are marked with `SymbolRole.Test`, so a consumer can locate the tests for any symbol
  5. On a clean macOS runner with Swift ≥ 6.1 and a cold cache, the same definitions and references come back — the readiness gate proves background indexing completed before harvest (no empty-but-lint-clean output)
**Plans**: 3 plans

Plans:
- [ ] 03-01: SourceKit-LSP client (spawn/initialize, position-encoding negotiation, readiness gate via `sourcekit/isIndexing` + watchdog, bounded concurrency) + SwiftPM workspace backend with build priming
- [ ] 03-02: Semantic harvest — documentSymbol + symbolInfo (USR) + definition + references per unique definition, USR→SCIP mapping through the shared namer, SDK/dependency filtering, per-file degradation to fallback, dual-path symbol-parity goldens
- [ ] 03-03: Import occurrences with `Import` role, test-target `SymbolRole.Test` marking, clean-runner reproducibility check on pinned-Xcode macOS CI

### Phase 4: Hierarchies & Relationships
**Goal**: Relationship edges layered over the semantic plumbing — protocol conformances and class overrides as SCIP relationships, and call/type hierarchies answerable from the emitted index
**Mode:** mvp
**Depends on**: Phase 3
**Requirements**: [REL-01, REL-02, REL-03]
**Success Criteria** (what must be TRUE):
  1. Protocol conformances emit `is_implementation` relationships — both type-level and requirement-witness-level — and class overrides emit override relationships, visible in `scip print` of the fixture index
  2. Incoming and outgoing calls are answerable for every indexed function/method (callHierarchy)
  3. Supertypes, subtypes, and protocol implementations are answerable for every indexed type (typeHierarchy), including types whose conformances or members live in extensions
  4. Relationship-bearing fixtures (conformances in extensions, subclass chains, witness methods, dynamic dispatch) pass `scip snapshot` goldens and `scip lint` on CI
**Plans**: 2 plans

Plans:
- [ ] 04-01: Relationship emission — protocol conformance `is_implementation` (type-level + witness-level per scip.proto semantics), class override relationships, `isDynamic`/receiver handling
- [ ] 04-02: Call/type hierarchy mapping from LSP hierarchy requests into occurrence roles and `SymbolInformation.relationships`, verified against the Phase-1 capability inventory

### Phase 5: Xcode Project Support
**Goal**: The `--scheme` backend indexes Xcode projects (the jarvis README promise) with shared-scheme discovery, build priming, and actionable failures; one index covers all targets/modules of a project with per-module symbol disambiguation
**Mode:** mvp
**Depends on**: Phase 4
**Requirements**: [PROJ-02, PROJ-03]
**Success Criteria** (what must be TRUE):
  1. `scip-swift --scheme <SharedScheme>` on an Xcode project discovers shared schemes, primes the build, and produces a semantic index where definitions and references work across the project's sources
  2. When the scheme cannot be built or index data is missing, the CLI exits with an actionable error naming the problem — never a silently wrong or empty index
  3. A multi-target project (app + extension/framework + tests) produces one index covering all targets, with per-module symbol disambiguation: same-named symbols in different modules are distinct, and cross-target references resolve
**Plans**: 2 plans

Plans:
- [ ] 05-01: `--scheme` backend — shared-scheme discovery, xcodebuild priming, index-store/DerivedData readiness, actionable error paths, xcodeproj fixture under testdata/projects/ (orchestration route decided by spike)
- [ ] 05-02: Multi-target one-index assembly — per-module symbol disambiguation and cross-target reference resolution over the multi-target fixture

### Phase 6: Release & End-to-End Validation
**Goal**: Ship it — dual-arch macOS release artifacts from a pinned-Xcode CI matrix, and prove the milestone done-criterion: jarvis MCP tools answer all five structural queries correctly on a real Swift project indexed by scip-swift
**Mode:** mvp
**Depends on**: Phase 5
**Requirements**: [FBQ-03, FBQ-04]
**Success Criteria** (what must be TRUE):
  1. Release artifacts `scip-swift-darwin-arm64.tar.gz` and `scip-swift-darwin-amd64.tar.gz` build on a dual-arch macOS CI matrix, are version-synced to `cmd/scip/version.txt`, and the binaries run on both architectures
  2. jarvis MCP tools — goToDefinition, findReferences, callHierarchy, typeHierarchy, documentSymbols — return correct answers on a real Swift project indexed by scip-swift (protocol-heavy fixture included)
  3. Hardening holds at scale: large-corpus performance/memory budgets are respected (no timeouts or OOM), clean-runner reproducibility is confirmed, and `nix flake check` is fully green
**Plans**: 2 plans

Plans:
- [ ] 06-01: Release pipeline — dual-arch macOS CI matrix (pinned Xcode, CGO_ENABLED=1 native builds), tarball artifacts version-synced to version.txt, `ToolInfo.version` derivation
- [ ] 06-02: jarvis end-to-end validation (real project → `jarvis index` → MCP query assertions across all five tools) + hardening sweep (large-corpus perf/memory budget, clean-runner check, full flake check)

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Symbol Scheme & Module Foundations | 0/3 | Not started | - |
| 2. Fallback Indexer & Emission Pipeline | 0/2 | Not started | - |
| 3. Semantic Indexing on SwiftPM | 0/3 | Not started | - |
| 4. Hierarchies & Relationships | 0/2 | Not started | - |
| 5. Xcode Project Support | 0/2 | Not started | - |
| 6. Release & End-to-End Validation | 0/2 | Not started | - |

---
*Roadmap created: 2026-08-16 — 17 v1 requirements mapped across 6 phases (milestone v1.0: Add Swift support)*
