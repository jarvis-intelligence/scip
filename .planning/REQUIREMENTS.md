# Requirements: scip-swift

**Defined:** 2026-08-16
**Core Value:** scip-swift produces accurate, `scip lint`-clean SCIP indexes for real Swift projects, powering working code navigation in jarvis on macOS.

## v1 Requirements

Requirements for milestone v1.0 (Add Swift support). Each maps to roadmap phases.

### Symbol & Index Foundation

- [ ] **SYM-01**: Every indexed Swift symbol has a stable SCIP symbol string — modules as `swiftpm` packages, overload disambiguators, backtick-escaped operators, `init`/accessor kinds — that round-trips through `bindings/go/scip` symbol parse/format
- [ ] **SYM-02**: Extension members are attributed to the extended type's symbol path (cross-file `extension Foo` members land under `Foo#`), so findReferences and typeHierarchy never miss them
- [ ] **SYM-03**: Index output is deterministic (canonical ordering, relative paths, UTF-8 positions) and passes `scip lint` with zero errors
- [ ] **SYM-04**: `import Foo` statements emit occurrences with the `Import` role resolving to the module's symbol

### Navigation

- [ ] **NAV-01**: goToDefinition and findReferences work precisely for types, funcs, methods, vars, enum cases, accessors, and params — with read/write roles distinguishing accesses
- [ ] **NAV-02**: documentSymbols returns a correct per-file outline (types, members, nesting via symbol paths; locals via `enclosing_symbol`)
- [ ] **NAV-03**: Occurrences in test targets are marked with `SymbolRole.Test` so agents can locate tests for a symbol

### Hierarchies & Relationships

- [ ] **REL-01**: Protocol conformances emit `is_implementation` relationships (type-level and requirement-witness-level) and class overrides emit override relationships — powering typeHierarchy "implementations" and reference grouping
- [ ] **REL-02**: callHierarchy answers incoming and outgoing calls for indexed functions/methods
- [ ] **REL-03**: typeHierarchy answers supertypes, subtypes, and protocol implementations for indexed types

### Project Support

- [ ] **PROJ-01**: Indexing a SwiftPM project (`Package.swift`) via build + semantic harvest works end-to-end — the primary fixture path
- [ ] **PROJ-02**: Indexing an Xcode project via `--scheme` (Periphery-style flag, shared-scheme discovery, actionable errors when the index store is missing) works — the jarvis README promise
- [ ] **PROJ-03**: One index covers all targets/modules of a project (app + extensions + frameworks + tests) with per-module symbol disambiguation

### Fallback & Quality

- [ ] **FBQ-01**: When the semantic path is unavailable (no toolchain/build), the tree-sitter fallback indexes definitions, document symbols, and intra-file references — explicitly flagged as degraded, never silently merged with semantic output
- [ ] **FBQ-02**: Golden snapshot fixtures cover extensions, protocols, generics, operators, accessors, `#if` blocks, and test targets, and pass `scip snapshot` on CI
- [ ] **FBQ-03**: Release artifacts ship for macOS arm64 and x86_64 (static tarballs, version-synced to `cmd/scip/version.txt`) with a dual-arch CI matrix
- [ ] **FBQ-04**: End-to-end validation: jarvis MCP tools (goToDefinition, findReferences, callHierarchy, typeHierarchy, documentSymbols) answer correctly on a real Swift project indexed by scip-swift

## v2 Requirements

Deferred until v1 is validated. Tracked but not in current roadmap.

### Documentation & Reuse

- **DOC-01**: Doc comments (`///`, `/** */`) and signatures attached to definitions via `SymbolInformation.documentation`
- **REUSE-01**: `--skip-build` / `--index-store-path` reuse of existing index stores (zero-cost re-index after developer builds)
- **EXT-01**: External symbol stubs via USR demangling so goToDefinition on SDK symbols (`String`, UIKit) lands somewhere useful
- **FILT-01**: `--target`/module filtering and generated-code exclusion refinement for large repos
- **CACHE-01**: Incremental re-indexing keyed on store record mtimes

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Generic specialization resolution | Combinatorial explosion; findReferences semantics require canonical unspecialized symbols — SCIP has no instantiation model |
| Full ObjC/C/C++ indexing | scip-clang's lane; references-only from Swift side (stubs in v1.x) |
| Live daemon / file-watching re-index | Conflicts with the static `.scip` artifact model; freshness is jarvis's `getIndexStatus` job |
| Rename refactoring | Compiler-grade edit application outside SCIP's data model; agents edit files themselves |
| Custom semantic analysis on tree-sitter | tree-sitter is syntax-only (maintainer-confirmed); hand-rolled Swift scope resolution would be wrong and erode trust |
| `scip.proto` schema changes | Schema follows upstream scip-code/scip; Swift data fits existing roles/kinds (`Language.Swift = 2` verified) |
| Linux support | Unreliable SourceKit/index-store toolchain story on Linux; tree-sitter fallback architecture keeps the door open for v2+ |
| Search / embeddings ("semantic" search) | jarvis serves searchCode/semanticSearch via Zoekt/embeddings — different lane; scip-swift stays precise/structural |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SYM-01 | Phase 1: Symbol Scheme & Module Foundations | Pending |
| SYM-02 | Phase 1: Symbol Scheme & Module Foundations | Pending |
| SYM-03 | Phase 2: Fallback Indexer & Emission Pipeline | Pending |
| SYM-04 | Phase 3: Semantic Indexing on SwiftPM | Pending |
| NAV-01 | Phase 3: Semantic Indexing on SwiftPM | Pending |
| NAV-02 | Phase 3: Semantic Indexing on SwiftPM | Pending |
| NAV-03 | Phase 3: Semantic Indexing on SwiftPM | Pending |
| REL-01 | Phase 4: Hierarchies & Relationships | Pending |
| REL-02 | Phase 4: Hierarchies & Relationships | Pending |
| REL-03 | Phase 4: Hierarchies & Relationships | Pending |
| PROJ-01 | Phase 3: Semantic Indexing on SwiftPM | Pending |
| PROJ-02 | Phase 5: Xcode Project Support | Pending |
| PROJ-03 | Phase 5: Xcode Project Support | Pending |
| FBQ-01 | Phase 2: Fallback Indexer & Emission Pipeline | Pending |
| FBQ-02 | Phase 2: Fallback Indexer & Emission Pipeline | Pending |
| FBQ-03 | Phase 6: Release & End-to-End Validation | Pending |
| FBQ-04 | Phase 6: Release & End-to-End Validation | Pending |

**Coverage:**
- v1 requirements: 17 total
- Mapped to phases: 17
- Unmapped: 0

---
*Requirements defined: 2026-08-16*
*Last updated: 2026-08-16 after roadmap creation (traceability filled, 17/17 mapped)*
