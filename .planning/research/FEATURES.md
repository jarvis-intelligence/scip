# Feature Research

**Domain:** Code-intelligence indexing for Swift (SCIP indexer, `scip-swift`) — feature landscape
**Researched:** 2026-08-16
**Confidence:** HIGH (schema and index-store claims verified against primary sources: local `scip.proto`, `indexstore.h` from swiftlang/llvm-project; ecosystem claims verified against project READMEs/issues; behavioral claims from forums tagged MEDIUM)

## Context: How Swift Indexing Actually Works (Opinionated Summary)

There is **no existing scip-swift or LSIF/Kythe Swift indexer** — Swift is absent from the Sourcegraph
indexer list (scip-typescript/go/java/python/ruby/scala/clang). The Swift ecosystem instead
standardizes on one mechanism: the **Swift compiler index store** ("index-while-building"). Passing
`-index-store-path` to `swiftc`/`clang` makes the compiler emit record files (`v5/records`) containing
every symbol, occurrence role, and relation for the compiled code. Xcode generates this automatically
at `DerivedData/<Project>-<hash>/Index.noindex/DataStore`; `swift build` puts it under
`.build/<config>/index/store`.

Three consumer layers exist on top of the store:

1. **libIndexStore** — C API over the raw store (`indexstore.h` in swiftlang/llvm-project). The
   authoritative data model (verified 2026-08-16):
   - Symbol kinds: `Module, Namespace, Enum, Struct, Class, Protocol, Extension, Union, Typealias, Function, Variable, Field, EnumConstant, InstanceMethod, ClassMethod, StaticMethod, InstanceProperty, ClassProperty, StaticProperty, Constructor, Destructor, ConversionFunction, Parameter, Using, Concept`
   - Occurrence roles: `Declaration, Definition, Reference, Read, Write, Call, Dynamic, AddressOf, Implicit, Undefinition, NameReference`
   - Relations: `ChildOf, BaseOf, OverrideOf, ReceivedBy, CalledBy, ExtendedBy, AccessorOf, ContainedBy, IBTypeOf, SpecializationOf`
2. **swiftlang/indexstore-db** — LMDB-accelerated query layer over the store; what SourceKit-LSP uses.
3. **MobileNativeFoundation/swift-index-store** (ex-Lyft) — thin Swift wrapper over libIndexStore for
   one-shot CLI tools (ships `tycat`, a supertype/subtype printer — proof the store supports type
   hierarchies; SPM + Bazel; macOS and Linux).

**Opinionated call:** the "SourceKit-LSP semantics" path from PROJECT.md should be realized as
**compiler index-store consumption** (build with `-index-store-path` via SwiftPM/xcodebuild, then read
the store) — this is the same data SourceKit-LSP's own hierarchy/references features are backed by —
**not** by driving the sourcekit-lsp LSP server over textDocument requests. Reasons: LSP request-based
batch indexing is position-by-position, needs the project built anyway (hierarchy/references requests
return empty or stale without an up-to-date index), and couples us to a live server process, which is
exactly what a static offline `.scip` file exists to avoid. **Periphery** is the working CLI
precedent for this architecture (`--scheme`, `--skip-build`, `--index-store-path`).

**SCIP is Swift-ready.** Verified against local `scip.proto`: `Language.Swift = 2` exists;
`SymbolInformation.Kind` already reserves Swift-specific kinds — `Getter` (18, "For 'get' in Swift"),
`Setter` (45), `Subscript` (47), `Protocol` (42, "for Swift and Objective-C"), `ProtocolMethod` (68),
`SelfParameter` (44). The symbol grammar supports escaped identifiers (backticks — needed for
operators and `init?`), method disambiguators (needed for overloads), and `local <id>` symbols for
function-local entities. No schema changes are required.

## Feature Landscape

### Table Stakes (Users Expect These)

