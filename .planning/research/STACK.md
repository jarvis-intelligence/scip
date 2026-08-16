# Stack Research

**Domain:** Source-code indexer (SCIP emitter) for Swift — hybrid SourceKit-LSP (primary semantics) + tree-sitter (fallback), living inside the existing scip monorepo
**Researched:** 2026-08-16
**Confidence:** HIGH overall (language/architecture/build decisions verified against local toolchain + current upstream releases; per-item levels below)

Local ground truth used (verified on this machine, 2026-08-16): Xcode 26.3 (Build 17C529), Apple Swift 6.2.4 (swiftlang-6.2.4.1.4), `sourcekit-lsp` present at `/usr/bin/sourcekit-lsp` and inside the Xcode toolchain; repo pins `go 1.25.0`, `github.com/tree-sitter/go-tree-sitter v0.25.0` (reprolang), nixpkgs `nixos-26.05`, `cmd/scip/version.txt` = `0.9.0`.

---

## Primary Recommendation (Opinionated Summary)

**Write scip-swift in Go, as a new in-repo Go module (`swift/`) following the reprolang precedent. Drive `sourcekit-lsp` (bundled with Xcode/swift.org toolchains, Swift 6.1+) as a subprocess over LSP/stdio for definitions/references/document symbols/hierarchies; vendor the tree-sitter-swift grammar (0.7.3 from alex-pinkus, parser regenerated at ABI 14) behind `go-tree-sitter v0.25.0` for the fallback path; emit `.scip` directly with the in-repo `bindings/go/scip` Protobuf module; validate with the in-repo `scip lint` / `scip snapshot` / `scip test` CLI; ship as `scip-swift-darwin-{arm64,amd64}.tar.gz` release assets from the existing release pipeline, version-synced to `cmd/scip/version.txt`.**

Confidence: **HIGH**. The one genuinely open question (MEDIUM) is the exact orchestration of the Xcode-scheme path (see "Xcode projects" below).

---

## Architecture Decision: Implementation Language — Go, not Swift

**Confidence: HIGH**

The scip-* ecosystem convention (scip-java in Java, scip-python in Python, scip-typescript in TypeScript) suggests writing the indexer in the target language — Swift here. **Reject that convention for this repo.** Reasons, in order of weight:

1. **SourceKit-LSP is consumed over LSP, not linked.** We do not embed `sourcekitd` or SourceKitLSPKit; we spawn `sourcekit-lsp` and speak JSON-RPC over stdio. That is fully language-agnostic — Swift offers zero advantage for the primary semantics path, and its unique libraries (SwiftSyntax, IndexStoreDB) are exactly the parts we deliberately do NOT depend on.
2. **The repo's validated patterns are Go and directly reusable.** `reprolang/` already demonstrates every piece of the fallback path: an in-repo Go module with a `replace` directive to `../bindings/go/scip`, a vendored tree-sitter parser behind cgo (`reprolang/grammar/binding.go`), snapshot testdata driven by autogold, and integration with `cmd/scip` subcommands. `go-tree-sitter v0.25.0` is already in `go.work.sum`. A Swift implementation gets none of this for free.
3. **SCIP emission is solved in Go.** `bindings/go/scip` is the published, in-repo Protobuf module (proto3, `google.golang.org/protobuf v1.36.12`). Writing Protobuf from Swift would mean adding a SwiftProtobuf (1.38.1) codegen+build step for no benefit.
4. **Swift cannot fit the repo's Nix/check matrix.** nixpkgs' Swift toolchain lags badly (5.10-era, reported not even building cleanly on NixOS; Darwin support is a long-open gap — nixpkgs issue #37473), while the flake (`flake.nix`) builds Go modules with `buildGoModule`. A Swift implementation forces the "CI exception" the project constraints want avoided. A Go implementation needs **no Swift toolchain at build time at all** — SourceKit-LSP is a *runtime* dependency discovered on the indexing machine via `xcrun -f sourcekit-lsp`.
5. **Distribution**: Go cross-compiles and the release workflow (`release.yaml`) already builds darwin arm64+amd64 tarballs from a Go matrix. A SwiftPM-executable distribution would add toolchain version coupling jarvis doesn't want.

When Swift *would* be the right choice (documented for future re-evaluation): if v2 needs direct `IndexStoreDB`/`.index` store reading (Periphery-style, impossible from Go — it's a Swift/C++ package), or direct SwiftSyntax tree access without an LSP hop. Neither is in v1 scope.

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended (confidence) |
|------------|---------|---------|-----------------|
| **Go** | 1.25 (matches repo `go 1.25.0`) | Implementation language of the indexer | Repo-native; reuses go.work, bindings/go/scip, CI, Nix build, release matrix; SourceKit-LSP is consumed via stdio so language-agnostic (HIGH) |
| **SourceKit-LSP** (subprocess) | Bundled with toolchains; require **Swift ≥ 6.1**, current local: 6.2.4 / Xcode 26.3 | Primary semantics: defs, refs, document symbols, call/type hierarchies | Compiler-grade accuracy via the only first-party Swift LSP server; background indexing **on by default since Swift 6.1** (no manual build needed first); bundled with Xcode *and* swift.org toolchains so it's already on every target machine (HIGH) |
| **go-tree-sitter** | v0.25.0 (latest; Feb 2025 — still latest as of 2026-08) | Binding layer for tree-sitter fallback path | Exact library reprolang already pins in this repo; embeds tree-sitter 0.25 runtime (ABI 13–15) which accepts our regenerated Swift parser (HIGH) |
| **tree-sitter-swift grammar** | **0.7.3** (alex-pinkus/tree-sitter-swift, released 2026-06-01; repo pushed 2026-08-16) | Swift syntax tree for fallback indexing | The canonical, actively maintained Swift grammar; vendor grammar sources and regenerate `parser.c` with `tree-sitter generate --abi 14` — identical pinning strategy to `reprolang/generate-tree-sitter-parser.sh` (HIGH on approach; the grammar repo does not commit parser.c, we generate it) |
| **bindings/go/scip** | in-repo (proto3; google.golang.org/protobuf v1.36.x) | Emit the `.scip` index | This IS the protocol module, in-repo, already consumed by reprolang via replace directive. Precedent: every scip-* indexer uses its language's protobuf runtime over `scip.proto` (scip-java → Java protobuf). Never shell out to emit protobuf (HIGH) |
| **github.com/sourcegraph/jsonrpc2** | **v0.2.2** (released 2026-07-31 — actively maintained) | JSON-RPC 2.0 wire layer for the LSP client | Battle-tested (gopls-lineage), allocation-conscious, stdio streams. Pair with a small hand-written set of LSP request/response structs (~10 requests) instead of a full protocol SDK (HIGH on approach, MEDIUM on library vs. hand-rolled) |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| xcode-build-server (SolaWing) | v1.3.0 (2026-01-05; `brew install xcode-build-server`) | BSP layer making SourceKit-LSP work with `.xcodeproj`/`.xcworkspace` by extracting compiler args + index-store paths from `xcodebuild` | Only for the `--xcode-scheme` path (jarvis's promised "Xcode scheme flag option"). SourceKit-LSP has no native Xcode workspace type (verified: `--default-workspace-type` accepts only `swiftPM\|compilationDatabase\|buildServer`); xcode-build-server is the ecosystem-standard workaround (MEDIUM confidence on exact orchestration; HIGH that this is the standard mechanism) |
| hexops/autogold/v2 | v2.3.1 (already in repo) | Golden snapshot tests for emitted indexes | All fixture tests — mirror `reprolang/repro/snapshot_test.go` + `scip snapshot` |
| stretchr/testify | v1.11.1 (already in repo) | Assertions | Standard; already in go.work |
| `xcrun` / `xcodebuild` / `swift build` (system tools) | Xcode 26.x / Swift 6.1+ | Discover `sourcekit-lsp` (`xcrun -f sourcekit-lsp`, PATH fallback for swift.org toolchains); build/prepare SwiftPM & Xcode workspaces before indexing | Always at runtime — document as environment prerequisites |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| Xcode 26.x (local 26.3) + Swift 6.1+ toolchain | Runtime engine for SourceKit-LSP integration tests | Integration tests must detect the toolchain and skip gracefully where absent (Linux/Nix CI) |
| GitHub Actions `macos-15` (arm64) + `macos-15-intel` (x86_64) | Native builds + LSP integration tests for both arches | macos-13 retired Dec 2025; `macos-15-intel` is the transitional Intel label, slated for ~Fall 2027. macos-latest already builds the scip CLI today |
| tree-sitter CLI (Nix devShell already ships `tree-sitter`; CLI 0.26.11 current) | Regenerate the vendored Swift parser | Always `--abi 14` (pinned, like reprolang) — do not let the CLI's default ABI (15) drift |
| `scip` CLI (in-repo `cmd/scip`) | `scip lint`, `scip snapshot`, `scip stats`, `scip test` on fixtures | The definition of done for emitted indexes |
| Nix flake (existing) | Reproducible dev env; optional `packages.scip-swift` | Go-only build fits `buildGoModule`; Swift toolchain stays OUT of the flake (it's a runtime dep, like a database) |

## LSP Integration Contract (what SourceKit-LSP gives us)

Verified against SourceKit-LSP docs/repo and LSP 3.17; request names standard:

| Capability | LSP requests | SCIP mapping |
|------------|--------------|--------------|
| Definitions | `textDocument/definition` | Occurrence role `Definition`; symbol per `SymbolInformation` |
| References | `textDocument/references` (incl. `includeDeclaration`) | Occurrences role `Reference` + `symbol_roles`; `references_symbols` |
| Document symbols | `textDocument/documentSymbol` (hierarchical) | `DocumentSymbol` information fields; document structure |
| Call hierarchy | `textDocument/prepareCallHierarchy` → `callHierarchy/incomingCalls` / `outgoingCalls` | Call edges are encoded as reference occurrences (call sites referencing the callee symbol); jarvis derives callHierarchy from refs. (Confirmed supported — release notes mention call-hierarchy name qualification) |
| Type hierarchy | `textDocument/prepareTypeHierarchy` → `typeHierarchy/supertypes` / `subtypes` | `Symbol.relationships` (`is_implementation` for protocol conformance; `is_type_definition`; inheritance per scip.proto relationship semantics). (Confirmed supported since LSP 3.17-era SourceKit-LSP) |

Headless-client requirements (the non-editor caveats that matter):

1. **Initialize** with `rootUri`/`workspaceFolders` + client capabilities declaring `window.workDoneProgress`. Send `initialized`, then `workspace/didChangeConfiguration` if needed.
2. **Wait for background indexing.** Default-on since Swift 6.1; report via `$/progress` (token from `window/workStatus`-style server requests). A naive fire-immediately query loop gets empty results — the indexer must drain progress until the index is "prepared" before querying, with a configurable timeout. First index costs ~2–3x a build.
3. **Known staleness bug**: symbols whose USR changes while staying API-compatible (e.g. added defaulted parameter) lose references until files are touched — handle by re-`didOpen`/edit round-trips or documenting as a limitation (upstream issues #1269/#1271 lineage).
4. **Background indexing covers SwiftPM workspaces only.** Compilation-database and buildServer workspaces rely on build-produced index data. This is the fork in the road below.

Workspace matrix (verified locally via `sourcekit-lsp --help`, Xcode 26.3): `swiftPM` | `compilationDatabase` | `buildServer`.

- **SwiftPM projects** (primary v1 target): open the package root as a workspace folder; background indexing handles the rest. If packages need custom flags, forward `-Xswiftc` etc. via CLI args.
- **Xcode projects** (`--xcode-scheme` path): SourceKit-LSP does not open `.xcodeproj`/`.xcworkspace` (long-standing issue #730). Standard solution: run an xcodebuild for the scheme (index store lands in DerivedData), and bridge via **xcode-build-server** (BSP `buildServer` workspace type): it parses xcodebuild logs into compiler arguments and points SourceKit-LSP at the DerivedData index store. Prescribe: `scip-swift --xcode-scheme <name>` orchestrates `xcodebuild` + xcode-build-server (runtime dependency, detected with a `brew install xcode-build-server` hint), then drives SourceKit-LSP exactly as the SwiftPM path. *(MEDIUM confidence on orchestration details — spike in phase 1 of the milestone.)*

## Installation / Repo Integration

No new system-level build requirements. Concretely:

```bash
# New module (reprolang pattern)
mkdir swift && cd swift
go mod init github.com/scip-code/scip/swift   # replace-dir → ../bindings/go/scip
go get github.com/tree-sitter/go-tree-sitter@v0.25.0
go get github.com/sourcegraph/jsonrpc2@v0.2.2
# add "swift" to go.work; add swift/go.{mod,sum} to ci.yaml fix-job module list

# Vendor the grammar (one-time; pin CLI + ABI like reprolang)
#   fetch alex-pinkus/tree-sitter-swift @ 0.7.3 grammar sources → swift/grammar/
#   tree-sitter generate --abi 14 --output grammar   (+ committed parser.c, node-types.json)
#   cgo binding: copy reprolang/grammar/binding.go pattern (tree_sitter_swift())

# Runtime prerequisites on the indexing machine (NOT build-time):
xcode-select --install … or full Xcode; sourcekit-lsp discovered via `xcrun -f sourcekit-lsp`
brew install xcode-build-server        # only for --xcode-scheme
```

Integration points with the existing repo:

- **go.work**: add `swift` as the 4th module. Non-breaking.
- **flake.nix**: optional `packages.scip-swift` via `buildGoModule` (subPackages `cmd/scip-swift`; needs vendorHash upkeep like the `scip` package). The tree-sitter cgo build works under Nix (clang from stdenv). Keep Swift/Xcode **out** of the flake — `checks.nix` gets a Go-test job that skips LSP tests when no `sourcekit-lsp` is on PATH (`xcrun` probe), so Linux/Nix CI still runs tree-sitter fallback + emission + lint tests.
- **CI (`ci.yaml`)**: unit/snapshot tests everywhere; add a macOS integration job (`macos-15` + `macos-15-intel`) running the LSP-dependent tests with the preinstalled Xcode.
- **Release (`release.yaml`)**: extend `build-go-binaries` matrix with `scip-swift-darwin-arm64` (macos-15) and `scip-swift-darwin-amd64` (macos-15-intel). NOTE: unlike the `scip` CLI (`CGO_ENABLED=0`), scip-swift needs **CGO_ENABLED=1** for go-tree-sitter, so the existing cross-compile-from-arm64 trick (`GOARCH=amd64` on macos-latest) needs `CC="clang -arch x86_64"` — prefer the native `macos-15-intel` runner instead. Trigger stays keyed on `cmd/scip/version.txt` (add `swift/**` to the paths filter is unnecessary; a version bump releases everything).
- **cmd/scip**: no changes required; fixtures are validated by invoking the existing CLI.

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| Go indexer driving `sourcekit-lsp` subprocess | Swift CLI (SwiftPM executable) driving SourceKit-LSP, SwiftProtobuf 1.38.1 emission, SwiftSyntax 602.0.x fallback | If we later need direct IndexStoreDB/.index-store access or sourcekitd in-process (v2+ accuracy/perf work); requires solving Swift-in-Nix/CI (currently broken: nixpkgs Swift = 5.10-era, darwin gap #37473) |
| SourceKit-LSP via LSP | Read `.index` store directly (IndexStoreDB, Periphery-style) | More robust for whole-program indexing, but IndexStoreDB is a Swift/C++ package — unusable from Go, forces Swift implementation and Xcode-build coupling; revisit only with the Swift alternative above |
| SourceKit-LSP primary | tree-sitter-only indexer (like reprolang is for reprolang) | Syntax-level defs/refs only — no type info, no hierarchy resolution, wrong for the accuracy bar jarvis promises; correctly demoted to fallback |
| go-tree-sitter + vendored generated parser | tree-sitter-languagepack (v1.9, 306 grammars) / smacker fork | Only if we want zero vendored C and a huge dependency; carrying one grammar's parser.c (reprolang pattern) is lighter and ABI-pinned |
| sourcegraph/jsonrpc2 v0.2.2 + hand-written LSP structs | go.lsp.dev/protocol (typed full LSP) or its go-language-server.org fork | If the LSP surface grows large; today go.lsp.dev is stale (2022-era) and we need ~10 request types — a thin client is ~300 LoC and zero stale deps |
| GitHub release tarball assets | Homebrew tap / SwiftPM binary distribution / mint | Homebrew later as convenience (jarvis installs programmatically; it fetches `scip-$OS-$ARCH.tar.gz`-style assets today) |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `tree-sitter/tree-sitter-swift` (org mirror) | Stale mirror, last pushed 2022-01, ships ABI 10 parser — flatly incompatible with go-tree-sitter v0.25 runtime (accepts ABI 13–15) | `alex-pinkus/tree-sitter-swift` @ 0.7.3, parser self-generated `--abi 14` |
| `go.lsp.dev/protocol` as the LSP client | Unmaintained upstream (community fork exists at go-language-server org but is not upstream-blessed); full-protocol surface we don't need | Thin client over `sourcegraph/jsonrpc2` v0.2.2 |
| Building any Swift in Nix for this project | nixpkgs Swift toolchain lags (5.10-era) and Darwin Swift-under-Nix is a multi-year open gap (#37437/#37473); would force the CI exception constraints warn about | Go + cgo; Swift toolchain purely a runtime prerequisite |
| Parsing/emitting `.scip` as JSON or shelling out to `scip print` round-trips | Wasteful, lossy, non-canonical | `bindings/go/scip` Protobuf structs directly (reprolang precedent; scip-java precedent for "native protobuf runtime") |
| LSIF emission | Superseded by SCIP; repo IS the SCIP home | `.scip` only |
| `sourcekitd` direct JSON messages | Private, unstable Apple-internal protocol; SourceKit-LSP exists precisely to abstract it | LSP over stdio |
| Assuming x86_64 CI forever | `macos-15-intel` is transitional (Intel macOS runners retire ~Fall 2027; macos-13 already gone Dec 2025) | Native Intel builds now + documented cross-compile escape hatch (`CC="clang -arch x86_64" CGO_ENABLED=1`) and/or `lipo` universal2 assets |

## Stack Patterns by Variant

**If target is a SwiftPM package (v1 default):**
- Open package root as workspace; SourceKit-LSP `swiftPM` workspace type; background indexing (Swift 6.1+) populates the index; drain `$/progress`, then query per file.
- Because this path needs no Xcode project, it also works with a standalone swift.org toolchain.

**If target is an Xcode project/workspace (`--xcode-scheme`):**
- Orchestrate `xcodebuild` for the scheme + xcode-build-server (BSP) → SourceKit-LSP `buildServer` workspace type reading the DerivedData index store. Requires full Xcode. Degraded-fallback to tree-sitter if orchestration fails.

**If `sourcekit-lsp` is missing/old or semantic queries fail hard (fallback path):**
- tree-sitter-swift parse → syntax-level definitions (declarations) and best-effort local references; emit document symbols and occurrence structure WITHOUT cross-file resolution; mark index metadata so jarvis knows it's degraded. This path must be pure Go (runs on any OS in CI, including Nix Linux checks).

## Version Compatibility

| Component A | Compatible With | Notes |
|-------------|-----------------|-------|
| go-tree-sitter v0.25.0 (runtime ABI 13–15) | tree-sitter-swift parser generated `--abi 14` | Safe; matches reprolang's pinned ABI. Never regenerate at the CLI default (15) without bumping go-tree-sitter. Grammar repo itself dev-depends on tree-sitter-cli ^0.23 (ABI 14 era) |
| SourceKit-LSP bundled with Swift 6.1+ | Background indexing default-on | Swift 6.0 needs config `~/.sourcekit-lsp/config.json` `{"backgroundIndexing": true}`; **enforce Swift ≥ 6.1** as minimum supported environment |
| SourceKit-LSP (Swift 6.2.x / Xcode 26.x) | macOS arm64 + x86_64 hosts | macOS 26 Tahoe is the last Intel macOS; Intel supported through the v1 window (Xcode 26 runs on both). Re-verify before Fall 2027 runner retirement |
| `bindings/go/scip` (proto3, protobuf-go 1.36.x) | Go 1.25 module graph | Existing repo versions; no changes |
| sourcegraph/jsonrpc2 v0.2.2 | Go 1.25 | Pure Go, no transitive deps of note |
| xcode-build-server v1.3.0 | Xcode 26.x, SourceKit-LSP `buildServer` workspace | Runtime-only dep; Homebrew formula maintained |

## Open Questions for Roadmap (LOW/MEDIUM confidence items)

1. Exact `--xcode-scheme` orchestration via xcode-build-server vs. in-house `xcodebuild` log parsing — needs a phase-1 spike (MEDIUM).
2. Whether SourceKit-LSP reference results include enough `write`/`read` role fidelity to populate all `SymbolRole` bits worth emitting, or whether roles stay coarse in v1 (MEDIUM).
3. Symbol-scheme details for Swift (`swift package <module> <Owner>.<member>().` descriptor shapes per scip.proto conventions) — design task, not a research gap (HIGH that it fits existing schema; PROJECT.md already rules out proto changes).

## Sources

- swiftlang/sourcekit-lsp repo + README — https://github.com/swiftlang/sourcekit-lsp (features, toolchain bundling; verified 2026-08-16)
- SourceKit-LSP background indexing doc — https://github.com/swiftlang/sourcekit-lsp/blob/main/Documentation/Enable%20Experimental%20Background%20Indexing.md (default in 6.1+, SwiftPM-only limitation, 2–3x first-index cost, USR staleness) — HIGH
- Swift Forums: background indexing default — https://forums.swift.org/t/how-to-use-background-indexing-in-sourcekit-lsp/79497 — HIGH
- SourceKit-LSP issues #730 (Xcode project support), #1269/#1271 (background indexing workspace types) — https://github.com/swiftlang/sourcekit-lsp/issues/730 — HIGH
- `sourcekit-lsp --help` on Xcode 26.3 (local) — workspace types `swiftPM|compilationDatabase|buildServer`, `-Xswiftc` passthrough — HIGH (primary source)
- SolaWing/xcode-build-server — https://github.com/SolaWing/xcode-build-server + https://formulae.brew.sh/formula/xcode-build-server (v1.3.0, Jan 2026, verified via GitHub API) — HIGH on existence/role, MEDIUM on orchestration details
- alex-pinkus/tree-sitter-swift releases (0.7.3, 2026-06-01; repo pushed 2026-08-16; parser.c not committed; tree-sitter-cli ^0.23 devDep) — https://github.com/alex-pinkus/tree-sitter-swift — verified via GitHub API — HIGH
- tree-sitter/tree-sitter-swift stale mirror (pushed 2022-01-13, LANGUAGE_VERSION 10) — verified via GitHub API + raw fetch — HIGH
- go-tree-sitter v0.25.0 latest — https://github.com/tree-sitter/go-tree-sitter , https://pkg.go.dev/github.com/tree-sitter/go-tree-sitter — HIGH
- tree-sitter ABI semantics (0.25 runtime = ABI 13–15; default 15) — https://docs.rs/tree-sitter/latest/tree_sitter/constant.LANGUAGE_VERSION.html , https://lists.gnu.org/r/emacs-devel/2025-04/msg00508.html , https://github.com/tree-sitter/tree-sitter/issues/966 — HIGH
- Swift 6.2 release — https://swift.org/blog/swift-6.2-released/ ; swift-syntax 602.x — https://github.com/swiftlang/swift-syntax/releases — HIGH
- SwiftProtobuf 1.38.1 (still 1.x) — https://github.com/apple/swift-protobuf — HIGH (context for rejected Swift path)
- LSP 3.17 spec (definition/references/documentSymbol/callHierarchy/typeHierarchy) — https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/ — HIGH
- SourceKit-LSP call/type-hierarchy support — https://swiftpackageindex.com/swiftlang/sourcekit-lsp (release notes) + https://github.com/swiftlang/sourcekit-lsp/issues/1890 — MEDIUM-HIGH
- GitHub runners: macos-13 retirement — https://github.blog/changelog/2025-09-19-github-actions-macos-13-runner-image-is-closing-down/ ; macos-15-intel label — https://github.com/actions/runner-images/issues/13045 ; Intel retirement ~2027 — https://github.com/actions/runner-images/issues/13637 — HIGH
- Swift on nixpkgs lag/darwin gap — https://ryantm.github.io/nixpkgs/languages-frameworks/swift/ , https://github.com/NixOS/nixpkgs/issues/37473 — HIGH
- sourcegraph/jsonrpc2 v0.2.2 (2026-07-31) — https://github.com/sourcegraph/jsonrpc2 (verified via GitHub API) — HIGH; go.lsp.dev staleness — https://pkg.go.dev/go.lsp.dev/protocol — MEDIUM-HIGH
- scip-java protobuf-runtime precedent — https://github.com/sourcegraph/scip-java — HIGH (well-established)
- Repo-local primary sources: `flake.nix`, `go.work`, `reprolang/go.mod`, `reprolang/generate-tree-sitter-parser.sh`, `.github/workflows/{ci,release}.yaml`, `cmd/scip/version.txt`, `scip.proto` (proto3, `Relationship` fields) — HIGH

---
*Stack research for: Swift SCIP indexer (scip-swift) in the scip monorepo*
*Researched: 2026-08-16*
