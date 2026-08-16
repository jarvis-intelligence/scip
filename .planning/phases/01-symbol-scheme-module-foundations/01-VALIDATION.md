---
phase: 1
slug: symbol-scheme-module-foundations
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-16
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (three-module workspace; rapid for property tests) |
| **Config file** | none — existing repo infrastructure |
| **Quick run command** | `go test ./...` from module root |
| **Full suite command** | `go test ./...` in root, `bindings/go/scip`, `swift/` + `nix flake check` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./...` (affected module)
- **After every plan wave:** Run full suite + `nix flake check`
- **Before `$gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | SYM-01 | — | N/A | unit | `go test ./bindings/go/scip/...` | ✅ | ⬜ pending |
| 1-01-02 | 01 | 1 | SYM-01 | — | N/A | unit | `go test ./swift/...` | ❌ W0 | ⬜ pending |
| 1-02-01 | 02 | 1 | SYM-01, SYM-02 | — | N/A | property | `go test ./swift/... -run Property` | ❌ W0 | ⬜ pending |
| 1-03-01 | 03 | 1 | SYM-01 | — | N/A | spike evidence | manual protocol | n/a | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `swift/` module skeleton with `go.mod` + go.work entry — property-test host for 01-02
- [ ] Existing `bindings/go/scip` rapid test patterns reused (see RESEARCH.md Code Examples)

*Existing infrastructure covers framework needs; only the new module skeleton is Wave 0.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| SourceKit-LSP spike evidence + go/no-go verdict | SYM-01 (spike) | Requires live toolchain run on macOS fixture | Follow Spike Protocol in 01-RESEARCH.md; record findings doc |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
