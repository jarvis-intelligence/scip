---
phase: 01-symbol-scheme-module-foundations
plan: 03
subsystem: sourcekit-lsp-spike
tags: [go, scip, swift, sourcekit-lsp, lsp, jsonrpc2, spike, evidence, nix]

# Dependency graph
requires:
  - phase: 01-symbol-scheme-module-foundations/01-01
    provides: swift/ Go module wired into go.work/ci.yaml/checks.nix; xcrun-based runtime-only toolchain policy
provides:
  - GO verdict on the SourceKit-LSP semantic path for v1, with measured evidence (swift/spikes/2026-08-sourcekit-lsp-findings.md)
  - Capability inventory over every hard case — overload USR distinctness (3/3), cross-file + retroactive extension attribution via USR, generics, protocol witness + existential call (isDynamic/receiverUsrs), operators, accessors (single-symbol), Unicode/UTF-16, override, call/type-hierarchy, system-module attribution
  - Harvest performance baseline on a 500-file fixture — cold 4m02s / warm 4m00s, 13506 requests, p50 35ms, p95 127-143ms, 0 crashes, 13,001 symbols + 35,500 occurrences, RSS 60.6/50.2MB
  - lsp_probe driver (jsonrpc2 v0.2.2 over stdio; readiness gate = symbolInfo-USR probe; capability + perf modes)
  - Two Phase-3 design inputs recorded: gate readiness on the symbolInfo probe (sourcekit/isIndexing unsupported on Xcode 26.3 build) and derive containers from USRs (containerName always null)
affects: [03-01-lsp-client, 03-02-semantic-harvest, phase-2-indexer]

actuals:
  tokens: 31000 # chars/4 over hand-written files (driver+generator+fixture+docs); full diff incl. committed capability evidence JSONL is ~73k
  tasks: 3
  commits: 3

tech-stack:
  added:
    - github.com/sourcegraph/jsonrpc2 v0.2.2 (spike-only; the project-decided LSP client lib)
  patterns:
    - "Readiness gate before any harvest: sourcekit/isIndexing poll + known-symbol symbolInfo probe requiring a non-empty USR — an empty result can never be recorded as 'LSP cannot do X'"
    - "Harvest discipline kept predictive: one documentSymbol pass per file + one references pass per unique def, bounded worker pool (4)"
    - "NAR-SHA256 vendorHash derivation without nix: thesis-grammar serializer (root + nested node parens), validated bit-for-bit against the known go-bindings hash before use"

key-files:
  created:
    - swift/spikes/lsp_probe/main.go
    - swift/spikes/fixture/Package.swift
    - swift/spikes/fixture/Sources/CapabilityFixture/Core.swift
    - swift/spikes/fixture/Sources/CapabilityFixture/Extensions.swift
    - swift/spikes/fixture/Sources/CapabilityFixture/Generics.swift
    - swift/spikes/fixture/Sources/CapabilityFixture/Unicode.swift
    - swift/spikes/perf/generate/main.go
    - swift/spikes/perf/.gitignore
    - swift/spikes/.gitignore
    - swift/spikes/2026-08-sourcekit-lsp-findings.md
    - swift/spikes/evidence-capability.jsonl
  modified:
    - swift/go.mod
    - swift/go.sum
    - checks.nix
    - .planning/STATE.md

key-decisions:
  - "VERDICT: GO — all five thresholds pass; symbolInfo USRs 60/60 incl. every overload, extension attribution recoverable from USRs, 500-file harvest 4m02s cold / 4m00s warm with 0 crashes, hierarchies usable"
  - "containerName is null on every symbolInfo response from this server — Phase 3 derives containers from USRs (or the documentSymbol tree), never from containerName"
  - "sourcekit/isIndexing returns -32601 method-not-found on the Xcode 26.3 build — Phase 3 readiness gates on the symbolInfo-USR probe with retry"
  - "Accessors (get/set) are not distinct LSP symbols (single var USR Sdvp) — getter/setter Kinds come from syntax/roles, confirming the scheme's flagged accessor assumption"
  - "checks.nix vendorHash refreshed via a NAR-SHA256 pipeline whose serializer follows the thesis grammar (root AND nested node parens); validated bit-for-bit against the known go-bindings hash before hashing the new tree (nix absent on host)"
  - "Perf evidence JSONL (6MB each) git-ignored as runtime output; capability evidence (168KB) committed because the findings doc quotes its excerpts"

patterns-established:
  - "Spike evidence format: one JSONL record per RPC with latency_ms + phase tags (meta/init/ready/harvest/hierarchy/summary); trimmed counts in perf mode, verbatim payloads in capability mode"
  - "Deterministic fixture generation: token-replacer template (@I@/@R1@/@R2@/@R3@) + seeded cross-file refs; byte-stability verified by regenerating and diffing"

requirements-completed: [SYM-01]