Missing any of these makes jarvis code navigation unusable on Swift repos; these are the v1 contract.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Definitions (goToDefinition) | The single most-used navigation query; index store `Definition` role maps 1:1 to SCIP `SymbolRole.Definition` | MEDIUM | Store gives def occurrences + USRs; the work is USR→SCIP symbol translation. Must cover types, funcs, methods, vars, enums/cases, accessors, params |
| References (findReferences) | #1 agent query for impact analysis; index store `Reference` role (+`NameReference`) → plain occurrences | MEDIUM | Must dedupe by symbol across files/modules; refine with Read/Write roles (SCIP `ReadAccess`/`WriteAccess`) which the store provides natively |
| Swift SCIP symbol scheme | Every emitted artifact depends on stable symbol identity; follow scip-python/scip-java conventions (`Package` = Swift module, e.g. manager `swiftpm`) | HIGH | Hardest table-stakes item: backtick-escaped operators, method disambiguators for overloads (`foo(+1).`), `init` as Constructor, extension members scoped under extended type (see below), `local` symbols for function-locals |
| Extensions attributed to extended type | Swift's dominant organizational pattern (`extension Foo` everywhere); if extension members get standalone symbols, find-references/type-hierarchy silently miss cross-file members | HIGH | Store already keys extension member USRs by extended module/type (`s:eC…` mangling; DocC forum confirms the design). Opinionated rule: extension members emit as `Module/ExtendedType#member` (symbol path of the *extended* type); the `extension` decl itself is a separate `Kind.Extension` symbol referencing the extended type. Verify against `ExtendedBy` relation |
| Protocol conformances → Relationships | Swift is protocol-oriented; typeHierarchy "implementations" and reference-grouping (see `Relationship` doc example in scip.proto: `Dog#sound()` ↔ `Animal#sound()`) are broken without it | MEDIUM | Conformance decl → `is_implementation` on the conforming type's symbol; protocol-requirement witnesses → `is_implementation` + `is_reference` between methods. Source: `BaseOf` relation |
| Document symbols (documentSymbols) | File overview for agents; derivable from `Definition` occurrences + `ChildOf`/`ContainedBy` relations nesting | LOW | Emit `Document.symbols` (SymbolInformation per doc); nesting encoded in symbol path; `enclosing_symbol` for locals |
| Call hierarchy (callHierarchy) | Promised v1 MCP tool; incoming/outgoing calls | MEDIUM | Source: `CalledBy` relation attached to call-site occurrences (+`ReceivedBy` for dynamic dispatch through protocols — collapse or keep, decide in spec). Outgoing calls = `Call`-role references `ContainedBy` the caller |
| Type hierarchy (typeHierarchy) | Promised v1 MCP tool; supertypes/subtypes/implementations | MEDIUM | `BaseOf` (superclass + conformances) and `OverrideOf` (class overrides) → `Relationship` entries; superclass refs also appear as plain type references |
| SwiftPM project support (`Package.swift`) | Baseline Swift project layout; `swift build` accepts `-Xswiftc -index-store-path` (or writes `.build/*/index/store` by default in recent toolchains) | LOW | Detect Package.swift → run build → read store. Cheapest entry point; snapshot fixtures live here |
| Xcode project support with `--scheme` | jarvis-index README already promises "pass --scheme if the Xcode project has several"; most real macOS apps are xcodeproj | HIGH | `xcodebuild -scheme X build` with index store auto-emitted to DerivedData `Index.noindex/DataStore`; discovery + hash-path resolution + shared-scheme requirement (Periphery hit this; schemes must be "Shared") |
| `scip lint`-clean, deterministic output | Repo's own quality bar (`scip lint`, `scip snapshot`, `scip test`); jarvis needs reproducible indexes | LOW | Deterministic ordering of documents/occurrences/symbols; canonical relative paths; UTF-8 position encoding (Swift toolchain is UTF-8 native) |
| tree-sitter degraded fallback | Decided in PROJECT.md (architecture constraint); indexes unbuildable/broken trees | MEDIUM | tree-sitter-swift (alex-pinkus → tree-sitter org) is syntax-only (maintainer-confirmed: no semantic analysis) — fallback emits defs + document symbols + intra-file refs only, clearly flagged degraded. Shares the SCIP-emitter layer with the semantic path |
| Test-code marking | Agents ask "where is this tested"; SCIP has `SymbolRole.Test` | LOW | Mark occurrences in test targets/modules (XCTest/Swift Testing targets) with Test role; store's unit metadata distinguishes test targets |
| Import statements as occurrences | `import Foo` should navigate to the module; SCIP `Import` role exists | LOW | Module symbols (`Kind.Module`) + `Import` role at the import stmt |

