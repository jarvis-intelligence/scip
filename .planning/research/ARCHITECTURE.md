# Architecture Research

**Domain:** Hybrid SCIP code indexer for Swift (SourceKit-LSP primary + tree-sitter fallback), integrated into the scip monorepo
**Researched:** 2026-08-16
**Confidence:** HIGH for repo-internal facts (read from source) and SourceKit-LSP extension surface (official contributor docs); MEDIUM for SourceKit-LSP behavioral details (position-encoding negotiation, background-indexing defaults) and xcodebuild backend mechanics; these are flagged inline.

---

## Standard Architecture

### System Overview

scip-swift is a **Go-driven indexer that treats SourceKit-LSP as a semantic oracle subprocess and tree-sitter as a syntax-level fallback**. The opinionated core decision: the indexer is a new Go module in this repo (reprolang precedent), not a Swift program. Rationale:

- The repo's entire value chain for index quality — `bindings/go/scip/` emission helpers, `bindings/go/scip/testutil/` golden snapshots, `scip lint`/`scip snapshot`/`scip test` validation — is Go. A Swift-native driver would re-implement all of it (or bind via FFI) and still spawn SourceKit-LSP as a process for the sanctioned API surface.
- Semantic fidelity comes from SourceKit-LSP over stdio JSON-RPC; the driver language is irrelevant to accuracy.
- jarvis distributes indexers as standalone binaries (`setup.sh` → `~/.jarvis/bin`); Go cross-compiles to a static binary for arm64 + x86_64 trivially.
- Linking sourcekitd directly from a Swift driver would be private-API territory; LSP + `textDocument/symbolInfo` is the documented, stable surface (see Pattern 2).

