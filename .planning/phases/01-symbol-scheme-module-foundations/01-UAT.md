---
status: testing
phase: 01-symbol-scheme-module-foundations
source: [01-VERIFICATION.md]
started: 2026-08-16T11:10:00Z
updated: 2026-08-16T11:10:00Z
---

## Current Test

number: 2
name: Dual-path identity prohibition (flagged, judgment-tier) — confirm and defer
expected: |
  No violation is currently observable — the single shared namer (swift/internal/symbol)
  is the only symbol-string producer, so the "no divergent identity between indexing
  paths" prohibition cannot yet be violated. This checkpoint asks you to acknowledge
  and defer: enforcement is assigned to Phase 3 via dual-path symbol-parity goldens
  (plan 03-02), where both the semantic and fallback paths emit symbols and their
  outputs are compared for identity.
awaiting: user response

## Tests

### 1. Nix gate confirmation (SC4 nix leg)

expected: `nix build .#checks.x86_64-linux.swift` and `nix flake check` pass on a nix-enabled host (or first CI run); vendorHash confirmed or pasted from the printed `got:` value; prettier clean on swift/README.md.
result: pass
context: WINDOWS.md ledger entries #1/#3/#4/#5/#6 — host lacked nix; hash derived via validated NAR pipeline; CR-01 subPackages defect already fixed in be27bd3.
evidence: CI run 31948572670 on draft PR #1 (jarvis-intelligence/scip, branch gsd/v1.0-milestone) — ALL jobs green: checks (swift) built .#checks.x86_64-linux.swift with the derived vendorHash unchanged (no `got:` correction needed); checks (formatting) prettier/nixfmt/buf clean (findings doc formatted in eb4cec2); packages (proto-generate) drift-free; go-bindings/reprolang/rust/haskell/typescript checks green. Local equivalents also verified: swift module vendor build (-mod=vendor -tags asserts) + tests pass on host.

### 2. Dual-path identity prohibition (flagged, judgment-tier)

expected: No violation observable — the single shared namer (swift/internal/symbol) is currently the only symbol-string producer, so the "no divergent identity between indexing paths" prohibition cannot yet be violated. Enforcement lands in Phase 3 via dual-path symbol-parity goldens. Confirm-and-defer: acknowledge this item so it is tracked, with enforcement ownership assigned to Phase 3.
result: [pending]
context: VERIFICATION.md flagged prohibition; non-authoritative verdict recorded 2026-08-16.

## Summary

total: 2
passed: 1
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