### Differentiators (Competitive Advantage)

These make scip-swift the best option for jarvis rather than merely adequate.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Offline static index (no live LSP) | SCIP file answers all five jarvis MCP tools without a server process; contrast SourceKit-LSP which must be running, warmed, and pointed at the right build; agents in CI/headless get instant cold-start | (free — inherent to SCIP) | This is the core strategic differentiator vs every LSP-based option |
| Index-while-building / `--skip-build` reuse | Harvest an existing DerivedData index store (developers build constantly); zero-cost fresh re-index; Periphery UX precedent (`--index-store-path`, `--skip-build`) | MEDIUM | Store is incremental by design; needs store-version detection (v5) and stale-record pruning. Default behavior: build if store missing/older than sources, else reuse |
| Cross-target/cross-module resolution in one index | One workspace index covers app + extensions + frameworks + tests, with symbols disambiguated per module — the thing Xcode's index gives only inside Xcode | LOW (falls out of the store) | Multi-target xcodeproj/SwiftPM: every target's units land in one store; SCIP `Package` per module keeps identity clean |
| macOS x86_64 + arm64 (both) | jarvis README currently promises arm64-only; shipping both exceeds the public contract | LOW | Index-store format is platform-portable; CI matrix cost is the main burden |
| Doc comments in `SymbolInformation.documentation` | High agent value (hover-equivalent context); index store has NO docs — requires overlaying the syntax tree (SwiftSyntax or tree-sitter) at definition sites | MEDIUM | Synergy: the tree-sitter/SwiftSyntax layer already exists for fallback; extraction of `///` and `/** */` docs is a contained add-on. Emit signature text via `signature_documentation` |
| External symbols stubs (`Index.external_symbols`) | goToDefinition on `String`/UIKit symbols lands on a real location (or documented stub) instead of nowhere; unique among Swift CLI indexers | HIGH | Store records references to non-workspace symbols with USRs but no source; options: synthesize SymbolInformation from USR demangling, parse `.swiftinterface`/`.h`. Recommend v1.x: demangle-only stubs with display names, no fake positions |
| Snapshot-fixture culture | Golden tests in-repo (`scip snapshot`) give users verifiable correctness claims; matches repo conventions (reprolang precedent) | LOW | Fixtures exercising: extensions, protocols, generics, operators, accessors, actors, #if, ObjC refs |
| Global module filtering (`--target`/`--module`) | Index only app module vs. including tests/dependencies; controls index size | LOW | Filter store units by module/target before emission |

### Anti-Features (Commonly Requested, Often Problematic)

Deliberately NOT building — documented to prevent scope creep.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Generic specialization resolution | "Find callers of `map` on *this* type" | `SpecializationOf` relations explode combinatorially; canonical (unspecialized) symbols are what find-references semantics require — SCIP has no notion of instantiations | Index canonical generic symbols; `TypeParameter` kind for generic params; ignore specializations |
| Full ObjC/C/C++ indexing | ObjC interop is everywhere in macOS codebases | The clang units in the store duplicate scip-clang's lane; doubles surface area; SCIP kind mapping for clang is a second indexer in disguise | Emit references to ObjC/C symbols only (external symbol stubs in v1.x); recommend scip-clang for mixed repos (jarvis: one language per index is already documented) |
| Live daemon / file-watching re-index | "Keep the index fresh" | Conflicts with the static-file model; sourcekit-lsp already occupies this niche; agent-driven workflows re-invoke `jarvis index` anyway | Fast full/reuse re-index (store reuse); freshness is jarvis's `getIndexStatus` job |
| Rename refactoring | Natural extension of references | Needs compiler-grade semantics + edit application; out of SCIP's data model; SourceKit-LSP does it live | Out of scope; jarvis agents edit files themselves |
| Custom semantic analysis on tree-sitter | Tempting to "improve" fallback accuracy | tree-sitter is syntax-only (maintainer-stated); hand-rolled scope resolution will be wrong in Swift (type-directed disambiguation, overload resolution) and erode trust | Fallback stays honestly degraded; semantics come only from the compiler store |
| `scip.proto` schema changes (e.g. `IBTypeOf`, `Dynamic` call flag) | Feels cleaner to model Swift exactly | Breaks the "schema follows upstream scip-code/scip" constraint; every consumer must re-generate | Use existing roles/relationships; encode extras in symbol naming conventions |
| Linux support in v1 | SourceKit exists on Linux | PROJECT.md out-of-scope: unreliable toolchain story; the fallback architecture keeps the door open | Defer; tree-sitter fallback documents the seam |
| Search / embeddings ("semantic" search) | Competitors (swift-index) have it | jarvis already serves searchCode/semanticSearch via Zoekt/embeddings — duplicating it in the indexer fragments the product; swift-index's approach is approximate (BM25+vector RRF), not precise — different lane | Stay precise/structural; let jarvis compose |
| Building without any build (heuristic full-project semantics) | Users with broken builds want full precision anyway | Without compiler build settings, Swift cannot be resolved (type inference, imports, conditional compilation) — any attempt is guessing | tree-sitter fallback is the answer for unbuildable trees; error message tells user to fix the build |