```
┌───────────────────────────────────────────────────────────────────────────┐
│ NEW MODULE: swift/  (github.com/scip-code/scip/swift, Go module #4)       │
│                                                                           │
│  CLI: cmd/scip-swift  ── flags: <project> [--scheme S] [--output O]       │
│         │                                                                │
│  ┌──────▼──────────┐   Package.swift? → swiftpm backend                  │
│  │ backend/        │   *.xcodeproj/xcworkspace? → xcodebuild backend     │
│  │ (detect+prime)  │   neither / no toolchain → FALLBACK                 │
│  └──────┬──────────┘                                                     │
│         │ primes build (swift build | xcodebuild -scheme)                │
│  ┌──────▼──────────┐  spawn `xcrun sourcekit-lsp`   ┌─────────────────┐  │
│  │ semantic/       │◄── stdio JSON-RPC ────────────►│ SourceKit-LSP   │  │
│  │  ├ lsp/         │  initialize / symbolInfo /     │ (subprocess,    │  │
│  │  ├ symbol/      │  definition / references /     │  Xcode-bundled) │  │
│  │  └ emit/        │  documentSymbol / hierarchies  └─────────────────┘  │
│  └──────┬──────────┘                                                     │
│  ┌──────▼──────────┐  parse .swift (cgo)      ┌────────────────────────┐ │
│  │ fallback/       │◄─────────────────────────│ grammar/ (tree-sitter- │ │
│  │ (degraded path) │                          │ swift parser.c vendor) │ │
│  └──────┬──────────┘                          └────────────────────────┘ │
│  ┌──────▼──────────┐  USR→SCIP symbol strings, UTF-16→UTF-8 positions,   │
│  │ emit/ assembly  │  dedup + canonicalize + sort → *scip.Index          │
│  └──────┬──────────┘                                                     │
└─────────┼─────────────────────────────────────────────────────────────────┘
          │ writes index.scip (protobuf)
┌─────────▼─────────────────────────────────────────────────────────────────┐
│ REUSED, UNCHANGED: bindings/go/scip (emission+canonicalize) ·             │
│ bindings/go/scip/testutil (snapshot goldens) · cmd/scip (lint/snapshot    │
│ validation gates, run as subprocess)                                     │
└─────────┬─────────────────────────────────────────────────────────────────┘
          │ .scip file
┌─────────▼─────────────────────────────────────────────────────────────────┐
│ CONSUMER: jarvis (SCIP + Zoekt MCP server) — goToDefinition,             │
│ findReferences, callHierarchy, typeHierarchy, documentSymbols            │
└───────────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| CLI (`swift/cmd/scip-swift`) | Argument surface matching the jarvis contract (project path, `--scheme`, `--output`, `--fallback`); exit codes | Single `main.go`, thin wiring over `internal/` packages |
| Backend detection (`internal/backend`) | Discover build graph: SwiftPM vs xcodeproj vs bare directory; prime the build so SourceKit-LSP has an up-to-date index | Filesystem sniffing + subprocess (`swift build`, `xcodebuild`) |
| LSP client (`internal/lsp`) | Spawn/manage SourceKit-LSP; initialize handshake; capability + position-encoding negotiation; typed wrappers for the requests we consume | `os/exec` + JSON-RPC over stdio (Content-Length framing) |
| Ready gating (`internal/lsp/progress.go`) | Wait until background indexing/build settles before querying | Poll `sourcekit/isIndexing`; absorb `window/logMessage` |
| Symbol mapper (`internal/symbol`) | Stable SCIP symbol strings: USR → descriptors, module attribution, overload disambiguators, identifier escaping | Pure functions; property-tested against `scip.ParseSymbol` round-trip |
| Semantic orchestrator (`internal/semantic`) | Walk documents; extract definitions, references, doc symbols, hierarchy edges; produce raw records | Per-file loop with bounded concurrent LSP requests |
| Fallback indexer (`internal/fallback`) | Degraded syntax-level indexing: document symbols + heuristic intra-file defs/refs when no toolchain or non-buildable project | tree-sitter AST walk (cgo) |
| Grammar (`swift/grammar`) | tree-sitter-swift parser vendored + cgo binding | Generated `parser.c` + hand-written `binding.go` (reprolang precedent) |
| Position translation (`internal/emit/positions.go`) | LSP UTF-16 (or negotiated) code-unit offsets → UTF-8 byte offsets for SCIP ranges | Line-indexed source text + conversion tables |
| Index assembly (`internal/emit`) | Records → `*scip.Index`: dedup occurrences, emit `SymbolInformation` for every referenced symbol, set `position_encoding`, canonicalize, sort | `bindings/go/scip` helpers (`Canonicalize*`, `Sort*`, ` Flatten`) |
| Validation harness | Prove `scip lint`-clean and stable golden output | `testutil.SnapshotTest` + subprocess `scip lint`/`scip snapshot` (see Integration Points) |

## Recommended Project Structure

Follows the reprolang precedent (own Go module at repo root, `replace` directive to bindings, cgo grammar, testdata snapshots) — verified against `/Users/ddphuong/Projects/jarvis-ai/scip/go.work` and `reprolang/go.mod`.

```
swift/                                  # NEW Go module: github.com/scip-code/scip/swift
├── go.mod                              # replace github.com/scip-code/scip/bindings/go/scip => ../bindings/go/scip
├── go.sum
├── README.md                           # usage, toolchain prerequisites (Xcode / swift toolchain)
├── cmd/
│   └── scip-swift/
│       └── main.go                     # CLI entry; jarvis contract: scip-swift <project> [--scheme S] [--output O] [--fallback]
├── internal/
│   ├── backend/
│   │   ├── detect.go                   # BuildBackendDetector: Package.swift → SwiftPM; *.xcodeproj/*.xcworkspace → xcodebuild; else fallback
│   │   ├── swiftpm.go                  # `swift build` warm-up; parse .build/layout for module names
│   │   └── xcode.go                    # `xcodebuild -scheme S` priming; compile-args extraction (build settings or synthesized compile_commands.json)
│   ├── lsp/
│   │   ├── client.go                   # process spawn (xcrun sourcekit-lsp), stdio JSON-RPC, request/response correlation, shutdown handling
│   │   ├── initialize.go               # initialize handshake: rootUri, capabilities; read negotiated positionEncoding
│   │   ├── query.go                    # textDocument/symbolInfo, definition, references, documentSymbol, prepareCallHierarchy/incoming/outgoing, prepareTypeHierarchy/super/subtypes, sourcekit/workspace/symbolNames
│   │   └── progress.go                 # sourcekit/isIndexing polling; readiness gating
│   ├── symbol/
│   │   ├── mapper.go                   # USR + container chain → scip.Symbol (scheme 'scip', manager 'swift', name=<module>, version='.')
│   │   ├── descriptor.go               # decl kind → Descriptor.Suffix; overload disambiguator from USR
│   │   └── escape.go                   # scip.proto identifier charset + backtick escaping
│   ├── emit/
│   │   ├── records.go                  # intermediate Record type (path, range, symbol, roles, kind, doc) shared by both paths
│   │   ├── positions.go                # UTF-16/negotiated → UTF-8 byte offsets; line tables
│   │   └── index.go                    # records → *scip.Index; completeness (SymbolInformation per symbol), Canonicalize + Sort
│   ├── semantic/
│   │   └── runner.go                   # orchestrates backend + lsp + symbol + emit; per-file error capture → per-file fallback
│   └── fallback/
│       ├── indexer.go                  # tree-sitter walk → records (doc symbols, defs, intra-file refs)
│       ├── tags.go                     # node-type → SyntaxKind / SymbolKind mapping table
│       └── symbols.go                  # synthetic symbol strings for fallback (module-scoped, heuristic nesting)
├── grammar/                            # vendored tree-sitter-swift (mirrors reprolang/grammar/)
│   ├── binding.go                      # cgo binding, hand-written (no official Go binding for the Swift grammar)
│   ├── parser.c, grammar.json, node-types.json   # generated, committed
│   └── tree_sitter/
└── testdata/
    ├── snapshots/
    │   ├── input/                      # per-case directories of .swift fixtures (semantic + fallback cases)
    │   └── output/                     # golden caret-annotated snapshots (via testutil)
    └── projects/                       # miniature SwiftPM + xcodeproj projects for e2e (git-ignored build artifacts)
