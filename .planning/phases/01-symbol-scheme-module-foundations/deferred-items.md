# Deferred Items — Phase 1 (Symbol Scheme & Module Foundations)

Out-of-scope discoveries logged during plan execution (per executor role
scope boundary: pre-existing issues unrelated to the current task's changes
are recorded here, not fixed).

## 1. ParseSymbol/VerboseSymbolFormatter round-trip asymmetry for empty package fields

- **Found during:** Plan 01-01, Task 2 (local 60s `-fuzz=FuzzParseSymbol` soak)
- **Input:** `" 0 0 0 0!"` — an empty scheme/manager/name spelled as a literal
  empty field instead of the canonical `"."` placeholder.
- **Behavior:** `ParseSymbol` succeeds, but `VerboseSymbolFormatter.Format(parsed)`
  returns `". 0 0 0 0!"` ≠ input, because `writeEscapedPackage`
  (bindings/go/scip/symbol_formatter.go:131-137) maps `""` → `"."` on
  re-format.
- **Pre-existing and unrelated to the 01-01 peekNext fix:** the counterexample
  is pure ASCII, where the old (`byteIndex+1`) and new
  (`byteIndex+bytesToNextRune`) guards are mathematically identical.
- **Impact:** the strict identity property of `FuzzParseSymbol` holds for
  canonical spellings only. Seed corpus is green under plain `go test` and
  `-tags asserts`; the discovered corpus entry was deliberately NOT committed
  so those suites stay green. A `-fuzz` soak will rediscover this within
  seconds — documented in the `FuzzParseSymbol` doc comment.
- **Candidate fix (needs its own TDD task):** reject the empty spelling in the
  parser (treat empty scheme/manager/name/version fields as errors) or
  canonicalize on parse; either is a bindings behavior change with API/semver
  implications for the published `github.com/scip-code/scip` module.

## 2. Nix gates not runnable on the executing host

- **Found during:** Plan 01-01, Task 3 and plan-level verification.
- Nix is not installed on this macOS host (no `/nix`, no `nix` on PATH), so
  `nix eval .#checks.x86_64-linux --apply builtins.attrNames --json`,
  `nix build .#checks.x86_64-linux.swift`, `nix flake check`,
  `nix develop --command prettier --check swift/README.md`, and `nixfmt` could
  not be executed locally.
- Mitigations applied instead: the `checks.nix` `swift` attribute mirrors the
  proven `go-bindings` entry; its `vendorHash` was computed as the NAR-SHA256
  of the `GOWORK=off go mod vendor` tree using a pipeline validated against
  the known-good go-bindings hash (exact match); ci.yaml validated as YAML;
  gofmt verified clean; README written to `.prettierrc` rules.
- **First CI run on a nix-enabled host must confirm** `nix flake check` green.

## 3. Untracked `.gsd/` directory at repo root

- `dispatch-isolation-sentinel.json` — runtime metadata of the dispatch
  harness, present before execution started. Left untouched; adding repo
  `.gitignore` entries is outside this plan's files_modified.