## Swift-Specific Semantics Cheat Sheet (What the Indexer Must Handle)

Mapping decisions per construct (opinionated, verified against store model + scip.proto):

| Swift construct | Store signal | SCIP emission |
|---|---|---|
| Module (`import`) | `Module` kind unit records | `Package{manager:"swiftpm", name:<module>}` in symbol scheme; `Kind.Module` for module symbol; `Import` role at import site |
| class / struct / enum / protocol / typealias | `Class/Struct/Enum/Protocol/Typealias` kinds | `Type` descriptor suffix (`Foo#`); Kind = Class/Struct/Enum/Protocol/TypeAlias |
| `extension Foo` | `Extension` kind + `ExtendedBy` relation | `Kind.Extension` symbol; extension *members* scoped under `Foo#` path (critical decision, see table stakes) |
| Protocol conformance / inheritance | `BaseOf` relation on conforming type | `Relationship{is_implementation: true}` to protocol symbol |
| Protocol requirement + witness | `BaseOf` + method-level override tracking | witness method `Relationship{is_implementation:true, is_reference:true}` to requirement (exact scip.proto TypeScript-example semantics) |
| Class method override | `OverrideOf` relation | `Relationship{is_reference: true}` (+ is_implementation if superclass abstract) between methods |
| func / method overloads | distinct USRs; store disambiguates | `Method` descriptor with `disambiguator` (e.g. `foo(+1).`, `foo(+2).` — scip-java convention) |
| operators (`+`, `==`, custom) | Function kind with operator name | escaped-identifier descriptor `` `+`(). ``; `Kind.Operator` |
| `init` / `init?` / deinit | `Constructor`/`Destructor` kinds | `Kind.Constructor`; escaped `` `init?` `` name; `Kind.Destructor` for deinit |
| Computed property accessors get/set, `willSet/didSet`, subscripts | `AccessorOf` relation | `Kind.Getter`/`Setter`/`Subscript` children of the property symbol (schema reserves these for Swift) |
| Property wrappers (`@Foo var x`) | synthesized `_x`, `$x` symbols | Emit user `x` only; skip synthesized (Implicit role records); document |
| Generic params (`<T>`) | `TypeParameter`-ish records | `TypeParameter` descriptor `[T]`; no specialization expansion |
| Actors, async/await | Class + method kinds (no special index treatment) | `Kind.Class` for actor (closest); async is signature text only — no role modeling |
| Access levels (private…open) | recorded in symbol properties | Does NOT change scoping unit: package == module in SCIP, so `internal`+ are normal global symbols; `private`/fileprivate file-local entities use `local <id>` symbols per the scip.proto rule (document-local only) |
| `#if` conditional compilation | only the compiled branch is in the store | One configuration per index; document limitation; fixture covers it |
| ObjC/C symbols referenced from Swift | cross-USR references into clang units | Reference occurrences with external symbol scheme (v1: optionally omit; v1.x: stub external_symbols) |
| Generated code / SwiftUI previews | source paths under `*.build/`, `PreviewProvider` conformances | `SymbolRole.Generated` on occurrences in generated files; exclude preview-provider bodies by default |
| Test targets | unit metadata marks test compilations | `SymbolRole.Test` on all occurrences in test modules |