```

### Structure Rationale

- **`swift/` as module root with own `go.mod`:** mirrors `reprolang/` exactly (module `github.com/scip-code/scip/reprolang` with `replace` — same pattern, verified). Add `swift` to `go.work` `use (...)`. Keeps the Go bindings module (`bindings/go/scip`) untouched — its API is public and must stay stable.
- **`internal/`:** reprolang uses a flat `repro/` package because it is tiny; scip-swift has six distinct concerns. `internal/` prevents accidental external import, which matters in a repo whose bindings library has external consumers (src-cli).
- **`cmd/scip-swift/`:** jarvis installs a binary named `scip-swift` into `~/.jarvis/bin` (jarvis-index README); reprolang has no binary, so this is a deliberate extension of the pattern.
- **`testdata/snapshots/{input,output}`:** exact layout `bindings/go/scip/testutil.SnapshotTest` expects (verified in `bindings/go/scip/testutil/snapshot_testing.go` — it hardcodes `snapshots/input` + `snapshots/output` under the passed base dir). `testdata/projects/` is separate because e2e needs buildable projects, not bare source files.
- **Grammar vendored, not fetched:** reprolang commits `grammar/parser.c` and generates via `generate-tree-sitter-parser.sh`; do the same for tree-sitter-swift (npm `tree-sitter-cli` generation step) so Nix builds stay offline-reproducible.

## Architectural Patterns

### Pattern 1: Hybrid dual-path indexer with per-file degradation

**What:** One CLI, two indexing engines. Semantic path (SourceKit-LSP) is primary; tree-sitter path activates (a) globally when no Swift/Xcode toolchain exists or the project is non-buildable, or (b) per-file when semantic queries fail for a specific file. Both paths emit the same intermediate `Record` type consumed by the same assembly layer.

**When to use:** Always — this is the milestone's defining decision (PROJECT.md Key Decisions).

**Trade-offs:** Duplicated extraction logic vs. guaranteed usefulness on every macOS machine (including CI without full Xcode, or broken-package states). Downside: symbol identity from the fallback path is heuristic (module-scoped synthetic symbols), so cross-file navigation is degraded — acceptable and documented via `ToolInfo.arguments` recording the mode used.

**Rules:**
- Global fallback: no `xcrun sourcekit-lsp`, backend detection finds nothing buildable, or LSP `initialize` fails → index everything with tree-sitter, exit 0 with a warning.
- Per-file fallback: a file whose semantic queries errored (missing from build graph, e.g. new file not yet compiled) is re-indexed via tree-sitter and merged. Never let one file fail the run.
- `--fallback` flag forces the syntax path (used by tests and by jarvis on constrained hosts).

### Pattern 2: SourceKit-LSP as a semantic oracle (LSP client, not a compiler plugin)

**What:** Spawn `xcrun sourcekit-lsp` as a child process; speak LSP 3.17 over stdio. Consume:

- Standard requests: `initialize`, `textDocument/definition`, `textDocument/references`, `textDocument/documentSymbol`, `textDocument/prepareCallHierarchy` + `callHierarchy/incomingCalls`/`outgoingCalls`, `textDocument/prepareTypeHierarchy` + `typeHierarchy/supertypes`/`subtypes`.
- SourceKit-LSP extensions (documented in `Contributor Documentation/LSP Extensions.md`, swiftlang/sourcekit-lsp — HIGH confidence):
  - `textDocument/symbolInfo` → `SymbolDetails[]` with `usr`, `bestLocalDeclaration`, `kind`, `containerName`, `isDynamic`, `receiverUsrs`, `systemModule.moduleName` — explicitly designed "to allow one LSP backend to query another LSP backend for information about symbols… not primarily designed to be sent from editors to the LSP server." This is the sanctioned machine-to-machine surface; it is exactly our use case.
  - `sourcekit/workspace/symbolNames` → flat deduplicated list of all indexed symbol names (drives work enumeration).
  - `sourcekit/isIndexing` → poll until background indexing settles before querying.
  - `sourcekit/workspace/triggerReindex` → recovery path if results look stale.

**When to use:** the semantic path, always.

**Trade-offs:** process orchestration complexity and request-count cost vs. zero private-API risk and automatic improvement with every Xcode release. The alternative (link sourcekitd or use indexstore directly, as Xcode does) was rejected: undocumented, version-fragile, and requires a Swift driver.

**Example (Go sketch of the identity query):**
```go
// per definition/reference site returned by definition/references/documentSymbol:
details, err := lsp.SymbolInfo(ctx, docURI, lspPos) // textDocument/symbolInfo
// details[0].usr        e.g. "s:6MyMod5ShapeP4area12CoreFoundation9CGFloatVvp"
// details[0].systemModule?.moduleName  e.g. "Swift" for stdlib symbols
sym := symbol.FromUSR(module, containerChain, details[0])
```

### Pattern 3: USR-anchored symbol identity mapped to the SCIP symbol grammar

**What:** Swift has overloads, extensions across files, and protocol witnesses — names alone are not identity. SourceKit USRs (unified symbol representations, e.g. `s:6MyMod...`) already disambiguate all of this. Map to the scip.proto symbol grammar:

```
scip swift <module-name> <module-version> <descriptors...>
```

- Package: `manager = "swift"`, `name = <module>` (from `symbolInfo.systemModule.moduleName`, else the owning target from the build backend), `version = "."` (SwiftPM has no per-module version; use package version when known, `"."` placeholder otherwise — grammar allows `.` for empty).
- Descriptors per the `scip.proto` grammar (verified lines 150–205): nested types → `Outer#Inner#`; free functions/properties/terms → `name.`; methods → `name().` normally, `name<disambiguator>().` only when the module+container has multiple same-name methods (derive disambiguator deterministically from the USR tail, which already encodes parameter types); protocol conformances implemented via extension attach under the type they extend (`Extension#Foo#bar().`) so that references from both the protocol and the type resolve to one symbol; parameters/locals → `Parameter`/`Local` descriptors or `local <id>` for function-local entities (locals MUST stay document-scoped per scip.proto).
- Escaping: Swift identifiers allow characters outside the scip `<identifier-character>` set (`_ + - $` alnum) — e.g. backticked keywords, operators (`+`), `$0`. Escape with backticks per `<escaped-identifier>` rules (`bindings/go/scip/identifier.go` is the reference).
- Cross-path consistency: the fallback path cannot know USRs; it emits the same descriptor scheme with a `Meta` marker? No — opinionated call: fallback emits plain `name-based` symbols with no disambiguators and never merges with semantic output. The two paths substitute, never merge (merging would create duplicate/ambiguous identities that break `scip lint` completeness checks).

