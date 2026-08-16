---
schema_version: 1
open_count: 2
waived_count: 0
fixed_count: 0
total_count: 2
last_updated: 2026-08-16T09:21:22.501Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 1 | unrun-verify | .github/workflows/ci.yaml |  | nix eval/flake check gates for swift wiring not run — nix not installed on executing host (see deferred-items.md #2) | open |  | 2026-08-16T09:21:22.416Z |  |
| 2 | 1 | deviation | bindings/go/scip/symbol_fuzz_test.go |  | FuzzParseSymbol identity property holds for canonical spellings only — pre-existing empty-field round-trip asymmetry found by soak (see deferred-items.md #1) | open |  | 2026-08-16T09:21:22.501Z |  |

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
  }
]
````