## Feature Dependencies

```
[SCIP emitter + Swift symbol scheme]  (foundation — no deps)
    └──requires──> (nothing; pure codegen layer)

[Index-store ingestion (libIndexStore via Swift helper / FFI)]
    └──required by──> [Definitions] [References] [Document symbols]
                              [Call hierarchy] [Type hierarchy]

[Definitions + References] ──requires──> [Symbol scheme]
[Symbol scheme]             ──requires──> [Extension-attribution rule decided FIRST]
                                                        (changes every emitted symbol path)

[Type hierarchy] ──requires──> [Relationships (BaseOf/OverrideOf → is_implementation)]
[Call hierarchy] ──requires──> [References + CalledBy relation mapping]
[Protocol-conformance grouping in findReferences] ──requires──> [Relationships]

[SwiftPM support] ──requires──> [Index-store ingestion]
[Xcode --scheme support] ──requires──> [Index-store ingestion] + [xcodebuild store discovery]
[Xcode --scheme support] ──enhances──> [Cross-target resolution] (multi-target stores)

[tree-sitter fallback] ──requires──> [SCIP emitter + symbol scheme] (shared)
[tree-sitter fallback] ──conflicts──> [all relation-based features] (syntax-only: no refs/hierarchies)

[Doc comments] ──requires──> [Definitions] + [syntax overlay (SwiftSyntax/tree-sitter at def sites)]

[External symbol stubs] ──requires──> [References] + [USR demangling]
[Test role marking] ──requires──> [Index-store ingestion] (unit metadata)
[scip lint/snapshot fixtures] ──requires──> every emitted feature (validation layer)
```

### Dependency Notes

- **Symbol scheme before everything:** the extension-attribution rule determines every symbol path;
  decide and freeze it in the spec phase before any emitter code.
- **tree-sitter fallback shares only the emitter:** it can ship in parallel with semantic work but
  cannot inherit relations — keep the degraded-mode feature set explicit (defs + document symbols +
  same-file refs) so jarvis can display a capability flag.
- **Xcode support depends on store discovery quirks:** DerivedData hash paths, shared schemes, and
  `COMPILER_INDEX_STORE_ENABLE=NO` builds (Periphery issue #1082 documents the failure mode: silently
  missing store). Plan detection + actionable errors.
- **Hierarchies depend on relations, which are free once ingestion works:** the store emits
  `CalledBy`/`BaseOf`/`OverrideOf` on occurrences — hierarchy features are mapping work, not
  computation work. Do not invent graph algorithms.

## MVP Definition

### Launch With (v1)

- [ ] Swift SCIP symbol scheme (modules as packages, method disambiguators, operators, accessors,
      extension-attribution rule) — foundation for all output
- [ ] Definitions + references with read/write roles — powers goToDefinition/findReferences
- [ ] Document symbols — powers documentSymbols
- [ ] Protocol conformance + override relationships — powers typeHierarchy implementations and
      reference grouping
- [ ] Call + type hierarchy edges from store relations — powers callHierarchy/typeHierarchy
- [ ] SwiftPM project indexing (`swift build` + store) — primary fixture path
- [ ] Xcode project indexing with `--scheme` (auto-detect or Periphery-style flag) — jarvis README
      promise