**When to use:** always; this is the module's hardest correctness problem.

**Trade-offs:** USR parsing is semi-documented (MEDIUM confidence on USR grammar stability across Xcode versions); mitigate by treating USRs as opaque identity keys and extracting only the minimal pieces (module index prefix, mangled tail for disambiguation) with a versioned mapper and golden tests per Xcode major.

### Pattern 4: Position normalization pipeline (UTF-16 in → UTF-8 out)

**What:** LSP positions are UTF-16 code units by default (LSP 3.17 spec; UTF-16 is the guaranteed common denominator). SCIP recommends Go indexers emit `UTF8CodeUnitOffsetFromLineStart` (scip.proto, `Document.position_encoding` comment: "For an indexer implemented in Go, Rust or C++, use UTF8ByteOffsetFromLineStart"). Therefore every LSP range passes through a translation layer before emission:

```
LSP Position{line, character:UTF-16 units}
    → load line start offsets for that file (byte-indexed)
    → decode UTF-16 units → byte offset within the line (surrogate pairs cost 4 bytes)
    → SCIP SingleLineRange / MultiLineRange with UTF-8 byte character offsets
    → Document.position_encoding = UTF8CodeUnitOffsetFromLineStart
```

**When to use:** every occurrence, both directions (queries out, results in). During `initialize`, advertise `general.positionEncodings: ["utf-8", "utf-16"]` — if the server's InitializeResult picks utf-8 (SourceKit-LSP support for this is unverified — MEDIUM confidence; do not rely on it), translation becomes a no-op but the layer stays for correctness. Default assumption: UTF-16.

**Trade-offs:** always translating costs a line-table build per file (cheap, `O(file)`) vs. emitting UTF-16 positions, which would work (SCIP has `UTF16CodeUnitOffsetFromLineStart`) but contradicts the protocol's per-implementation-language guidance and would make tree-sitter fallback (byte-native) inconsistent with the semantic path. Uniform UTF-8 wins.

**Also set:** `Metadata.text_document_encoding = UTF8` (Swift sources are UTF-8; unrelated to range encoding) and `Document.language = "Swift"` (enum `Language.Sharp = 2` exists in scip.proto — verified). Use the typed `SingleLineRange`/`MultiLineRange` fields, never the deprecated `range` int32 array (scip.proto Occurrence comment: new producers SHOULD set typed ranges).

### Pattern 5: Validation gates as subprocesses (no library extraction from cmd/scip)

**What:** "lint-clean" is enforced by running the existing `scip` CLI as a subprocess in tests (`go run ./cmd/scip lint out.scip` or a built binary), not by importing `cmd/scip` — it is `package main` and unimportable (verified: `cmd/scip/lint.go` lives in package main). reprolang already does exactly this in `reprolang/repro/test_cmd_test.go` with `testdata/test_cmd/`.

**When to use:** CI and snapshot tests.

**Trade-offs:** subprocess startup cost in tests vs. respecting the repo's module boundaries. Alternative — extracting lint into the bindings module — touches a published module's API surface for zero consumer value; rejected.

## Data Flow

### Indexing pipeline (primary path)