coverage:
  - id: D1
    description: "Capability inventory with observed evidence for every hard case, harvested only after the readiness gate passed (symbolInfo-USR probe); findings doc quotes a JSONL excerpt per row"
    requirement: SYM-01
    verification:
      - kind: integration
        ref: swift/spikes/evidence-capability.jsonl (204 records; ready records at index <=6, first harvest at index 7)
        status: pass
      - kind: other
        ref: swift/spikes/2026-08-sourcekit-lsp-findings.md#Capability-inventory (14 rows)
        status: pass
    human_judgment: false
  - id: D2
    description: "Harvest performance on the ~500-file generated fixture with all nine prescribed metrics, cold and warm, under the prescribed harvest discipline with bounded concurrency (4)"
    requirement: SYM-01
    verification:
      - kind: integration
        ref: swift/spikes/evidence-perf-cold.jsonl + evidence-perf-warm.jsonl summary records (git-ignored; metrics transcribed in findings doc)
        status: pass
    human_judgment: false
  - id: D3
    description: "Explicit GO/NO-GO verdict evaluated mechanically against the stated (flagged-assumed) thresholds; STATE.md decision entry references the findings doc"
    requirement: SYM-01
    verification:
      - kind: other
        ref: swift/spikes/2026-08-sourcekit-lsp-findings.md#Verdict + .planning/STATE.md Decisions
        status: pass
    human_judgment: false
  - id: D4
    description: "jsonrpc2 added to swift/go.mod without breaking the Nix gate: vendorHash refreshed in the same task/commit as the go.mod change"
    requirement: SYM-01
    verification:
      - kind: other
        ref: commit 763183d (go.mod+go.sum+checks.nix together); hash pipeline validated against known go-bindings hash
        status: pass
      - kind: unit
        ref: command:cd swift && GOWORK=off go build ./spikes/... && go vet ./spikes/... && go test ./...
        status: pass
    human_judgment: true
    rationale: "nix is not installed on the executing host, so `nix build .#checks.x86_64-linux.swift` and `nix flake check` were NOT run; the vendorHash was derived via a NAR-hash pipeline validated bit-for-bit against the known go-bindings hash (first candidate serializer failed validation and was fixed against the thesis grammar before use). Recorded as WINDOWS.md entries 4-6; first CI run on a nix-enabled host must confirm."

# Metrics
duration: 92min
completed: 2026-08-16
status: complete
---

# Phase 1 Plan 03: SourceKit-LSP Spikes Summary

**GO verdict on the SourceKit-LSP semantic path, on measured evidence: a jsonrpc2 probe driver with a readiness gate harvested a hard-case capability fixture (60/60 symbolInfo USRs, all overloads distinct, extension attribution via USRs, hierarchies live) and a generated 500-file fixture in 4m02s cold / 4m00s warm with zero crashes — Phase 3's LSP-client architecture is validated.**

## Performance

- **Duration:** 92 min
- **Started:** 2026-08-16T09:05:00Z (approx; includes ~35 min of live spike runs)
- **Completed:** 2026-08-16T10:39:30Z
- **Tasks:** 3
- **Files modified:** 16 (+ committed evidence)

## Accomplishments

- Built `swift/spikes/lsp_probe`: discovers sourcekit-lsp via `xcrun -f` (PATH fallback), records toolchain versions in the first evidence record, performs the full LSP handshake with the fixture as rootUri, records `InitializeResult.capabilities` verbatim, gates readiness (isIndexing poll + known-symbol symbolInfo probe requiring a non-empty USR), then harvests with per-request latency into JSONL — capability mode (symbolInfo/definition/references per def + call/type hierarchy probes) and perf mode (references pass per unique def, bounded pool of 4)
- Created the hand-written capability fixture covering every RESEARCH hard case (verified to compile with `swift build` before use) and the deterministic 500-file perf generator (byte-stable re-generation verified by diff; fixture compiles in 12.5s)
- Ran the capability spike live on Xcode 26.3 / Swift 6.2.4: 204 evidence records; 60/60 symbolInfo responses carried USRs; overloads, generics, witnesses, operators, Unicode, override, hierarchies all observed with excerpts
- Ran cold + warm perf spikes: identical deterministic harvest (13,001 symbols, 35,500 occurrences, 13,506 requests), zero crashes, all nine metrics recorded
- Wrote the findings doc with thresholds, inventory, numbers, per-threshold GO evaluation, untriggered escape-hatch note, and re-run commands; recorded the STATE.md decision
- Refreshed checks.nix vendorHash for jsonrpc2 via a NAR-SHA256 pipeline validated against the known go-bindings hash — which also surfaced and fixed a latent stale hash from 01-02 (go-cmp had entered the vendor tree without a refresh)

## Task Commits

1. **Task 1: driver + capability fixture + jsonrpc2 + vendorHash** - `763183d` (feat)
2. **Task 2: perf generator + cold/warm runs** - `10e7cfd` (feat)
3. **Task 3: findings doc + STATE decision** - `e63680b` (docs)

## Files Created/Modified

See key-files in frontmatter. Headlines: `swift/spikes/lsp_probe/main.go` (driver, ~900 lines), `swift/spikes/perf/generate/main.go` (generator), `swift/spikes/fixture/**` (5 committed Swift files), `swift/spikes/2026-08-sourcekit-lsp-findings.md`, `swift/spikes/evidence-capability.jsonl` (committed evidence), `swift/go.mod`+`go.sum` (jsonrpc2 v0.2.2), `checks.nix` (vendorHash), `.planning/STATE.md` (decision).

