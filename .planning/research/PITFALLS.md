# Pitfalls Research

**Domain:** Swift code indexing (SourceKit-LSP semantic path + tree-sitter fallback) emitting SCIP indexes, added to the existing scip protocol monorepo
**Researched:** 2026-08-16
**Confidence:** HIGH (repo-specific facts verified by inspection 2026-08-16; external facts source-cited; individual claims tagged HIGH/MEDIUM/LOW)

---

## Critical Pitfalls

### Pitfall 1: Querying SourceKit-LSP before the index is warm — silently wrong or empty results

**What goes wrong:**
The indexer starts `sourcekit-lsp`, immediately asks for definitions/references, and gets empty, partial, or stale answers — producing an index that looks valid, passes `scip lint`, and is wrong. Cross-module references are simply missing.

**Why it happens:**
SourceKit-LSP's semantic answers depend on index data that historically only existed after a build. Background indexing (which fixes this) is **enabled by default only since Swift 6.1 (March 2025)** and **only for SwiftPM projects** — Xcode/xcodebuild projects are excluded (sourcekit-lsp issues #1269, #1271). Even with it, the initial index build takes "2–3x the time of a regular build" (issues #1262, #1268) and is **separate from `swift build`** — a command-line build does not update it (issue #1270). On Swift 5.9 toolchains background indexing doesn't exist at all (issue #1626). There is no LSP notification that reliably says "indexing done."

**How to avoid:**
- Make index readiness an explicit gate, not an assumption: run a warm-up (background indexing or an actual build), then verify by probing known symbols; track "% of files with zero cross-file occurrences" as a fail-loud metric.
- Cache the index-store/build directory in CI between runs (fresh checkouts are the worst case).
- Document and test the minimum toolchain (Swift 6.1+) and refuse to run the semantic path (fall back + warn) on older toolchains.

**Warning signs:**
Works on the developer machine (warm `.build/` and index cache), fails or is empty in CI on a fresh clone; definitions resolve only within the same file; tests pass after a second run.

**Phase to address:**
Phase 3 (SourceKit-LSP path) — readiness gate designed in from the first semantic-path commit, not retrofitted.

---

### Pitfall 2: Treating LSP as a whole-workspace indexer — per-symbol round-trip storms

**What goes wrong:**
The indexer opens each file, then issues `textDocument/references` for every symbol in every file. On a real app (thousands of files × hundreds of symbols) this is O(files × symbols) JSON-RPC round-trips, hours of runtime, and server crashes/respawns mid-run — and there is no LSP "bulk index workspace" API.

**Why it happens:**
LSP is an editor protocol, not a batch indexer protocol. This is precisely why scip-clang drives Clang ASTs / index stores instead of driving clangd (Sourcegraph evaluated the converter approaches explicitly — see sourcegraph issue #42280). The Swift ecosystem has direct index-store consumers: indexstore-db, MobileNativeFoundation/swift-index-store, kateinoigakukun/swift-indexstore (pure Swift).

**How to avoid:**
- Architecture decision, early: prefer reading the compiler **index store** (produced by `swift build -Xswiftc -index-store-path …` or SourceKit-LSP's background indexing) as the bulk data source, using LSP only for what the store lacks. Keep SourceKit-LSP as the "primary semantic engine" for hierarchies/live queries but don't make it the bulk def/ref harvest engine.
- If LSP harvesting is kept: one `documentSymbol` pass per file, one `references` pass per *unique* definition, concurrency-capped client with watchdog timeouts and crash-respawn that re-opens files.
- Set a performance budget fixture (e.g. "index a ~500-file Swift package in < N minutes") and track it in CI.

**Warning signs:**
Indexing time grows super-linearly with repo size; intermittent "server crashed" logs; CI timeout flakes.

**Phase to address:**
Phase 1 (spike decides LSP-harvest vs index-store-read; HIGH-value decision) — implementation Phase 3.

---

### Pitfall 3: UTF-16 (LSP) vs UTF-8-byte (SCIP repo tooling) position mismatch

**What goes wrong:**
SourceKit-LSP returns positions in **UTF-16 code units** (LSP default). If those are written into SCIP occurrences unconverted (or with the wrong `position_encoding`), any line containing emoji, CJK, or other multi-byte characters (ubiquitous in Swift string literals and even identifiers) yields ranges that point mid-rune or past line end. `scip.proto` allows UTF-8/UTF-16/UTF-32 per document — but this repo's own tooling assumes UTF-8 bytes: `bindings/go/scip/source_file.go` `RangeText` slices `Lines[line][start:end]` as Go bytes, and it does so **unchecked** (a known CONCERNS.md entry: panics on out-of-range).

**Why it happens:**
LSP's UTF-16 heritage (microsoft/language-server-protocol#872) vs Swift sources being UTF-8 vs Swift `String.Index` being grapheme-cluster-based — three different index spaces. SourceKit-LSP itself has a history of position-encoding bugs (SR-9311, `encodedOffset` misuse).

**How to avoid:**
- Convert UTF-16 → UTF-8 byte offsets at the LSP boundary; emit `Document.position_encoding = UTF8CodeUnitOffsetFromLineStart` on every document (the proto's own guidance for indexers implemented in "Go, Rust or C++", and the only encoding this repo's testutil handles correctly).
- Never mix encodings across documents in one index.
- Add a fixture file with emoji identifiers and string literals to snapshot tests from day one — it converts this class of bug from "found in production" to "found in CI."

**Warning signs:**
`scip snapshot` output with mojibake or `index out of range` panics; navigation lands one character off on lines with non-ASCII; only ASCII fixtures pass.

**Phase to address:**
Phase 1 (position encoding contract in the skeleton) — verified in Phase 2 fixtures.

---

### Pitfall 4: Swift's Unicode identifiers trigger the known `ParseSymbol` multi-byte-rune panic

**What goes wrong:**
Swift identifiers are Unicode (`func π()`, `let 🚀`, Vietnamese/CJK identifiers in real codebases). CONCERNS.md documents a VERIFIED bug: `scip.ParseSymbol` panics when a symbol string ends with a multi-byte rune that isn't a valid descriptor suffix — and `scip lint` / `scip snapshot` call `ParseSymbol` on **every occurrence symbol**, so one emoji-named Swift symbol crashes the entire CLI instead of producing a lint error.

**Why it happens:**
The upstream Go bindings were built for mostly-ASCII identifier languages. Swift makes this a certainty, not an edge case. scip-swift feeds attacker-of-no-one-but-reality Unicode names straight into the fragile parser.

**How to avoid:**
- Fix the `peekNext` guard in `bindings/go/scip/symbol_parser.go` (one-line change per CONCERNS) and add a fuzz/regression test **before** mass symbol emission begins — upstream-worthy PR.
- Decide the symbol-name escaping policy (backtick escaping is part of the SCIP symbol grammar) and cover it with fixtures (`unicodesymbols.swift`).
- Related: `scip stats` LOC bug — pass `--project-root` explicitly when measuring scip-swift indexes, or LOC reads 0.

**Warning signs:**
`runtime error: index out of range` from `scip lint` on any Swift index containing non-ASCII final descriptor characters.

**Phase to address:**
Phase 1 — fix lands before the first emitted index; guarded by a fuzz test.

---

### Pitfall 5: Symbol-scheme instability and missing disambiguators for overloads/extensions

**What goes wrong:**
The symbol-string format (`swift Package:module Type.method().…`) drifts between releases or, worse, **differs between the tree-sitter path and the SourceKit-LSP path** for the same entity. Consequence: downstream dedup (jarvis goToDefinition/findReferences keyed on symbol strings) returns duplicates or broken references. `scip lint` will also fail on non-canonical formatting (`ParseSymbol` → `Format` round-trip check, `nonCanonicalSymbolError`).

**Why it happens:**
Swift makes symbol identity hard: same-named methods across extensions in multiple files/modules; overloaded `f(_:)` variants; same symbol string legitimately produced from two harvesting paths. Without a written scheme + shared namer module, each path (and each contributor) invents rules.

**How to avoid:**
- Write the Swift symbol scheme spec (module, type nesting, disambiguators for overloads — e.g. arity/type-based, following scip-java's disambiguator precedent) as a Phase-1 deliverable.
- One shared "namer" package both paths call (reprolang's `repro/namer.go` is the in-repo precedent).
- Golden test asserting tree-sitter and semantic paths produce identical symbol strings for the shared fixture corpus.

**Warning signs:**
Snapshot diffs show renames of unrelated symbols; jarvis findReferences returns two definitions for one call site; lint reports non-canonical symbols.

**Phase to address:**
Phase 1 (scheme spec + namer) — enforced continuously via the dual-path golden test (Phase 2/3).

---

### Pitfall 6: Silent degradation to tree-sitter fallback

**What goes wrong:**
When SourceKit-LSP is unavailable/unwarm (Linux, no toolchain, unbuilt Xcode project), the indexer quietly emits heuristic, syntax-level defs/refs. Users (and jarvis) believe results are semantic; they get name-matching guesses with false positives (same-named methods in unrelated types) and missing cross-file refs — trust in the product erodes invisibly.

**Why it happens:**
"Graceful fallback" is the design goal, but without visibility controls graceful becomes silent. Heuristic def/ref has no type checking, so every same-named identifier is a candidate reference.

**How to avoid:**
- Stamp the mode into the index: `ToolInfo` + per-document marker or metadata; jarvis can surface "syntax-only" confidence.
- CLI warning on fallback + `--strict-semantic` flag that fails instead of degrading.
- Publish the accuracy delta: a fixture corpus indexed both ways, diffed, with numbers (also drives roadmap honesty about what fallback is *for*).

**Warning signs:**
Bug reports "definition goes to the wrong class"; no telemetry distinguishing modes; tests only exercise the happy semantic path.

**Phase to address:**
Phase 2 (fallback built with mode-signaling from the start).

---

### Pitfall 7: tree-sitter-swift grammar gaps on modern Swift

**What goes wrong:**
The fallback silently produces garbage or drops symbols on Swift the grammar doesn't know. Known: freestanding macros (Swift 5.9+) unsupported (alex-pinkus/tree-sitter-swift #438) — including ubiquitous `#Preview` in SwiftUI code — yielding ERROR nodes; subtrees under ERROR nodes vanish from the emitted index. The grammar historically lags language evolution.

**Why it happens:**
tree-sitter-swift is a community grammar; Swift moves fast (macros 5.9, Typed throws 6.0, InlineArray 6.2…). ERROR nodes don't fail the parse — they fail *silently downstream*.

**How to avoid:**
- Pin the grammar version; vendor the generated parser the reprolang way (`generate-tree-sitter-parser.sh`, ABI pin).
- Count ERROR/MISSING nodes per file during indexing; surface as warning + stat; fixtures assert zero parse errors on the representative corpus (SwiftUI-heavy, macros, `#if`, generics).
- CI job that re-runs the corpus on grammar bumps.

**Warning signs:**
Fixture coverage numbers (occurrences/file) drop after a grammar bump; SwiftUI preview code absent from snapshots.

**Phase to address:**
Phase 2.

---

### Pitfall 8: xcodebuild scheme discovery fragility (Xcode-project indexing)

**What goes wrong:**
For `.xcodeproj`-based repos, SourceKit-LSP shells out to `xcodebuild` for scheme/build-settings discovery — which triggers package resolution and even provisioning **network** queries, is slow (minutes on cold CI), breaks on unshared schemes (invisible to headless `xcodebuild`), and has produced JSON-RPC `-32603` failures (Swift Forums thread 86000). And background indexing does **not** cover xcodebuild projects at all — a full build is required for semantics.

**Why it happens:**
xcodebuild was designed for interactive Xcode, not headless consumers; the jarvis-index README already promises an "Xcode scheme flag option," meaning this path is on the roadmap.

**How to avoid:**
- v1: SwiftPM projects are first-class; Xcode-project support behind an explicit `--scheme` flag (matching the README promise), never implicit discovery.
- Document/implement the required warm-up build for Xcode projects; treat scheme discovery as a fallible, timeout-guarded, cached step.

**Warning signs:**
CI hangs at startup on Xcode-project fixtures; intermittent failures that pass on retry; "does not contain scheme" errors.

**Phase to address:**
Phase 3 (SwiftPM) — Xcode-project support consciously later; flag plumbed in Phase 3, hardened when scheduled.

---

### Pitfall 9: Emitting SDK / dependency occurrences unfiltered — index blowup

**What goes wrong:**
Swift code references Foundation/SwiftUI/ObjC constantly. SourceKit-LSP/index-store results include clang-generated decls in SDK headers (ObjC interop: `c:objc(cs)…` USRs). Emitting those occurrences/external symbols wholesale produces gigabyte indexes where 95% of content is SDK references nobody navigates to — tripping the repo's documented scaling limits (whole-index in-memory CLI reads; 1–2 GB comfort zone).

**Why it happens:**
The naive rule "every reference becomes an occurrence" ignores that SCIP's value is *project-relative* navigation; external symbols should be minimal stubs (scip-java's JDK precedent), not full graphs.

**How to avoid:**
- Occurrences only for files under `project_root`; external symbols only for actually-referenced entities (name + kind, no relationships).
- Size budget in tests: assert index size per LOC stays under a threshold; `scip stats` in CI (with `--project-root` — see the LOC bug).

**Warning signs:**
Index size ≫ source size; `scip lint`/`stats` slow and memory-hungry; snapshot diffs dominated by SDK symbols.

**Phase to address:**
Phase 3 (first semantic emission) — budget asserted in Phase 5 CI.

---

### Pitfall 10: Mis-modeling protocol witnesses, generics, and extension semantics

**What goes wrong:**
A call `x.foo()` where `foo` comes from a protocol extension resolves (via LSP) to the protocol requirement, not the concrete implementing method — or to several locations. Generic specializations produce several distinct USRs for one source declaration. Extensions add the same method to a type from multiple files/modules. Naive emission yields: wrong is_definition, duplicated definitions for one entity, type hierarchies that miss witnesses (jarvis typeHierarchy), call hierarchies that break at protocol boundaries. Also: SourceKit-LSP's call-hierarchy support is limited/immature (MEDIUM confidence — verify in the spike).

**Why it happens:**
Swift's semantic model (witness tables, retroactive extensions, specialization) doesn't map 1:1 onto SCIP's symbol/relationship model; LSP answers reflect *compiler* truth, not *navigation-friendly* truth.

**How to avoid:**
- Phase-1 spike produces a capability inventory: run SourceKit-LSP against fixtures exercising each hard case (protocol witness, retroactive extension, overload, generic, ObjC bridging) and record what actually comes back; design emission rules from observations, not assumptions.
- Emit relationships conservatively (is_definition, implemented-by/implements where confident); document known-wrong cases as explicit limitations rather than shipping guesses.
- Decide access-level + test-target scope explicitly (see Looks-Done checklist).

**Warning signs:**
E2E tests only use single-file fixtures; hierarchies "work" in demos on simple classes but miss protocol-conforming call sites; duplicate SymbolInformation warnings in lint.

**Phase to address:**
Phase 1 (spike/inventory) and Phase 4 (hierarchies + relationships).

---

### Pitfall 11: `#if` conditional compilation creates multiple symbol universes

**What goes wrong:**
The semantic path sees only the branch active under the build configuration used (`#if os(macOS)`, `#if DEBUG`, arch canary checks) — so the index content depends on *how* it was built. On an arm64 vs x86_64 machine, the same repo yields different indexes → snapshot tests non-reproducible across the two CI platforms. Meanwhile the tree-sitter fallback parses *all* branches, producing a third universe — the same symbol emitted with different identities per path (see Pitfall 5).

**Why it happens:**
Compiler truth is configuration-dependent; tree-sitter is not. Neither matches "one canonical index."

**How to avoid:**
- Declare a canonical indexing configuration (e.g. macOS arm64, DEBUG, Swift 6.x) recorded in `Metadata`; document that other-branch symbols are absent from the semantic path.
- Fixture corpus avoids `#if` unless the test pins the config; fallback path must apply the same canonical branch-selection policy (pick the canonical branch) rather than indexing all branches.

**Warning signs:**
Snapshot diffs between local (arm64) and CI (other arch) machines; symbols appearing/disappearing between runs.

**Phase to address:**
Phase 1 (policy decision), enforced Phase 2–3 fixtures.

---

### Pitfall 12: Snapshot/golden flakiness from LSP nondeterminism and toolchain drift

**What goes wrong:**
Semantic-path golden tests churn: LSP response ordering is nondeterministic, background-indexing timing races the test, and Xcode/Swift versions differ between dev machines and CI runners (symbol tables differ slightly per toolchain). The repo's snapshot infra (`scip snapshot`, `testutil`, `-update-snapshots`) then produces diffs unrelated to actual changes — teams start blind-updating snapshots, destroying their value.

**Why it happens:**
Reprolang's golden model works because tree-sitter parsing is deterministic and toolchain-independent. SourceKit-LSP is neither.

**How to avoid:**
- Canonicalize + sort everything before emission (`CanonicalizeSorting`/`Sort*` in the Go bindings — note: they mutate in place; a documented CONCERNS trap).
- Separate fixture classes: (a) deterministic tree-sitter fixtures — run everywhere incl. Linux CI; (b) semantic fixtures — recorded, hand-verified, regenerated only deliberately on toolchain bumps, run only on macOS runners with pinned Xcode version.
- Never run semantic snapshot assertions against a racing warm-up (gate on Pitfall 1's readiness check).

**Warning signs:**
Snapshot tests flake "randomly"; contributors routinely run `-update-snapshots`; semantic snapshots pass only on the author's machine.

**Phase to address:**
Phase 3 (test architecture), hardened Phase 5 (pinned runners).

---

### Pitfall 13: Breaking the existing Go workspace / Nix / codegen conventions

**What goes wrong:**
Adding scip-swift cracks the existing scaffolding in several small, each-annoying ways: `go.work` gains a 4th module but the CI `fix` job's `dorny/paths-filter` doesn't know about its `go.mod`/`go.sum` (module bumps then break the Nix build); Nix vendor hashes not updated (`nix-update` flow); tree-sitter grammar vendoring drifts (generated `parser.c` not regenerated with the pinned ABI); `version.txt` parity broken because `ToolInfo.version` was hard-coded instead of derived at build; scip-swift's test binary panics on flag re-registration because `testutil` registers `-update-snapshots` at init (documented CONCERNS entry).

**Why it happens:**
The repo has many invisible contracts (workspace, paths-filter, checks.nix, release trigger on `version.txt`, generated-bindings parity). None fails loudly at design time.

**How to avoid:**
- Copy the reprolang module structure exactly (own `go.mod`, `replace` to bindings, `testdata/{snapshots,test_cmd}`), and update `ci.yaml`'s filter + `checks.nix` in the same PR that adds the module.
- Derive `ToolInfo.version` from `version.txt` at build time (Nix or ldflags).
- If scip-swift needs testutil in its own binary, fix the flag-registration issue or use a private `flag.FlagSet`.

**Warning signs:**
Red CI on unrelated PRs after module bumps; `git diff --exit-code` regen checks failing; `nix flake check` failing on vendor hash mismatches.

**Phase to address:**
Phase 1 (module skeleton wired into workspace/CI correctly) — continuously enforced Phase 5.

---

### Pitfall 14: macOS CI reality — x86_64 is a dead end; repo CI is Linux-only today

**What goes wrong:**
The semantic path needs macOS + Xcode, but this repo's CI runs **Linux x86_64 only** (verified in `.github/workflows/ci.yaml`; Nix checks are `x86_64-linux`). Meanwhile GitHub retired `macos-13` — the last Intel image — in December 2025; Intel is available only via a transitional macOS-15 Intel image until ~Fall 2027. Planning "macOS arm64 + x86_64 CI" naively means either impossible (Linux can't run SourceKit-LSP semantics) or ephemeral (Intel images disappearing mid-project).

**Why it happens:**
GitHub's runner fleet consolidated on Apple Silicon; the project's dual-arch requirement predates awareness of that timeline. Also, macOS runners cost ~10x Linux minutes and lack useful caches by default.

**How to avoid:**
- New dedicated macOS job (arm64, pinned Xcode) running semantic-path tests only; Linux CI keeps running everything else (tree-sitter fallback, lint, snapshots of the deterministic path).
- For x86_64: build via cross-compile/arm64-runner where possible, run the *semantic* e2e on an Intel transitional image or self-hosted runner, and mark x86_64 as best-effort tier with an explicit deprecation note (Fall 2027 cliff).
- Decide Rosetta acceptability for tests (index content should be arch-independent — but see Pitfall 11's `#if arch` universe).

**Warning signs:**
Roadmap assuming `macos-latest` covers both arches; CI minutes exploding; x86_64 tests silently skipped.

**Phase to address:**
Phase 5 (CI/packaging) — with the tier decision made in Phase 1 planning.

---

### Pitfall 15: Memory blowup on large SwiftUI codebases

**What goes wrong:**
Real SwiftUI codebases generate enormous occurrence counts (deeply nested builders, result-builder chains where nearly every token is a reference). Building the whole `scip.Index` in memory before writing — the pattern the CLI itself uses and documents as a 1–2 GB comfort limit — OOMs or crawls on big repositories. Also: accidentally embedding `Document.text` (proto says indexers should NOT include it by default), which doubles-to-triples index size.

**Why it happens:**
Small fixtures mask quadratic/unbounded structures; per-symbol caching absent (see Performance Traps); the easy implementation path (accumulate then marshal once) matches the toy reprolang precedent but not the stated scale goal ("real Swift projects").

**How to avoid:**
- Stream documents: emit + marshal per-document (or shard-then-merge like scip-clang's driver/worker design, which exists precisely because single-process indexes OOM'd).
- Never set `Document.text` except in dedicated unit fixtures.
- Memory budget test: index a large corpus (clone a real open-source SwiftUI app) under a heap cap in CI.

**Warning signs:**
RSS grows linearly with occurrences even after emission; CI OOM kills; `scip stats`/`lint` on scip-swift's own output exceeding the documented CLI limits.

**Phase to address:**
Phase 3 (emit-as-you-go architecture), verified Phase 6 (e2e on a real large project).

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Heuristic tree-sitter def/ref shipped as "good enough" default | Fallback works day one | Wrong-definition bugs erode trust in jarvis; hard to walk back | Only with explicit mode signaling (Pitfall 6) |
| Two independent symbol-namers (one per path) | Ship each path faster | Hybrid duplicates/broken dedup downstream; lint churn | Never — share the namer from Phase 1 |
| Skipping the ParseSymbol fix, ASCII-mangling Swift identifiers instead | Avoids touching shared bindings | Real symbol names corrupted; the panic remains for everyone; upstream divergence | Never for mangling; the fix is one line + tests |
| Hard-coded `ToolInfo.version` | One less build hook | Release/version.txt parity breaks; jarvis can't tell index versions apart | Never — derive from version.txt |
| Storing LSP UTF-16 positions "temporarily" | Saves conversion code | Position_encoding mismatch with repo tooling; corrupted snapshots on emoji | Never — convert at the boundary |
| Whole-index-in-memory emission | Matches reprolang precedent | OOM at exactly the scale the project targets | Prototyping only (Phase 2), not semantic path |
| Recording semantic golden fixtures ad hoc on dev machines | Fast fixture creation | Unreproducible churn (Pitfall 12); team blind-updates snapshots | Only with pinned-toolchain regeneration procedure |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| SourceKit-LSP | Assuming queries work without a warm index/build; assuming background indexing covers Xcode projects | Readiness gate + build/warm-up; SwiftPM-first; Swift 6.1+ requirement enforced |
| SourceKit-LSP | Long-lived single server instance with no watchdog | Watchdog timeouts, crash-respawn with file re-open, bounded request concurrency |
| SourceKit-LSP (Xcode projects) | Implicit `xcodebuild` scheme discovery in CI | Explicit `--scheme` flag; cached, timeout-guarded discovery; documented warm-up build |
| xcodebuild toolchain | Mixing Xcode's sourcekit-lsp with a different swift toolchain on PATH | Pin `xcode-select`/toolchain; record toolchain in ToolInfo/Metadata; verify in CI |
| tree-sitter-swift | Unpinned grammar; ignoring ERROR nodes | Pin version; vendor parser.c via the repo's generation script; count and warn on ERROR nodes |
| index store (if adopted) | Reading `.build/index/store` while a build writes it (races, partial records) | Index only after build completes; or consume via indexstore-db which handles incremental state |
| Go workspace / CI fix job | New module invisible to `dorny/paths-filter` and Nix vendor-hash flow | Update ci.yaml filter + checks.nix in the same PR as the module (Pitfall 13) |
| `scip lint`/`snapshot`/`stats` | Assuming they're robust to whatever the indexer emits | They panic on multi-byte symbols, unchecked ranges, metadata-less indexes (CONCERNS); test the CLI against every scip-swift fixture in CI |
| jarvis (consumer) | Treating all SCIP indexes as equal quality | Surface mode/toolchain from Metadata; smoke-test jarvis tools against fixtures in e2e phase |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Per-symbol LSP references queries | Runtime hours; server churn | documentSymbol pass + per-unique-def queries; or index-store bulk read | ~500+ files |
| No cross-run caching (re-index everything each run) | Every jarvis re-index is O(full project) | Persist index store/build dir; incremental re-index of changed files only | Second run onward |
| Whole-index accumulation in memory | OOM / GC thrash | Stream per-document; shard-merge at scale (scip-clang pattern) | ~10k occurrences/file × many files; large SwiftUI repos |
| xcodebuild discovery on every run | Minutes of dead time before indexing starts | Cache scheme/build-settings; explicit scheme | Any Xcode project |
| Unfiltered SDK occurrences | Index 10–100x source size | project-file-only occurrences; minimal external symbols | First real project |
| Background-index warm-up in CI without cache | CI minutes dominated by 2–3x-build-time indexing | Cache `.build` + index dirs keyed on lockfile hash | Fresh CI runners |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Indexing an untrusted Swift repo executes code (SwiftPM plugins run arbitrary code during resolution/build; Xcode build phases run scripts) | Code execution on the indexing machine when jarvis indexes a cloned repo | Run indexing in a sandbox/container; never auto-resolve/build untrusted packages; document the trust boundary |
| SourceKit-LSP spawns xcodebuild with repo-controlled project files | Malicious project/scheme definitions steer process execution | Treat `--scheme` input as untrusted; prefer SwiftPM path for untrusted sources |
| Feeding the emitted index into `scip snapshot`/`scip test` (path traversal via `RelativePath`, destructive `os.RemoveAll --to`) — both documented in CONCERNS | Arbitrary file write/read during CI verification of generated indexes | Validate `RelativePath` stays under root before running snapshot steps; never pass unvalidated `--to` |
| Streaming-parse length-prefix allocation bug (CONCERNS) if huge indexes get piped through CLI tools | Memory-exhaustion DoS on CI | Cap allocation or grow incrementally when the fix lands; avoid piping untrusted indexes |

## UX Pitfalls

*(Users here are jarvis end users + developers running scip-swift locally.)*

| Pitfall | User Impact | Better Approach |
|---------|-------------|------------------|
| Silent fallback to syntax-only results (Pitfall 6) | Confidently wrong navigation; trust loss | Mode visible in CLI output + index metadata |
| No actionable error when build/toolchain missing | "It indexed but everything is missing" mysteries | Detect toolchain/build state up front; explain exactly what to run (e.g. `swift build` first) |
| Misleading lint noise: duplicate lint output not deduplicated (`Unique()` unwired, CONCERNS) and swapped duplicate-symbol messages | Wall of confusing warnings hides real errors | Fix/wire dedup before first big lint runs; correct the swapped message pair |
| `scip stats` reports `linesOfCode: 0` (verified bug) | Size/coverage metrics look broken, decisions made on bad data | Pass `--project-root`; fix the bug when touching stats |
| Index run with no progress/ETA (LSP warm-up + harvesting is slow) | Users kill the process mid-run | Progress reporting per phase (warm-up, harvest, emit, verify) |

## "Looks Done But Isn't" Checklist

- [ ] **Semantic path:** "definitions work" on demo fixtures — but only same-file; verify cross-module refs on a multi-target SwiftPM package
- [ ] **Warm index:** fresh-clone CI run (no caches) produces the same index as a warm dev machine — verify in a clean-runner job
- [ ] **Fallback path:** exercised in CI on Linux, not just present in code — verify a Linux job runs its golden tests
- [ ] **Symbol parity:** tree-sitter and semantic paths emit identical symbol strings for shared fixtures — verify dual-path golden diff is empty
- [ ] **Unicode:** fixture with emoji/CJK identifiers and string literals passes lint + snapshot without panic — verify (ParseSymbol bug, Pitfall 4)
- [ ] **Position encoding:** every document sets `position_encoding`; snapshot ranges land on exact symbol boundaries on non-ASCII lines — verify (Pitfall 3)
- [ ] **is_definition roles:** occurrences with definition roles exist for all emitted symbols; `forwardDefIsDefinitionError` clean in lint — verify
- [ ] **Relationships:** callHierarchy/typeHierarchy answer through jarvis on a protocol-heavy fixture, not just class inheritance — verify (Pitfall 10)
- [ ] **Scope decisions:** access-level (private?) and test-target (Tests/?) symbols — deliberately included/excluded and documented; not accidental — verify by inspecting fixtures
- [ ] **Reproducibility:** same commit indexed twice on the same runner produces byte-identical (or canonically identical) output — verify
- [ ] **Packaging:** binary + version derived from version.txt; release workflow knows the new artifact; jarvis-intelligence README claim actually true on both arches — verify
- [ ] **Repo health:** existing Go workspace, Nix checks, bindings codegen, reprolang tests all still green — verify full `nix flake check` + CI

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Cold-index wrong results shipped (Pitfall 1) | MEDIUM | Add readiness gate + re-index with warm-up; audit affected indexes via stored ToolInfo |
| Symbol scheme drift (Pitfall 5) | HIGH | Version the scheme in ToolInfo; write a migration pass (remap old symbol strings); re-index all users |
| Position-encoding corruption (Pitfall 3) | LOW | Re-emit from source (positions recomputed); no data loss since sources are authoritative |
| ParseSymbol panic (Pitfall 4) | LOW | One-line guard fix + fuzz test; re-run lint on existing indexes |
| Silent degradation trust damage (Pitfall 6) | MEDIUM | Add mode metadata retroactively; re-index; communicate accuracy delta openly |
| Snapshot-suite rot (Pitfall 12) | MEDIUM | Re-pin toolchain; regenerate all semantic fixtures in one audited commit; add regeneration runbook |
| CI/workspace breakage (Pitfall 13) | LOW | Revert module wiring; re-add with ci.yaml/checks.nix updates atomically |
| OOM on large repo (Pitfall 15) | MEDIUM | Switch to streaming/sharded emission; index-merge step; keep symbol scheme unchanged |

## Pitfall-to-Phase Mapping

*(Phases refer to the anticipated roadmap: P1 foundations/skeleton + spike, P2 tree-sitter fallback, P3 SourceKit-LSP semantic path, P4 hierarchies/relationships, P5 CI/Nix/packaging, P6 jarvis e2e validation.)*

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 1. Cold-index wrong results | P3 | Clean-runner CI job (no caches) yields full cross-module index |
| 2. LSP round-trip storms | P1 (spike) / P3 | Perf-budget fixture meets time cap |
| 3. UTF-16/UTF-8 mismatch | P1 / P2 | Emoji fixture snapshot exact-boundary check |
| 4. ParseSymbol Unicode panic | P1 | Fuzz test + regression fixture green |
| 5. Symbol scheme instability | P1 | Dual-path symbol-parity golden test empty diff |
| 6. Silent fallback | P2 | Mode present in ToolInfo/metadata; `--strict-semantic` tested |
| 7. Grammar gaps | P2 | Zero-ERROR-node corpus assertion; grammar-bump CI job |
| 8. xcodebuild fragility | P3 | `--scheme` flag flow documented + timeout-guarded |
| 9. SDK occurrence blowup | P3 / P5 | Index-bytes-per-LOC budget in CI |
| 10. Witness/generic mismodeling | P1 / P4 | Capability-inventory doc; protocol-heavy fixture passes jarvis hierarchy queries |
| 11. `#if` universes | P1 / P3 | Canonical-config policy doc; arch-stable snapshots |
| 12. Snapshot flakiness | P3 / P5 | Pinned Xcode runners; deterministic vs recorded fixture split |
| 13. Workspace/CI breakage | P1 / P5 | Full `nix flake check` + CI green in module-introduction PR |
| 14. macOS runner reality | P1 / P5 | arm64 macOS job green; x86_64 tier decision documented |
| 15. Memory blowup | P3 / P6 | Heap-capped large-corpus CI test |

## Sources

- SourceKit-LSP background indexing doc (SwiftPM-only, 6.1+ default, caveats): https://github.com/swiftlang/sourcekit-lsp/blob/main/Documentation/Enable%20Experimental%20Background%20Indexing.md
- Swift 6.1 release (background indexing default): https://swift.org/blog/swift-6.1-released/
- Background indexing slowness (2–5x Xcode): https://github.com/swiftlang/sourcekit-lsp/issues/1837
- Go-to-def not working without build (Swift 5.9): https://github.com/swiftlang/sourcekit-lsp/issues/1626
- Swift Forums — background indexing usage/limits: https://forums.swift.org/t/how-to-use-background-indexing-in-sourcekit-lsp/79497
- SR-9311 encodedOffset/UTF-16 bug: https://github.com/apple/swift-issues/issues/9311
- LSP position encoding history: https://github.com/microsoft/language-server-protocol/issues/872
- tree-sitter-swift macros gap: https://github.com/alex-pinkus/tree-sitter-swift/issues/438 ; grammar repo: https://github.com/alex-pinkus/tree-sitter-swift
- Xcode-project LSP setup pain (incl. JSON-RPC -32603): https://forums.swift.org/t/need-setup-help-for-sourcekit-lsp-with-an-xcode-project/86000
- xcodebuild slowness/provisioning queries: https://dimillian.medium.com/why-is-xcodebuild-slower-than-the-xcode-gui-38f3d7b0c0bc ; unshared schemes: https://stackoverflow.com/questions/14368938/xcodebuild-says-does-not-contain-scheme
- macOS 13 (last Intel) runner retirement: https://github.blog/changelog/2025-09-19-github-actions-macos-13-runner-image-is-closing-down/ ; Intel transitional images: https://github.com/actions/runner-images/issues/13045
- scip-clang design (driver/worker shards, index-store alternatives evaluated): https://github.com/sourcegraph/scip-clang/blob/main/docs/Design.md ; approach evaluation: https://github.com/sourcegraph/sourcegraph-public-snapshot/issues/42280 ; merge bottleneck: https://github.com/sourcegraph/scip-clang/issues/139
- Index-store consumers: https://github.com/swiftlang/indexstore-db ; https://github.com/MobileNativeFoundation/swift-index-store ; kateinoigakukun/swift-indexstore (https://swiftpackageindex.com/kateinoigakukun)
- clangd SCIP/index-store discussion: https://github.com/clangd/clangd/issues/1340
- Repo-internal (verified by inspection, 2026-08-16): `.planning/codebase/CONCERNS.md` (ParseSymbol panic, RangeText unchecked indexing, stats LOC bug, lint dedup unwired, testutil flag, dual-range encoding); `cmd/scip/lint.go` (check taxonomy); `bindings/go/scip/source_file.go` (byte-sliced ranges); `scip.proto` (PositionEncoding, Language.Swift = 2, Document.text guidance); `.github/workflows/ci.yaml` (Linux-only checks; paths-filter); `go.work`; `reprolang/` (module layout, tree-sitter vendoring, namer precedent).

---
*Pitfalls research for: hybrid Swift SCIP indexer (SourceKit-LSP + tree-sitter) in the scip monorepo*
*Researched: 2026-08-16*