```
scip-swift /path/to/project [--scheme S]
    ↓
[backend/detect]  Package.swift → SwiftPM │ *.xcodeproj/xcworkspace (+--scheme) → xcodebuild │ none → FALLBACK
    ↓ (semantic path)
[backend/swiftpm|xcode]  prime build: `swift build` / `xcodebuild -scheme S build`
    ↓                                   (SourceKit needs a built module graph for full fidelity;
    ↓                                    typeHierarchy in particular requires an up-to-date index)
[lsp/client]  spawn `xcrun sourcekit-lsp`; initialize (rootUri=project, negotiate positionEncoding)
    ↓
[lsp/progress]  poll sourcekit/isIndexing until false   ← MEDIUM confidence: background indexing
    ↓                                               is on by default in current releases; keep polling
[semantic/runner]  for each *.swift file (bounded concurrency):        regardless
    ├─ didOpen (text from disk, version 1)
    ├─ documentSymbol        → document structure, definition candidates
    ├─ per symbol: textDocument/symbolInfo → USR, kind, containerName, systemModule
    ├─ per def site: definition (confirm def range + roles)
    ├─ per def: references   → reference occurrences (call sites ⇒ call-graph edges)
    ├─ per func: prepareCallHierarchy + incoming/outgoing (cross-check refs; caller grouping)
    ├─ per type: prepareTypeHierarchy + supertypes/subtypes → SymbolInformation.relationships
    │             (protocol conformance ⇒ isImplementation links; superclass ⇒ reference links)
    └─ on per-file error → route file to fallback/
    ↓ raw Records (path, raw LSP range, symbol-or-USR, roles, kind, docstring)
[symbol/mapper]  USR → scip symbol string (Pattern 3); local vars → local symbols
    ↓
[emit/positions] UTF-16 → UTF-8 byte offsets (Pattern 4)
    ↓
[emit/index]  dedup occurrences (lint's duplicateOccurrenceWarning), emit SymbolInformation for
    every referenced symbol (lint's missingSymbolForOccurrenceError is a hard error), attach
    relationships; then scip.Canonicalize + Sort (nonCanonicalSymbolError otherwise)
    ↓
write --output index.scip (or <project>/index.scip default)
    ↓
subprocess validation: `scip lint index.scip` must exit 0; `scip snapshot --to …` for goldens
    ↓
jarvis: `jarvis index <repo>` → MCP tools (goToDefinition/findReferences/callHierarchy/
        typeHierarchy/documentSymbols) served from the .scip + Zoekt
```

### Fallback substitution flow

```
no toolchain / initialize failed / --fallback │ per-file semantic failure
    ↓
[fallback/indexer] tree-sitter parse → AST walk:
    document symbols (kind from node type table), definitions (named decl nodes),
    intra-file identifier references resolved through a file-local scope map
    ↓ same Record type, syntax-level only
[emit/*] identical assembly (synthetic module-scoped symbols, no disambiguators, no relationships)
    ↓ same .scip emission + lint gate
```

Substitution, not merging: a run is semantic-per-file or fallback-per-file for each document; `ToolInfo.arguments` records which path produced each run (e.g. `["--mode=hybrid", "--fallback-files=3"]`) so jarvis can surface degradation to users.

### State Management

- The LSP session is the only stateful long-lived object (open documents, server-assigned IDs); it is torn down deterministically (shutdown → exit) via `defer`, including on signal.
- Everything downstream (records → index) is batch-pure: assemble at end of file walk, single `*scip.Index` in memory. Streaming (`bindings/go/scip/parse.go` `ParseStreaming`) is a *reader*-side facility; writer-side memory is bounded by index size, which for real Swift projects (thousands of files) is tens of MB — acceptable; revisit only if jarvis reports pressure.

### Key Data Flows

1. **Symbol identity flow:** LSP range → `symbolInfo` → USR (+container chain) → mapper → symbol string → used as map key for dedup and SymbolInformation completeness. One USR = one symbol, across files, extensions, and overload sets (disambiguator participates only at the string level).
2. **Call-graph flow:** callee definition + reference occurrences at call sites carry `SymbolRole` (read/write/definition bits from `bindings/go/scip/symbol_role.go`); jarvis derives callHierarchy from these edges. LSP `incomingCalls`/`outgoingCalls` are used as a *verification sample* and to catch dynamic-dispatch sites (`isDynamic`, `receiverUsrs` from symbolInfo → emit references for each potential receiver USR).
3. **Type-graph flow:** `prepareTypeHierarchy` at each type definition → supertypes/subtypes → `SymbolInformation.relationships` (`is_implementation` for protocol conformances; reference relationships for superclass links) — this is what powers typeHierarchy in jarvis without a second query path.

## Component Boundaries: New vs Reused vs Modified (explicit)

### New (all under `/Users/ddphuong/Projects/jarvis-ai/scip/swift/`)

Everything in Recommended Project Structure above. Nothing outside `swift/` is added except the wiring below.

### Reused unchanged (integration points by absolute path)

| Component | Path | How scip-swift uses it |
|-----------|------|------------------------|
| Go bindings library | `/Users/ddphuong/Projects/jarvis-ai/scip/bindings/go/scip/` | Import for `scip.Index/Document/Occurrence/SymbolInformation`, `Canonicalize`, `Sort`, `FormatSymbol`; dependency via `replace` in `swift/go.mod` (reprolang pattern) |
| testutil snapshot harness | `/Users/ddphuong/Projects/jarvis-ai/scip/bindings/go/scip/testutil/` | `testutil.SnapshotTest` / `SnapshotTestDirectories` for golden tests under `swift/testdata/snapshots/` |
| SourceFile helpers | `.../bindings/go/scip/source_file.go` | `NewSourcesFromDirectory` for test fixture loading |
| scip CLI (lint/snapshot/test) | `/Users/ddphuong/Projects/jarvis-ai/scip/cmd/scip/` | Subprocess validation gate in tests + CI (Pattern 5) |
| reprolang as reference | `/Users/ddphuong/Projects/jarvis-ai/scip/reprolang/` | Copybook for module layout, cgo grammar binding, snapshot wiring — not imported |

