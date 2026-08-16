# Walking Skeleton — scip-swift (Phase 1)

**Phase:** 1
**Generated:** 2026-08-16
**Mode:** mvp / walking skeleton (adapted to a Go CLI/library monorepo — not a web app)

## Capability Proven End-to-End

A minimal `swift/` Go module is wired into the repo's `go.work` workspace (and CI/Nix), and one real Swift symbol string — `scip-swift swiftpm MyApp . Shape#area().` — is emitted by the namer skeleton through `scip.VerboseSymbolFormatter` and round-tripped through `scip.ParseSymbol` + `FormatSymbol` as the identity in a passing test, proving module wiring + the symbol identity pipeline end-to-end on day one.

This is the Phase-1 special case of the tracer slice: later phases' vertical slices (fallback indexing, semantic harvest, relationships, Xcode, release) all build on this backbone without altering its architectural decisions.

## Architectural Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Module placement | New in-repo Go module `github.com/scip-code/scip/swift` (4th workspace entry; reprolang precedent) | Locked project decision — shared repo, Nix/CI reuse, relative `replace` to `../bindings/go/scip` keeps per-module `GOWORK=off` builds working |
| Symbol identity | Build `*scip.Symbol` protobufs and format via `scip.VerboseSymbolFormatter`; parse/validate via `scip.ParseSymbol` | The bindings already own grammar conformance (backtick escaping, `"."` version placeholder); hand-rolled string building is a documented anti-pattern |
| Symbol scheme | Header `scip-swift swiftpm <Module> .` (SwiftPM) / `scip-swift swift <Module> <swift-version>` (system); descriptor grammar per scip.proto; overload disambiguators `(+N)` in source order | Frozen in Phase 1 (01-02) so both indexing paths and all later goldens share one vocabulary; precedents: scip-java (`maven … List#of(+2).`), scip-python |
| Semantics strategy | Hybrid: SourceKit-LSP subprocess over LSP/stdio is primary; tree-sitter fallback is degraded-but-working; outputs never merged | Locked project decision; Phase 1 records the spike evidence + go/no-go verdict, Phase 2 builds the fallback, Phase 3 the semantic path |
| Toolchain | Go 1.25.0 (go.work pin), Nix (nix develop / flake checks) as canonical; Swift/Xcode is a runtime prerequisite only, discovered via `xcrun -f sourcekit-lsp` | Repo constraint; `swift/` must build on Linux CI with zero Swift toolchain at build time |
| Directory layout | `swift/cmd/scip-swift/` (stub now, CLI Phase 2+), `swift/internal/symbol/` (frozen scheme + namer), `swift/spikes/` (throwaway evidence tooling, never promoted) | internal/ until a second consumer justifies promotion to bindings; spikes stay out of the product path |
| Data layer | N/A — no database | This skeleton's "data" is the SCIP symbol pipeline (namer → ParseSymbol → FormatSymbol); index persistence is the existing protobuf `.scip` artifact model |
| Auth | N/A — local CLI/library | No user-facing service; no authentication surface |
| Deployment target | N/A — in-repo module; "deploy" = CI green + `nix flake check` | Release artifacts (dual-arch tarballs) are Phase 6; the skeleton's delivery channel is the repo's CI/Nix matrix |
| UI | N/A — no user interface | Consumers are the `scip` CLI, jarvis MCP tools, and Go library users |

## Stack Touched in Phase 1

- [x] Project scaffold — `swift/` Go module (go.mod/go.sum, cmd stub, README), `go.work` entry, CI paths-filter/tidy/nix-update wiring, `checks.nix` `swift` buildGoModule attribute
- [x] One real pipeline path — namer skeleton emits one real Swift symbol string through `VerboseSymbolFormatter`, round-tripped via `ParseSymbol` in a passing test (Task 1 of plan 01-01)
- [x] One real external integration boundary exercised — SourceKit-LSP subprocess over JSON-RPC (spike, evidence-grade; production client is Phase 3)
- [ ] Database read/write — N/A (no DB in this project; see Decisions)
- [ ] UI interaction — N/A (CLI/library project)
- [x] "Deployment" equivalent — the module builds and tests green through the repo's three delivery gates: go.work workspace, paths-filtered CI job, `nix flake check`

## Out of Scope (Deferred to Later Slices)

Be explicit — this list prevents later phases from re-litigating Phase 1's minimalism:

- Tree-sitter grammar vendoring + fallback indexer (Phase 2)
- Emission pipeline: canonicalization, UTF-16→UTF-8 positions, golden snapshot harness, `scip lint` gates (Phase 2)
- Production SourceKit-LSP client, SwiftPM workspace backend, semantic harvest, USR→SCIP mapping (Phase 3)
- Relationship edges: conformances, overrides, call/type hierarchies (Phase 4)
- Xcode `--scheme` backend, multi-target one-index assembly (Phase 5)
- Release artifacts, jarvis end-to-end validation, hardening (Phase 6)
- scip.proto schema changes (permanently out of scope — Swift fits existing roles/kinds; `Language.Swift = 2` verified)
- Real CLI surface for `scip-swift` (three-tier urfave/cli pattern applies from Phase 2; Phase 1 ships a stub main only)
- Property-wrapper synthesized accessors (`_x`/`$x`), `self`/`Self` symbols, generic specialization resolution (v1 exclusions per REQUIREMENTS)

## Subsequent Slice Plan

Each later phase adds one vertical slice on top of this skeleton without altering its architectural decisions:

- Phase 2: Fallback Indexer & Emission Pipeline — tree-sitter path produces a deterministic, `scip lint`-clean, degraded-signaled `.scip` from Swift sources on Linux CI
- Phase 3: Semantic Indexing on SwiftPM — the SourceKit-LSP path (validated by the Phase-1 spike verdict) delivers definitions/references/document symbols/imports/test-role marking
- Phase 4: Hierarchies & Relationships — conformance/override relationships and call/type-hierarchy edges as SCIP relationships
- Phase 5: Xcode Project Support — `--scheme` backend; one index over all targets with per-module disambiguation
- Phase 6: Release & End-to-End Validation — dual-arch artifacts + jarvis MCP answering all five structural queries on a real indexed Swift project
