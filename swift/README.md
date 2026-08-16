# scip-swift

scip-swift is an in-repo Swift indexer that emits SCIP indexes. It exists so
that jarvis (a local-first code-intelligence MCP server) can answer structural
queries — goToDefinition, findReferences, callHierarchy, typeHierarchy,
documentSymbols — on Swift codebases.

The module is a work in progress: this phase ships the symbol scheme
foundations (the shared namer in `internal/symbol`); the indexer itself lands
in later phases.

## Symbol scheme

Every scip-swift global symbol is frozen in `internal/symbol` (see
`scheme.go` for the normative doc, `namer_test.go` for the golden table):

```text
scip-swift swiftpm <Module> . <descriptors>       SwiftPM target modules
scip-swift swift <Module> <swift-version> <descriptors>   system/SDK modules
```

Descriptors chain without separators, one per ancestry node: module
namespace `M/`, types `Shape#`, methods `area().` (with the `(+N)`
disambiguator for overloads), terms `origin.`, type parameters `[T]`,
parameters `(x)`, macros `Preview!`. Backtick escaping of non-simple
identifiers (`==`, `🚀`) is the bindings formatter's job — the namer never
escapes by hand.

Frozen rules (v1; changing them invalidates stored indexes):

- **Extension attribution** — a member of `extension Foo`, whether declared
  in the same file, another file, or retroactively in a module that does not
  own `Foo`, produces exactly the symbol it would have if declared inside
  the type body: the extended type's owner-module package and `Foo#` path.
  A retroactive method-family collision from a second declaring module
  disambiguates as `Foo#name(+1)`.
- **Determinism** — overload indices and local ordinals derive from source
  declaration order only; index 0 renders no disambiguator, index N renders
  `(+N)`.
- **Canonical `#if` policy** — the semantic path indexes the canonical build
  configuration (macOS arm64, host Swift version, `DEBUG`); the fallback
  selects the same single canonical branch, never all branches.
- **Dual-path identity** — the SourceKit-LSP semantic path and the
  tree-sitter fallback emit identical symbol strings for the same
  declaration; the fallback never merges output with semantic output.
- **Known limitation** — the SCIP grammar allows disambiguators only on
  Method descriptors, so retroactive Term-family (property/let/case)
  same-name collisions across declaring modules cannot carry `(+N)`. If
  real-world cases appear, Phase 3 resolves them with USR evidence
  (recorded fallback: synthetic simple-identifier suffix).

## Running tests

From the repo root (workspace mode, covers all modules):

```bash
go build ./... && go test ./...
```

From this module, standalone:

```bash
GOWORK=off go test ./...
```

## Runtime prerequisites

Swift is a runtime-only prerequisite of scip-swift, never a build dependency:

- Swift >= 6.1 with sourcekit-lsp (bundled with Xcode), discovered via
  `xcrun -f sourcekit-lsp` — never a hardcoded path.
- The Go module, its tests, and the Nix checks build without any Swift
  toolchain present.

## Repo checks

The module is wired into the repository's Nix check matrix:

```bash
nix flake check
```