### Modified (small, mechanical)

| File | Change |
|------|--------|
| `/Users/ddphuong/Projects/jarvis-ai/scip/go.work` | `use (...)` gains `swift` (4th module) |
| `/Users/ddphuong/Projects/jarvis-ai/scip/flake.nix` + `checks.nix` | Add `swift` module to build/check matrix. cgo needs a C compiler (already required by reprolang); tree-sitter grammar regeneration needs `tree-sitter-cli` (npm/node input). SourceKit-dependent tests CANNOT run under Nix sandbox/Linux — gate them behind a Darwin+Xcode check or a separate CI job; the fallback path must be Nix-checkable everywhere |
| `/Users/ddphuong/Projects/jarvis-ai/scip/.github/workflows/ci.yaml` | New `macos-latest` job (arm64; add an x86_64 runner or Rosetta build step for the second arch) building `swift/cmd/scip-swift` and running semantic tests; release workflow gains a `scip-swift` macOS artifact |
| `/Users/ddphuong/Projects/jarvis-ai/scip/README.md`, `docs/` | Mention the new module (docs updates only) |

### External processes (boundaries)

| Process | Contract | Notes |
|---------|----------|-------|
| SourceKit-LSP | stdio JSON-RPC child; our client owns lifecycle | Locate via `xcrun sourcekit-lsp` (bundled with Xcode — README confirms; xcrun resolves per-toolchain). Requires Xcode or a swift.org toolchain on PATH |
| `swift build` / `swift package describe` | subprocess, exit code + stdout | SwiftPM backend priming + module/target enumeration |
| `xcodebuild -scheme S …` | subprocess | xcodeproj backend: prime build; extract compile settings. SourceKit-LSP has NO native .xcodeproj support (confirmed: it handles SwiftPM + compile_commands.json natively, and Build Server Protocol via extension). Options: synthesize a compile_commands.json from `xcodebuild` build settings, or drive SolaWing/xcode-build-server (BSP, brew-installable) — MEDIUM confidence on which is less fragile; decide by spike in phase order |
| tree-sitter (in-process) | cgo, vendored grammar | alex-pinkus/tree-sitter-swift, actively maintained (py binding last released 2025-06); no official Go binding → hand-write `binding.go` exactly as reprolang does with `github.com/tree-sitter/go-tree-sitter` (already a repo dependency at v0.25.0) |

## Suggested Build Order (dependency-rationale)

Each phase produces a lint-clean artifact and is demo-able; later phases depend only on earlier ones.

1. **Module skeleton + tree-sitter emission end-to-end** (`swift/` module, `go.work`, `grammar/`, `fallback/`, `emit/`, snapshot harness, `--fallback` CLI).
   *Rationale:* validates the entire emission pipeline — records → assembly → canonicalization → snapshot goldens → `scip lint` subprocess gate — with zero dependency on Xcode, SourceKit, or a Mac with a toolchain. It is the riskiest *integration* surface (new module, cgo grammar vendoring, Nix wiring) and the cheapest *semantic* surface. Everything later reuses `emit/` unchanged.
2. **Symbol mapper** (`internal/symbol/`): USR/descriptor scheme, escaping, disambiguators; property tests against `scip.ParseSymbol` round-trips; golden symbol-string fixtures per decl kind (class/struct/enum/protocol/extension/operator/generic/accessor).
   *Rationale:* pure logic, testable offline; it is the shared vocabulary both paths must speak. Doing it before the LSP layer means LSP integration work is never blocked on identity-design churn.
3. **SourceKit-LSP client + semantic defs/refs on SwiftPM projects** (`internal/lsp/`, `internal/semantic/`): spawn/initialize/position-negotiation, position translation (Pattern 4), `documentSymbol` + `symbolInfo` + `definition` + `references`; per-file fallback wiring.
   *Rationale:* the core value delivery (powers goToDefinition/findReferences/documentSymbols in jarvis). Depends on 1 (emission) + 2 (identity). SwiftPM-only: it is SourceKit-LSP's first-class workspace type (README), so no build-server complexity yet.
4. **Hierarchies** (`prepareCallHierarchy`/`incomingCalls`/`outgoingCalls`, `prepareTypeHierarchy`/`supertypes`/`subtypes` → occurrences roles + `relationships`).
   *Rationale:* both are verified-supported capabilities (Swift Package Index feature matrix shows Call Hierarchy and Type Hierarchy supported), but they require a fully built/indexed project and more LSP round-trips — pure superset of 3's plumbing. Also where `isDynamic`/`receiverUsrs` handling lands (protocol-method references).
5. **xcodebuild backend** (`internal/backend/xcode.go`, `--scheme` flag end-to-end, `testdata/projects/` xcodeproj fixture).
   *Rationale:* last because it is the least-documented integration (compile-args extraction vs BSP — spike needed), it only affects backend selection, and jarvis's contract exposes it as merely "pass `--scheme` if the project has several" — SwiftPM already covers the majority of indexable Swift repos. Also fold in arm64 + x86_64 CI matrix + release artifacts and the jarvis end-to-end validation here (real project → `jarvis index` → MCP query assertions), which is the milestone's done-criteria.

