# scip-swift

## What This Is

This repo is a fork of the SCIP ("skip") protocol monorepo — the Protobuf code-intelligence
schema, its Go CLI and bindings, and five generated language bindings — extended with
**scip-swift**, a new in-repo Swift indexer that emits SCIP indexes. It exists so that
jarvis (a local-first code-intelligence MCP server, distributed via
jarvis-intelligence/jarvis-index) can answer structural queries — goToDefinition,
findReferences, callHierarchy, typeHierarchy, documentSymbols — on Swift codebases.

## Core Value

scip-swift produces accurate, `scip lint`-clean SCIP indexes for real Swift projects,
powering working code navigation in jarvis on macOS.

## Business Context

- **Customer**: jarvis users — macOS developers using AI coding agents via MCP
- **Revenue model**: N/A — internal capability for the jarvis product line
- **Success metric**: jarvis MCP tools return correct definitions/references/hierarchies
  on a real Swift project indexed by scip-swift
- **Strategy notes**: jarvis-index README already promises Swift support "via the
  scip-swift indexer (macOS arm64 only, Xcode scheme flag option)" — this project is
  that indexer

## Requirements

### Validated

<!-- Inferred from existing code (brownfield init, 2026-08-16). -->

- ✓ SCIP Protobuf schema (Index/Document/Occurrence/Symbol model) — `scip.proto` — existing
- ✓ Go CLI with `lint`, `print`, `snapshot`, `stats`, `test`, `expt-convert` subcommands — `cmd/scip/` — existing
- ✓ Published Go bindings library (symbol parse/format, canonicalization, streaming parse) — `bindings/go/scip/` — existing
- ✓ Generated bindings for TypeScript, Rust, Haskell, Java, Kotlin via Buf codegen — `bindings/` — existing
- ✓ reprolang tree-sitter reference indexer exercising the CLI end-to-end — `reprolang/` — existing
- ✓ Reproducible Nix build + CI/release pipelines — `flake.nix`, `.github/workflows/` — existing

### Active

<!-- Milestone v1.0 — Add Swift support. Hypotheses until shipped and validated. -->

- [ ] scip-swift indexes a Swift project and emits a valid `.scip` index that passes `scip lint`
- [ ] Definitions and references are emitted (powers goToDefinition / findReferences)
- [ ] Document symbols are emitted (powers documentSymbols)
- [ ] Call hierarchies and type hierarchies are emitted (powers callHierarchy / typeHierarchy)
- [ ] SourceKit-LSP semantic indexing path works on macOS arm64 and x86_64
- [ ] tree-sitter fallback path (degraded, syntax-level indexing) exists and is exercised in tests
- [ ] Snapshot test fixtures cover representative Swift code; golden tests pass
- [ ] Verified end-to-end: jarvis MCP answers structural queries on a real Swift project indexed by scip-swift

### Out of Scope

- Linux support in v1 — SourceKit-LSP on Linux is unreliable; the tree-sitter fallback
  architecture keeps the door open for a later milestone
- Lexical/semantic search (searchCode, semanticSearch) — served by Zoekt/embeddings inside
  jarvis, not by SCIP indexing
- Indexers for other languages — existing ecosystem indexers (scip-java, scip-python, …) cover those
- `scip.proto` schema changes in v1 — Swift data must fit the existing protocol; schema
  evolution follows upstream scip-code/scip

## Context

- Working directory is a fork of scip-code/scip under `~/Projects/jarvis-ai/`; the consumer
  is the private jarvis MCP server, publicly distributed via
  [jarvis-intelligence/jarvis-index](https://github.com/jarvis-intelligence/jarvis-index)
  (SCIP + Zoekt based; nine MCP tools; three agent skills).
- The repo already demonstrates both patterns this project needs: `reprolang/` shows a
  tree-sitter indexer as an in-repo Go module with snapshot testdata; `cmd/scip/` +
  `bindings/go/scip/testutil/` show how index quality is validated (`scip lint`,
  `scip snapshot`, `scip test`).
- Codebase map: `.planning/codebase/` (STACK, ARCHITECTURE, STRUCTURE, CONVENTIONS,
  TESTING, INTEGRATIONS, CONCERNS) — analysis date 2026-08-16.
- Known pre-existing bugs are catalogued in `.planning/codebase/CONCERNS.md` (e.g.
  `scip.ParseSymbol` panic on trailing multi-byte runes in
  `bindings/go/scip/symbol_parser.go`; `scip stats` LOC never counted when project root
  exists in `cmd/scip/stats.go`). Unrelated to this milestone, but relevant when touching
  shared code.

## Constraints

- **Tech stack**: Swift + SourceKit-LSP for semantics; tree-sitter (Swift grammar) for
  fallback — hybrid decided during questioning
- **Platform**: macOS arm64 + x86_64 for v1, full semantics on both
- **Integration**: Emitted index must pass `scip lint` and follow SCIP symbol-format
  conventions documented in `scip.proto`
- **Environment**: The repo's canonical toolchain is Nix (`nix develop`, flake checks);
  Swift toolchain integration must fit the flake/check matrix or justify a CI exception
- **Compatibility**: Must not break existing modules (Go workspace, bindings codegen,
  reprolang tests)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Build scip-swift in this repo as a new module (reprolang precedent) | Keeps the indexer beside the protocol and validation CLI; the in-repo pattern already exists | — Pending |
| Hybrid semantics: SourceKit-LSP primary, tree-sitter fallback | Accurate defs/refs/hierarchies need compiler semantics; tree-sitter gives degraded-but-working indexing without a full Xcode build | — Pending |
| macOS arm64 + x86_64 only in v1 | Matches the jarvis audience; SourceKit is reliable on macOS | — Pending |
| Done = `scip lint` + snapshot fixtures + jarvis e2e | Ecosystem-standard validation plus real-consumer proof | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `$gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `$gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-08-16 after initialization*
