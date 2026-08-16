# SourceKit-LSP Spike Findings — Capability Inventory & Harvest Performance (Phase 1, Plan 01-03)

## Metadata

| Field | Value |
| --- | --- |
| Date | 2026-08-16 |
| Host | macOS arm64 (darwin 25.5.0, arm64) |
| sourcekit-lsp | `/Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/bin/sourcekit-lsp` (discovered via `xcrun -f sourcekit-lsp`) |
| sourcekit-lsp version | Not directly printable on this build (`--version` is not a flag; `initialize` returned no `serverInfo`). Version is evidenced by the bundling toolchain: Xcode 26.3 (Build 17C529), Apple Swift 6.2.4 (swiftlang-6.2.4.1.4), swift-driver 1.127.15. Requirement was >= 6.1 — satisfied by toolchain version. |
| Driver | `swift/spikes/lsp_probe` (jsonrpc2 v0.2.2 over stdio; readiness gate before any harvest) |
| Capability evidence | `swift/spikes/evidence-capability.jsonl` (204 records; committed) |
| Perf evidence | `swift/spikes/evidence-perf-cold.jsonl`, `swift/spikes/evidence-perf-warm.jsonl` (14,012 records each; runtime output, git-ignored — regenerate via the commands at the end of this doc) |
| Perf fixture | 500 generated files, 13,001 unique defs, 35,500 harvested occurrences |
| Readiness discipline | Every harvest was preceded by the gate: `sourcekit/isIndexing` poll + a `textDocument/symbolInfo` probe at a known definition returning a non-empty USR. No capability conclusion below is taken from a cold or unready index. |

## Thresholds

Agreed go/no-go thresholds (ASSUMED numbers — RESEARCH confidence gap 1: no measured baseline existed when they were proposed; if revised, the verdict is re-evaluated before Phase 3 planning):

1. `textDocument/symbolInfo` returns distinct USRs for >= 95% of fixture definitions, including every overload.
2. Cross-file same-module extension members resolve to the extended type's container.
3. Retroactive extension member container reflects the extended type.
4. Full perf-fixture harvest <= 30 min cold / <= 10 min warm, with no server crash requiring restart.
5. Call/type-hierarchy requests return usable data (not hard errors).

## Capability inventory

Observed on the hard-case fixture (`swift/spikes/fixture/`, module `CapabilityFixture`). Every row cites a response excerpt from `evidence-capability.jsonl`.

