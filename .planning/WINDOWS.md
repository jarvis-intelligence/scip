---
schema_version: 1
open_count: 2
waived_count: 0
fixed_count: 4
total_count: 6
last_updated: 2026-08-16T13:06:01.379Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 1 | unrun-verify | .github/workflows/ci.yaml |  | nix eval/flake check gates for swift wiring not run — nix not installed on executing host (see deferred-items.md #2) | fixed |  | 2026-08-16T09:21:22.416Z | 2026-08-16T13:05:47.176Z |
| 2 | 1 | deviation | bindings/go/scip/symbol_fuzz_test.go |  | FuzzParseSymbol identity property holds for canonical spellings only — pre-existing empty-field round-trip asymmetry found by soak (see deferred-items.md #1) | open |  | 2026-08-16T09:21:22.501Z |  |
| 3 | 1 | unrun-verify | swift/README.md |  | prettier --check swift/README.md via nix develop not run — nix absent on executing host; section hand-written to .prettierrc rules (01-02 Task 3) | fixed |  | 2026-08-16T09:44:08.603Z | 2026-08-16T13:06:01.214Z |
| 4 | 1 | unrun-verify | checks.nix |  | nix build .#checks.x86_64-linux.swift paste-and-rerun flow not run for the jsonrpc2 vendorHash refresh — nix absent on host; hash derived via NAR-SHA256 pipeline (thesis-grammar serializer) validated bit-for-bit against the known go-bindings hash (01-03 Task 1) | fixed |  | 2026-08-16T10:38:56.275Z | 2026-08-16T13:06:01.293Z |
| 5 | 1 | unrun-verify | swift/spikes/2026-08-sourcekit-lsp-findings.md |  | prettier --check via nix develop not run — nix absent on host; doc hand-written to .prettierrc rules (01-03 Task 3) | fixed |  | 2026-08-16T10:38:56.377Z | 2026-08-16T13:06:01.379Z |
| 6 | 1 | deviation | checks.nix |  | latent: 01-02 promoted go-cmp to a direct require (new vendored package) without refreshing the swift vendorHash — stale since 01-02, incidentally fixed by the 01-03 jsonrpc2 refresh (discovered during hash validation) | open |  | 2026-08-16T10:38:56.476Z |  |

````json
[
  {
    "id": 1,
    "kind": "unrun-verify",
    "phase": "1",
    "file": ".github/workflows/ci.yaml",
    "line": null,
    "description": "nix eval/flake check gates for swift wiring not run — nix not installed on executing host (see deferred-items.md #2)",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-08-16T09:21:22.416Z",
    "resolved_at": "2026-08-16T13:05:47.176Z"
  },
  {
    "id": 2,
    "kind": "deviation",
    "phase": "1",
    "file": "bindings/go/scip/symbol_fuzz_test.go",
    "line": null,
    "description": "FuzzParseSymbol identity property holds for canonical spellings only — pre-existing empty-field round-trip asymmetry found by soak (see deferred-items.md #1)",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-16T09:21:22.501Z",
    "resolved_at": null
  },
  {
    "id": 3,
    "kind": "unrun-verify",
    "phase": "1",
    "file": "swift/README.md",
    "line": null,
    "description": "prettier --check swift/README.md via nix develop not run — nix absent on executing host; section hand-written to .prettierrc rules (01-02 Task 3)",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-08-16T09:44:08.603Z",
    "resolved_at": "2026-08-16T13:06:01.214Z"
  },
  {
    "id": 4,
    "kind": "unrun-verify",
    "phase": "1",
    "file": "checks.nix",
    "line": null,
    "description": "nix build .#checks.x86_64-linux.swift paste-and-rerun flow not run for the jsonrpc2 vendorHash refresh — nix absent on host; hash derived via NAR-SHA256 pipeline (thesis-grammar serializer) validated bit-for-bit against the known go-bindings hash (01-03 Task 1)",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-08-16T10:38:56.275Z",
    "resolved_at": "2026-08-16T13:06:01.293Z"
  },
  {
    "id": 5,
    "kind": "unrun-verify",
    "phase": "1",
    "file": "swift/spikes/2026-08-sourcekit-lsp-findings.md",
    "line": null,
    "description": "prettier --check via nix develop not run — nix absent on host; doc hand-written to .prettierrc rules (01-03 Task 3)",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-08-16T10:38:56.377Z",
    "resolved_at": "2026-08-16T13:06:01.379Z"
  },
  {
    "id": 6,
    "kind": "deviation",
    "phase": "1",
    "file": "checks.nix",
    "line": null,
    "description": "latent: 01-02 promoted go-cmp to a direct require (new vendored package) without refreshing the swift vendorHash — stale since 01-02, incidentally fixed by the 01-03 jsonrpc2 refresh (discovered during hash validation)",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-16T10:38:56.476Z",
    "resolved_at": null
  }
]
````