## Scaling Considerations

| Scale | Architecture adjustments |
|-------|--------------------------|
| < 200 files | Single LSP session, sequential per-file loop; whole index in memory. Ship as-is. |
| 200–5k files (typical app) | Bounded concurrent document processing (cap ~4–8 in-flight requests: sourcekitd serializes much of its work, so more buys little); batch `references` by definition rather than by file to avoid duplicate queries; ensure build priming (`swift build`) happens before LSP spawn — one build, thousands of saved recompiles. |
| > 5k files / multi-target monorepos | Index per-target (module) then merge `*scip.Index` documents before canonicalization; consider reusing one long-lived scip-swift daemon later if jarvis re-index cadence makes spawn+build the bottleneck (explicitly out of scope for v1). |

### Scaling Priorities

1. **First bottleneck — LSP round-trip count** (definition+references per symbol): batch by symbol, cache `symbolInfo` results per (file,range), and reuse `documentSymbol` ranges as def-site candidates before querying. Measure requests/sec in a `testdata/projects/large` fixture.
2. **Second bottleneck — build priming time** on first index: jarvis-level concern (caching `.build/` and the emitted `.scip` between runs); scip-swift just must not rebuild when `swift build` is a no-op.

## Anti-Patterns

### Anti-Pattern 1: Emitting LSP UTF-16 positions straight into the index

**What people do:** Copy `Position.character` into SCIP ranges and set `position_encoding = UTF16…` (or forget the field).
**Why it's wrong:** Forgetting it leaves `UnspecifiedPositionEncoding`, which scip.proto explicitly says new indexers must not emit (ambiguous for consumers). Mixing encodings per path (UTF-16 semantic, UTF-8 fallback) makes cross-file diffs and snapshot tests nondeterministic; any consumer comparing tree-sitter and SourceKit ranges then disagrees on non-ASCII lines (emoji/accents in Swift strings are common).
**Do this instead:** One normalization layer (`emit/positions.go`), always emit `UTF8CodeUnitOffsetFromLineStart`, always set the field, always typed ranges.

### Anti-Pattern 2: Occurrences without SymbolInformation

**What people do:** Emit reference occurrences for symbols whose definition wasn't visited (stdlib, dependency modules).
**Why it's wrong:** `missingSymbolForOccurrenceError` is a hard `scip lint` error (verified in `cmd/scip/lint.go` taxonomy: `missingSymbolForOccurrenceError`, plus `bothLocalAndExternalSymbolError`, `forwardDefIsDefinitionError`).
**Do this instead:** Maintain a symbol table during assembly: every occurrence's symbol string must resolve to a `SymbolInformation` in some document (external symbols go to `Index.external_symbols` with a stub package); lint in-process via a `SymbolTable` before writing, mirroring `cmd/scip/lint.go`'s `symbolTable` approach.

### Anti-Pattern 3: Merging semantic and fallback output per file, or name-based identity

**What people do:** Run tree-sitter first, then "enrich" with LSP results keyed by identifier name; or let fallback emit `Foo/bar()` without USR-grade disambiguation into the same document.
**Why it's wrong:** Overloads and extensions collide; duplicate occurrence warnings and wrong-definition navigation follow; golden tests flicker between machines with/without Xcode.
**Do this instead:** Per-file substitution with a recorded mode (Pattern 1); fallback symbols stay unambiguous-by-construction (no disambiguators, no cross-file claims).

### Anti-Pattern 4: Querying before the index is ready / without a build

**What people do:** Send `definition`/`typeHierarchy` immediately after `initialize`.
**Why it's wrong:** SourceKit-LSP answers from its workspace index; on an unbuilt project results are empty or stale (Swift forums: type hierarchy "requires an up-to-date index… you might need to build your project first").
**Do this instead:** Prime the build in the backend layer, then gate on `sourcekit/isIndexing == false`, with a timeout + warning degradation to fallback rather than a hang.

### Anti-Pattern 5: Repo-boundary violations

**What people do:** Put shared helpers in the root module (`cmd/scip`), hand-edit `scip.pb.go`, or add a `scip-swift` subcommand to the `scip` CLI.
**Why it's wrong:** The codebase architecture doc is explicit — new shared code goes in the bindings module, generated files are regenerated, and commands must be registered in `commands()`; but scip-swift is a separate binary with its own release cadence consumed by jarvis's `setup.sh`.
**Do this instead:** Everything lives in `swift/`; if genuinely shared utilities emerge (e.g. LSP position math), they go in `swift/internal/` first and are promoted to bindings only with a second consumer.

### Anti-Pattern 6: Ad-hoc error handling around a subprocess

**What people do:** Treat any LSP error as fatal, or silently swallow it.
**Why it's wrong:** One unresponsive file kills a whole-repo index (or quietly degrades it without trace).
**Do this instead:** Typed error taxonomy in the repo's style (small structs implementing `error` with `error:`/`warning:` prefixes, aggregated with `errors.Join` — mirror `cmd/scip/lint.go:273-410`); per-file degradation surfaces a `warning:`-level report; process-level failures (spawn, initialize) fall back globally; all modes recorded in `ToolInfo.arguments`.