| # | Hard case | Observed | Evidence (excerpt) |
| --- | --- | --- | --- |
| 1 | Overload USR distinctness | yes | Three `f` overloads, three distinct USRs: `f(_: Int)` -> `s:17CapabilityFixture1fyS2iF`, `f(_: String)` -> `s:17CapabilityFixture1fyS2SF`, `f(_:_: Int, Int)` -> `s:17CapabilityFixture1fyS2i_SitF` |
| 2 | Cross-file same-module extension attribution | yes (via USR) | `extension Shape { func area2() }` declared in `Extensions.swift` resolves to `usr: s:17CapabilityFixture5ShapeV5area2SdyF` — the USR embeds `5ShapeV` (struct Shape). `systemModule.moduleName: CapabilityFixture`, `isSystem: false` |
| 3 | Retroactive extension containerName | yes via USR; `containerName` itself is null | `extension String { func spikeFlag() }` -> `usr: s:SS17CapabilityFixtureE9spikeFlagSSyF` — `SS` = extended type `String` (Swift stdlib), `17CapabilityFixtureE` marks the extending module. The extended type's identity is fully recoverable from the USR. Note: `containerName` was null on all 60 symbolInfo responses — sourcekit-lsp does not populate it; containers must be derived from USRs or the documentSymbol tree (a Phase 3 mapper input, not a gap). |
| 4 | Generics (generic func, generic type, generic method) | yes | `id<T>` -> `s:17CapabilityFixture2idyxxlF`; `Box<T>.map<U>` -> `s:17CapabilityFixture3BoxV3mapyACyqd__Gqd__xXElF`; `Repository.find` with associatedtype -> `s:17CapabilityFixture10RepositoryP4findy6EntityQzSgSiF` |
| 5 | Protocol requirement + witness + existential call | yes | Requirement `Drawable.draw` -> `s:17CapabilityFixture8DrawableP4drawSSyF`; witness `Circle.draw` -> `s:17CapabilityFixture6CircleV4drawSSyF` (distinct). Call through the existential `d.draw()` returns `isDynamic: true`, `receiverUsrs: ["s:17CapabilityFixture8DrawableP"]` — exactly the dynamic-dispatch signal SCIP relationships need. |
| 6 | Operator declarations | yes | `static func +` -> `s:17CapabilityFixture3VecC1poiyA2C_ACtFZ`; `static func ==` -> `s:17CapabilityFixture3VecC2eeoiySbAC_ACtFZ` (trailing `Z` = static) |
| 7 | Computed-var get/set accessors | partial (single symbol) | `var area { get set }` is one symbol: `s:17CapabilityFixture5ShapeV4areaSdvp` (var descriptor). Accessors are not separate documentSymbols or USRs. Getter/setter Kinds must therefore come from syntax (Phase 2 fallback) or be derived from occurrence roles, not from LSP — matches the flagged ASSUMED accessor mapping in the scheme. |
| 8 | Unicode declarations + emoji literals | yes | `func 🚀()` -> `usr: s:17CapabilityFixture004BFIhSSyF`, kind 12 (Function); `var π` -> `s:17CapabilityFixture003BxaSdvp`, kind 13 (Variable); `struct F⃗` -> kind 23 with USR. Emoji string literal positions are UTF-16-correct (e.g. documentSymbol for `🚀()` at line 2 character 12 = `public func ` prefix length). `positionEncoding` is not advertised in capabilities -> LSP default UTF-16 applies; the driver's UTF-16 math agrees with the server. |
| 9 | Class inheritance + override | yes | `Animal.speak` -> `s:17CapabilityFixture6AnimalC5speakSSyF`; `Dog.speak` (override) -> `s:17CapabilityFixture3DogC5speakSSyF` — own identity preserved, distinct from the base. |
| 10 | Call hierarchy | yes | Advertised `callHierarchyProvider: true`. `textDocument/prepareCallHierarchy` at `d.draw()` returned the item (`data.usr: s:17CapabilityFixture8DrawableP4drawSSyF`); `callHierarchy/incomingCalls` returned 1 call from `s:17CapabilityFixture8callDrawySSAA8Drawable_pF` with `fromRanges`; at `Vec` init: 4 incoming callers. `outgoingCalls` returned a well-formed empty array (draw has no outgoing calls). |
| 11 | Type hierarchy | yes | Advertised `typeHierarchyProvider: true`. `textDocument/prepareTypeHierarchy` at `Vec` returned the item (detail `CapabilityFixture`); `typeHierarchy/supertypes` returned `[]` (Vec has no supertypes — semantically correct); `typeHierarchy/subtypes` returned 1 item with detail `Extension at Extensions.swift:15` (the `extension Vec`). No hard errors. |
| 12 | System/SDK symbol attribution | yes | Probing `String` at the retroactive extension returns `isSystem: true`, `systemModule: {moduleName: "Swift", groupName: "String"}`, `usr: s:SS` — the system-module manager path of the frozen scheme (`scip-swift swift Swift <ver>`) is directly supported. References even include occurrences in `.swiftinterface` system files (`arm64e-apple-macos.swiftinterface`). |
| 13 | `sourcekit/isIndexing` extension | no — not supported | `jsonrpc2: code -32601 message: method not found: sourcekit/isIndexing` (Xcode 26.3 build). The readiness gate's authoritative symbolInfo-USR probe still passed in 2.3-4.4 s. Phase 3 must gate readiness on the symbolInfo probe (with retry), not on isIndexing. |
| 14 | USR coverage (threshold input) | yes | 60/60 symbolInfo calls returned a non-empty USR (100% >= 95%), including all 3 overloads (3/3 distinct). |

Additional recorded facts: `InitializeResult.capabilities` advertises `referencesProvider`, `documentSymbolProvider`, `definitionProvider`, `declarationProvider`, `implementationProvider`, `semanticTokensProvider`, `renameProvider {prepareProvider: true}` (full verbatim record in the initialize response in the evidence file). Hierarchical document symbols work (`hierarchicalDocumentSymbolSupport` requested and honored). `documentSymbol` returns `init`/`deinit`/operators as ordinary symbols.

## Performance

Harvest discipline: one `didOpen` + one `documentSymbol` pass per file, then one `references` pass per unique definition, bounded concurrency 4 (worker pool over definitions). Numbers are from the summary records of the two evidence files.