## Decisions Made

See key-decisions in frontmatter. The load-bearing ones for Phase 3: containers from USRs (containerName always null), readiness via symbolInfo probe (isIndexing unsupported here), accessors are single symbols.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Readiness gate needed a didOpen before symbolInfo**
- **Found during:** Task 1 (first live run)
- **Issue:** The gate probed a file never opened via didOpen; sourcekit-lsp answers -32001 "No language service" for unopened documents — the run aborted at the gate
- **Fix:** Gate didOpens the probe document first (tracked in an open-docs set so harvest never re-opens); protocol-correct and consistent with the plan's sequence
- **Committed in:** 763183d

**2. [Rule 1 - Bug] Wrong hierarchy method names**
- **Found during:** Task 1 (first live run)
- **Issue:** Driver used `callHierarchy/prepare`/`typeHierarchy/prepare`; LSP 3.17 defines `textDocument/prepareCallHierarchy`/`textDocument/prepareTypeHierarchy` — both returned -32601
- **Fix:** Correct method names; hierarchy probes then returned real data
- **Committed in:** 763183d

**3. [Rule 3 - Blocking] Nix gates unrunnable — vendorHash derived via validated NAR pipeline (two-iteration validation)**
- **Found during:** Task 1
- **Issue:** `nix build .#checks.x86_64-linux.swift` paste-and-rerun cannot run (nix absent); the plan's acceptance assumed it
- **Fix:** Reconstructed the NAR-SHA256 pipeline; the first serializer failed validation against the known go-bindings hash, was corrected against the thesis grammar (root AND nested-node parentheses), then matched the go-bindings hash bit-for-bit before hashing the new tree; recorded as WINDOWS.md #4. Also added fixture/perf .gitignores and a spikes .gitignore for the 6MB perf evidence files (runtime outputs)
- **Committed in:** 763183d, 10e7cfd

**4. [Rule 1 - Bug] Perf generator template compile error + probe-name mismatch**
- **Found during:** Task 2
- **Issue:** `&+` applied to String operands; and pad-3 filenames (Gen000) vs the driver's Gen0000Widget default probe symbol
- **Fix:** String `+`; driver default changed to Gen000Widget. Determinism and compilability re-verified (swift build 12.5s)
- **Committed in:** 10e7cfd

**5. [Rule 3 - Blocking] Findings-doc gate counted two 'How to re-run' matches**
- **Found during:** Task 3
- **Issue:** Metadata table referenced the section by its exact title; `grep -cF` gate expects exactly 1
- **Fix:** Rephrased the metadata cell
- **Committed in:** e63680b

---

**Total deviations:** 5 auto-fixed (2 bug-class protocol/compile fixes caught by live runs, 1 blocking env constraint with validated mitigation, 1 bug, 1 gate hygiene)
**Impact on plan:** All fixes were required to satisfy the plan's own acceptance criteria. No scope creep; spike stays throwaway (nothing promoted into internal/, no CLI wiring).

## Known Stubs

None. All driver surfaces are live implementations; the spike has no placeholder behavior. (The `trimmed` result summaries in perf mode are an evidence-size policy, documented in the findings doc, not a stub.)

## Issues Encountered

- `sourcekit/isIndexing` is not supported on the Xcode 26.3 sourcekit-lsp build (-32601); the gate's symbolInfo probe is the real readiness authority — recorded as an inventory row and a Phase 3 design input, not hidden
- sourcekit-lsp on this build has no `--version` flag and returns no `serverInfo`; the findings metadata records the toolchain-derived version evidence honestly
- `containerName` is null on all 60 symbolInfo responses — attribution must come from USRs; recorded as inventory row 3 with the mechanism change spelled out
- Latent 01-02 issue discovered: go-cmp entered the swift vendor tree without a vendorHash refresh; fixed incidentally by this plan's refresh, logged as WINDOWS.md #6 (deviation)

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 3 (03-01/03-02) consumes the findings doc: LSP client with symbolInfo-USR-probe readiness gating, USR-derived containers, references-based occurrences, hierarchy data available
- Phase 2 is unaffected (syntax fallback path; accessor Kind decision confirmed as syntax-derived)
- Open follow-ups: WINDOWS.md #1/#4 — first CI run on a nix-enabled host must confirm `nix flake check` green with the refreshed vendorHash; #5 — prettier on the findings doc

## Self-Check: PASSED

- Created files exist on disk: swift/spikes/lsp_probe/main.go, swift/spikes/fixture/Package.swift + 4 sources, swift/spikes/perf/generate/main.go, swift/spikes/2026-08-sourcekit-lsp-findings.md, swift/spikes/evidence-capability.jsonl — all FOUND
- Commits exist on branch gsd/v1.0-milestone: 763183d, 10e7cfd, e63680b — all FOUND
- Findings doc gates: `## Verdict` x1, `How to re-run` x1, STATE.md references the findings doc x1, verdict word present — VERIFIED

---
*Phase: 01-symbol-scheme-module-foundations*
*Completed: 2026-08-16*