## Integration Points

### External Services

| Service | Integration pattern | Notes / gotchas |
|---------|---------------------|-----------------|
| SourceKit-LSP | child process, stdio, JSON-RPC (Content-Length headers) | `xcrun sourcekit-lsp` resolves the active toolchain; pin/record `sourcekit-lsp --version` in `ToolInfo`. Extensions used: `textDocument/symbolInfo`, `sourcekit/workspace/symbolNames`, `sourcekit/isIndexing` (all documented in LSP Extensions.md). UTF-16 positions assumed (MEDIUM confidence the server supports utf-8 negotiation; layer handles either) |
| swift / swiftPM | subprocess `swift build` | Must run before LSP queries; sandboxed/readonly filesystems break builds → fallback |
| xcodebuild | subprocess with `-scheme S` (from jarvis `--scheme` passthrough) | No native .xcodeproj support in SourceKit-LSP; bridge via compile_commands.json synthesis or xcode-build-server (BSP). Spike decides (MEDIUM confidence) |
| tree-sitter-swift | vendored cgo grammar via `github.com/tree-sitter/go-tree-sitter` v0.25.x | Commit generated `parser.c`; regeneration script like `reprolang/generate-tree-sitter-parser.sh`; grammar actively maintained (last binding release 2025-06) |
| jarvis | file-based: `.scip` at agreed path; binary installed by jarvis `setup.sh` to `~/.jarvis/bin` | Contract verified from jarvis-index README: `jarvis index <path>`, `--scheme` "if the Xcode project has several". README currently says "macOS arm64 only" — this milestone supersedes with arm64 + x86_64 per PROJECT.md; update jarvis docs at phase 5 |
| `scip` CLI | subprocess in tests/CI (`lint`, `snapshot`, `test`) | `cmd/scip` is package main — importable-behavior duplication is worse than a subprocess; reprolang's `test_cmd_test.go` is the template |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `swift` module ↔ `bindings/go/scip` | Direct Go import (via `replace`) | Read-only consumption of published API; never modify bindings for indexer convenience without treating it as a public-API change |
| `swift` module ↔ `cmd/scip` | Process boundary (CLI invocation) | Keeps validation authoritative and identical to what users/jarvis run |
| `semantic/` ↔ `lsp/` | In-process interface: a narrow `SemanticSource` (document symbols, def/ref/hierarchy queries) | The interface is what lets tests fake an LSP server (replay fixtures) and lets the fallback path be a sibling implementation |
| `emit/` ↔ both paths | Shared `Record` type | The single point where "source of truth about a symbol occurrence" is defined; position normalization and dedup live only here |
| `backend/` ↔ `semantic/` | Backend returns a `WorkspacePlan` (files, modules, primed bool) | Detection must be pure (no side effects) so tests and `--dry-run` style reporting stay cheap |

## Sources

- Local repository (read 2026-08-16): `/Users/ddphuong/Projects/jarvis-ai/scip/scip.proto` (symbol grammar lines ~150–205, PositionEncoding ~118–152, Occurrence typed-range note), `/Users/ddphuong/Projects/jarvis-ai/scip/go.work`, `reprolang/go.mod`, `reprolang/repro/indexer.go`, `bindings/go/scip/testutil/snapshot_testing.go`, `cmd/scip/lint.go` (error taxonomy)
- SourceKit-LSP LSP Extensions (official contributor docs): https://github.com/swiftlang/sourcekit-lsp/blob/main/Contributor%20Documentation/LSP%20Extensions.md
- SourceKit-LSP README (workspace types, toolchain bundling): https://github.com/swiftlang/sourcekit-lsp
- SourceKit-LSP feature matrix (Call Hierarchy / Type Hierarchy supported): https://swiftpackageindex.com/swiftlang/sourcekit-lsp
- Type hierarchy requires built project — Swift Forums: https://forums.swift.org/t/sourcekit-lsp-and-the-typehierarchy-family-of-requests/77058
- LSP 3.17 specification (UTF-16 default, positionEncoding negotiation, hierarchy requests): https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/
- tree-sitter-swift grammar (maintained): https://github.com/alex-pinkus/tree-sitter-swift (binding cadence: https://pypi.org/project/py-tree-sitter-swift/)
- xcode-build-server (BSP bridge for .xcodeproj): https://github.com/SolaWing/xcode-build-server ; maintainer context: https://forums.swift.org/t/xcode-project-support/20927
- jarvis-index (consumer contract, setup.sh, `--scheme` wording): https://github.com/jarvis-intelligence/jarvis-index
- SCIP protocol and ecosystem background: https://github.com/scip-code/scip ; https://sourcegraph.com/blog/announcing-scip ; clangd SCIP discussion: https://github.com/clangd/clangd/issues/1340

---
*Architecture research for: scip-swift hybrid indexer (SourceKit-LSP + tree-sitter) in the scip monorepo*
*Researched: 2026-08-16*
