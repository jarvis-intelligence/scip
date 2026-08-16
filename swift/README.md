# scip-swift

scip-swift is an in-repo Swift indexer that emits SCIP indexes. It exists so
that jarvis (a local-first code-intelligence MCP server) can answer structural
queries — goToDefinition, findReferences, callHierarchy, typeHierarchy,
documentSymbols — on Swift codebases.

The module is a work in progress: this phase ships the symbol scheme
foundations (the shared namer in `internal/symbol`); the indexer itself lands
in later phases.

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