- [ ] tree-sitter degraded fallback — architecture constraint from PROJECT.md
- [ ] `scip lint`-clean, deterministic, snapshot-tested output (fixtures: extensions, protocols,
      generics, operators, ObjC references, #if, previews, tests) — repo quality bar
- [ ] macOS arm64 + x86_64 binaries — exceeds the current jarvis-index promise

### Add After Validation (v1.x)

- [ ] Doc comments + signature documentation — trigger: semantic path stable, agents asking for hover context
- [ ] `--skip-build` / `--index-store-path` store reuse — trigger: users complaining about rebuild time
- [ ] External symbol stubs via USR demangling — trigger: goToDefinition misses on SDK symbols reported
- [ ] `--target`/module filtering + generated-code exclusion refinement — trigger: large-repo index size complaints
- [ ] Incremental caching keyed on store record mtimes — trigger: re-index latency > a few seconds on big repos

### Future Consideration (v2+)

- [ ] Linux support via toolchain index store or tree-sitter-only mode — deferred per PROJECT.md out-of-scope
- [ ] ObjC/C emission for mixed codebases — defer; recommend scip-clang composition
- [ ] Bazel/BSP build integration (`rules_swift` index stores) — niche; architecture permits
- [ ] scip-clang-style parallel shard+merge architecture — trigger: single-process too slow on Chromium-class repos

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Swift symbol scheme (incl. extension rule) | HIGH (invisible but foundational) | HIGH | P1 |
| Definitions / references | HIGH | MEDIUM | P1 |
| Document symbols | HIGH | LOW | P1 |
| Protocol/override relationships | HIGH | MEDIUM | P1 |
| Call + type hierarchies | HIGH | MEDIUM | P1 |
| SwiftPM indexing | HIGH | LOW | P1 |
| Xcode `--scheme` indexing | HIGH | HIGH | P1 |
| Determinism + lint-clean + snapshots | HIGH | LOW | P1 |
| tree-sitter fallback | MEDIUM | MEDIUM | P1 |
| Store reuse / `--skip-build` | MEDIUM | MEDIUM | P2 |
| Doc comments | MEDIUM | MEDIUM | P2 |
| External symbol stubs | MEDIUM | HIGH | P2 |
| Module/target filtering | MEDIUM | LOW | P2 |
| Test role marking | MEDIUM | LOW | P2 |
| Incremental caching | MEDIUM | MEDIUM | P2 |
| Parallel shard architecture | LOW | HIGH | P3 |
| ObjC/C indexing | LOW | HIGH | P3 |
| Linux | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for launch
- P2: Should have, add when possible
- P3: Nice to have, future consideration

## Competitor Feature Analysis

| Feature | SourceKit-LSP (live server) | Xcode built-in index | swift-index (MCP) | Periphery (CLI) | scip-clang (in-family) | scip-swift (our plan) |
|---------|------------------------------|----------------------|-------------------|------------------|------------------------|------------------------|
| Data source | sourcekitd + indexstore-db (background indexing default since Swift 6.1; slow — issue #1837) | private index store, Xcode-locked | SwiftSyntax + tree-sitter + ML embeddings (BM25+vector RRF) | compiler index store via xcodebuild/swift build | Clang AST, per-TU compile via compilation database | compiler index store via swift build/xcodebuild |
| goToDefinition | yes (needs build/index; .swiftinterface quirks — issue #2693) | yes (in Xcode only) | no (approximate search only) | no (unused-code lane) | yes, precise | yes, precise, offline |
| findReferences | yes | yes (in Xcode) | no (ranked guesses) | partial (for its analysis) | yes | yes + protocol-grouping via relationships |
| call/type hierarchy | yes (callHierarchy + typeHierarchy since ~Swift 5.9/6; needs built index) | yes | no | no (tycat-style: no) | n/a (C++ only) | yes, from CalledBy/BaseOf/OverrideOf |
| Document symbols | yes | yes | parse_tree (AST dump) | no | yes | yes |
| Xcode projects | partial — BSP bridges like Cadmus needed (issue #730 open) | native | n/a | yes (`--scheme`, shared schemes) | n/a (comp db) | yes (`--scheme`, Periphery UX) |
| Offline static artifact | no (server must run) | no (DerivedData + Xcode) | SQLite/USearch DB | no output index | `.scip` file | `.scip` file — the jarvis-native artifact |
| ObjC interop | yes (clangd) | yes | tree-sitter fallback | yes (store has clang units) | is the C indexer | references-only (v1), stubs (v1.x) |
| Deterministic/testable | weak (server state) | no | no | n/a | snapshot-tested | `scip lint` + `scip snapshot` golden fixtures |

**Strategic read:** the precise-navigation lane for Swift is occupied only by live-editor tooling
(SourceKit-LSP/Xcode); nothing produces a portable, offline, precise index — scip-swift's niche is
exactly that, and it is what jarvis's MCP tools are designed to consume.

## Sources

- Local schema (primary): `/Users/ddphuong/Projects/jarvis-ai/scip/scip.proto` — symbol grammar, SymbolRole, Relationship, Swift-specific Kind values, Language.Swift
- Index store data model (primary): [indexstore.h, swiftlang/llvm-project `next` branch](https://raw.githubusercontent.com/swiftlang/llvm-project/next/clang/include/indexstore/indexstore.h)
- [swiftlang/indexstore-db](https://github.com/swiftlang/indexstore-db) — LMDB query layer, `-index-store-path` provenance; [Index Store.md doc](https://github.com/swiftlang/indexstore-db/blob/main/Sources/IndexStore/Index%20Store.md) (v5/records)
- [MobileNativeFoundation/swift-index-store](https://github.com/MobileNativeFoundation/swift-index-store) — libIndexStore wrapper, tycat type-hierarchy tool, SPM/Bazel, macOS+Linux
- [swiftlang/sourcekit-lsp](https://github.com/swiftlang/sourcekit-lsp) — features incl. callHierarchy/typeHierarchy; [background indexing slowness #1837](https://github.com/swiftlang/sourcekit-lsp/issues/1837); [Xcode project support #730](https://github.com/swiftlang/sourcekit-lsp/issues/730); [.swiftinterface definition issue #2693](https://github.com/swiftlang/sourcekit-lsp/issues/2693)
- [Background indexing default since Swift 6.1 — Peter Kos](https://peterkos.me/posts/a-swift-kick-in-the-lsp/); [hierarchies require a build — Swift forums](https://forums.swift.org/t/sourcekit-lsp-and-the-typehierarchy-family-of-requests/77058); [pre-generating index-db — Swift forums](https://forums.swift.org/t/sourcekit-lsp-best-way-to-pre-generate-index-db-for-sourcekit-lsp/80870)
- [peripheryapp/periphery](https://github.com/peripheryapp/periphery) — CLI UX precedent (`--scheme`, `--skip-build`, `--index-store-path`, shared schemes); [issue #1082 store discovery](https://github.com/peripheryapp/periphery/issues/1082)
- [alexey1312/swift-index](https://github.com/alexey1312/swift-index) — MCP semantic-search competitor (approximate, not precise)
- [alex-pinkus/tree-sitter-swift](https://github.com/alex-pinkus/tree-sitter-swift); [tree-sitter is syntax-only — tree-sitter#1631](https://github.com/tree-sitter/tree-sitter/issues/1631)
- [sourcegraph/scip-clang](https://github.com/sourcegraph/scip-clang) + [Design.md](https://sourcegraph.com/r/github.com/sourcegraph/scip-clang/-/blob/docs/Design.md) — in-family compile-driven indexer precedent (driver/worker shards)
- [sourcegraph/scip-python](https://github.com/sourcegraph/scip-python) — package-manager-based symbol conventions (pip versions)
- [scip-code.org](https://scip-code.org/); [LSP 3.17 spec](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/) — hierarchy request shapes mirrored by jarvis MCP tools
- [jarvis-intelligence/jarvis-index](https://github.com/jarvis-intelligence/jarvis-index) — consumer contract: 9 MCP tools, scip-swift + `--scheme` promise, one language per index
- [Symbol-graph USR discussion for extensions — Swift forums](https://forums.swift.org/t/symbol-graph-adaptions-for-documenting-extensions-to-external-types-in-docc/56684); [Swift ABI mangling libraries — Swift forums](https://forums.swift.org/t/any-libraries-for-working-with-swift-abi-manglings/72228)
- [Cadmus BSP bridge for Xcode ↔ sourcekit-lsp](https://open-v-vs.org/extension/cadmus/cadmus); [LLVM D39050 — original index-while-building patch](https://reviews.llvm.org/D39050)

---
*Feature research for: Swift code-intelligence indexing (scip-swift)*
*Researched: 2026-08-16*