| Metric | Cold (fresh server, no `.build/`) | Warm (second run, `.build/` present) |
| --- | --- | --- |
| Time to ready (gate) | 3.27 s | 1.03 s |
| Total harvest wall time | 4 m 02.4 s | 4 m 00.4 s |
| Request count | 13,506 | 13,506 |
| Requests/sec | 55.7 | 56.2 |
| p50 request latency | 35.3 ms | 34.9 ms |
| p95 request latency | 126.9 ms | 142.7 ms |
| Server restarts/crashes | 0 | 0 |
| Symbols + occurrences harvested | 13,001 + 35,500 | 13,001 + 35,500 |
| Server RSS at end | 62,064 KB (60.6 MB) | 51,456 KB (50.2 MB) |

Notes: the 500-file fixture is ~26 defs/file (above the ~15 target — conservative, i.e. more work per file than planned). Cold and warm are nearly identical because sourcekit-lsp's background indexing had already progressed during the didOpen/documentSymbol wave of the cold run; the gate time (3.3 s vs 1.0 s) and p95 (127 vs 143 ms) are the visible cold/warm differences. Deterministic: both runs harvested exactly the same symbol and occurrence counts.

## Verdict

GO

Per-threshold evaluation:

- Threshold 1 (>= 95% distinct USRs incl. every overload): PASS — 60/60 symbolInfo responses returned USRs; the three `f` overloads have three distinct USRs (inventory rows 1, 14).
- Threshold 2 (cross-file extension attribution): PASS — `area2()` USR embeds the extended struct (`5ShapeV`) even though declared in another file (row 2).
- Threshold 3 (retroactive container reflects extended type): PASS with a mechanism change — `containerName` is never populated by this server, but the retroactive member's USR (`s:SS17CapabilityFixtureE9spikeFlagSSyF`) encodes both the extended type (`SS` = String) and the extending module; the extended type's identity is fully recoverable. The Phase 3 mapper derives containers from USRs, not from `containerName` (row 3).
- Threshold 4 (perf: <= 30 min cold / <= 10 min warm, no unrecoverable crash): PASS — 4 m 02 s cold / 4 m 00 s warm on 500 files; zero crashes in both runs.
- Threshold 5 (call/type-hierarchy usable): PASS — both providers advertised and returning real data (incoming calls with ranges; subtypes including extensions); empty arrays only where semantically correct (rows 10, 11).

Index-store escape hatch (not triggered): had any threshold failed, the documented fallback was to keep the symbol/emission layers unchanged and swap the bulk harvest to reading `.build/index/store` via a Swift helper subprocess or cgo libIndexStore, keeping LSP for live queries only. With GO, Phase 3 builds the LSP-client harvest path per this evidence; the two design inputs it must honor are (a) gate readiness on the symbolInfo probe because `sourcekit/isIndexing` is unsupported here, and (b) derive container attribution from USRs because `containerName` is always null.

## How to re-run

Requires macOS with Xcode (runtime-only; the server is discovered via `xcrun -f sourcekit-lsp`, never hardcoded). From the repo root:

```bash
# Capability inventory (evidence-capability.jsonl, committed)
cd swift
GOWORK=off go run ./spikes/lsp_probe \
  -fixture spikes/fixture -out spikes/evidence-capability.jsonl \
  -mode capability -ready-timeout 10m

# Perf fixture (deterministic; byte-stable given the seed)
cd swift/spikes/perf
GOWORK=off go run ./generate -out gen -files 500 -seed 42
rm -rf gen/.build

# Perf runs (cold first on the untouched tree, then warm on the built tree)
cd swift
GOWORK=off go run ./spikes/lsp_probe \
  -fixture spikes/perf/gen -out spikes/evidence-perf-cold.jsonl \
  -mode perf -ready-timeout 30m
GOWORK=off go run ./spikes/lsp_probe \
  -fixture spikes/perf/gen -out spikes/evidence-perf-warm.jsonl \
  -mode perf -ready-timeout 30m
```

The perf evidence JSONL files (~6 MB each) are git-ignored runtime outputs; the summary metrics live in the last `phase:"summary"` record of each file. Build/vet gates: `cd swift && GOWORK=off go build ./spikes/... && GOWORK=off go vet ./spikes/...`.

---

*Phase: 01-symbol-scheme-module-foundations, plan 01-03. Evidence recorded 2026-08-16.*
