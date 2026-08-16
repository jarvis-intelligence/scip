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
  On a host with nix installed (or in the first CI run of branch gsd/v1.0-milestone):
  1. `nix build .#checks.x86_64-linux.swift` succeeds — the swift check builds with the
     derived vendorHash sha256-Vz6S8i6… and subPackages [cmd/scip-swift internal/symbol].
     If the hash mismatches, paste the printed `got: sha256-…` into checks.nix and re-run
     (the derivation pipeline was validated bit-for-bit against the known go-bindings hash,
     but first-live-run confirmation is the point of this test).
  2. `nix flake check` fully green (includes the new swift check alongside existing checks).
  3. Prettier check passes on swift/README.md (hand-conformed to .prettierrc; see WINDOWS.md #3).
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
