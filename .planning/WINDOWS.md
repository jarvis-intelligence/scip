---
schema_version: 1
open_count: 3
waived_count: 0
fixed_count: 0
total_count: 3
last_updated: 2026-08-16T09:44:08.603Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 1 | unrun-verify | .github/workflows/ci.yaml |  | nix eval/flake check gates for swift wiring not run — nix not installed on executing host (see deferred-items.md #2) | open |  | 2026-08-16T09:21:22.416Z |  |
| 2 | 1 | deviation | bindings/go/scip/symbol_fuzz_test.go |  | FuzzParseSymbol identity property holds for canonical spellings only — pre-existing empty-field round-trip asymmetry found by soak (see deferred-items.md #1) | open |  | 2026-08-16T09:21:22.501Z |  |
| 3 | 1 | unrun-verify | swift/README.md |  | prettier --check swift/README.md via nix develop not run — nix absent on executing host; section hand-written to .prettierrc rules (01-02 Task 3) | open |  | 2026-08-16T09:44:08.603Z |  |

````json
[
  {
    "id": 1,
    "kind": "unrun-verify",
    "phase": "1",
    "file": ".github/workflows/ci.yaml",
    "line": null,
    "description": "nix eval/flake check gates for swift wiring not run — nix not installed on executing host (see deferred-items.md #2)",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-16T09:21:22.416Z",
    "resolved_at": null
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
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-16T09:44:08.603Z",
    "resolved_at": null
  }
]
````
