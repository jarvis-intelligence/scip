# Research: Phase 1 — Symbol Scheme & Module Foundations

**Project:** scip-swift (fork of scip-code/scip at `/Users/ddphuong/Projects/jarvis-ai/scip`)
**Phase:** 1 of 6 — Requirements SYM-01, SYM-02
**Researched:** 2026-08-16
**Confidence:** HIGH for all in-repo facts (read this session; the ParseSymbol panic and its one-line fix were **empirically reproduced and verified on this tree**) and for SourceKit-LSP extension shapes (official contributor docs). MEDIUM/ASSUMED items are flagged inline and listed at the end.

Plans this research feeds: **01-01** (module skeleton + ParseSymbol fix), **01-02** (symbol-scheme spec + namer + round-trip tests), **01-03** (SourceKit-LSP spikes).

---

## User Constraints

No CONTEXT.md exists; constraints copied from PROJECT.md (`/Users/ddphuong/Projects/jarvis-ai/scip/.planning/PROJECT.md`) and STATE.md.

- **Tech stack**: "Swift + SourceKit-LSP for semantics; tree-sitter (Swift grammar) for fallback — hybrid decided during questioning" [VERIFIED: .planning/PROJECT.md:83-84]
- **Platform**: "macOS arm64 + x86_64 for v1, full semantics on both" [VERIFIED: .planning/PROJECT.md:85]
- **Integration**: "Emitted index must pass `scip lint` and follow SCIP symbol-format conventions documented in `scip.proto`" [VERIFIED: .planning/PROJECT.md:86-87]
- **Environment**: "The repo's canonical toolchain is Nix (`nix develop`, flake checks); Swift toolchain integration must fit the flake/check matrix or justify a CI exception" [VERIFIED: .planning/PROJECT.md:88-89]
- **Compatibility**: "Must not break existing modules (Go workspace, bindings codegen, reprolang tests)" [VERIFIED: .planning/PROJECT.md:90-91]
- **Key decision (module placement)**: "Build scip-swift in this repo as a new module (reprolang precedent)" [VERIFIED: .planning/PROJECT.md:97]
- **Key decision (hybrid)**: "Hybrid semantics: SourceKit-LSP primary, tree-sitter fallback" [VERIFIED: .planning/PROJECT.md:98]
- **Key decision (validation)**: "Done = `scip lint` + snapshot fixtures + jarvis e2e" [VERIFIED: .planning/PROJECT.md:100]
- **Phase-1 blocking bug**: "[Pre-existing, fix in Phase 1]: `scip.ParseSymbol` panics on trailing multi-byte runes (`bindings/go/scip/symbol_parser.go` …) — must be fixed before mass symbol emission" [VERIFIED: .planning/STATE.md:67]
- **Schema frozen**: "`scip.proto` schema changes" are explicitly Out of Scope — Swift data must fit existing roles/kinds ("Language.Swift = 2 verified") [VERIFIED: .planning/REQUIREMENTS.md:65]

## Project Constraints (from AGENTS.md)

Actionable directives extracted from `/Users/ddphuong/Projects/jarvis-ai/scip/AGENTS.md` (GSD-managed merge of PROJECT.md + codebase analysis):

1. **Go workspace topology**: "Three modules in `go.work` (root, `bindings/go/scip`, `reprolang`) linked by `replace` directives — new shared code goes in the bindings module, not the root module." scip-swift extends this to a 4th module; anything shared *inside* scip-swift lives in `swift/internal/` first and is promoted to bindings only with a second consumer.
2. **Never hand-edit generated code**: "`scip.pb.go` — never hand-edit; regenerate with `nix run .#proto-generate`" [VERIFIED: AGENTS.md:130]. The namer builds `*scip.Symbol` protobuf values; it must never string-patch generated types.
3. **File/code conventions**: `snake_case.go` one concern per file (`symbol.go`, `symbol_parser.go` precedent); tests co-located `<file>_test.go`; exported identifiers get godoc comments; typed struct errors with `errors.Join`; `, ok` second returns for "no result"; `slices`/stdlib Go 1.21+ helpers.
4. **Formatting enforced by Nix**: gofmt + goimports on all Go; prettier on `**/*{ts,js(on)?,md,yml}` (a `swift/README.md` must be prettier-clean: semi:false, singleQuote, es5 trailing commas); `nixfmt` on `*.nix` [VERIFIED: AGENTS.md:149-151].
5. **Build tags**: Nix compiles tests with `buildTags = [ "asserts" ]` [VERIFIED: AGENTS.md:157] — the parser fix must keep `assert()` preconditions true under `-tags asserts`.
6. **Do not add test files** to `bindings/go/scip/memtest/` (sentinel file `DO_NOT_ADD_NEW_TEST_FILES_HERE`); memtest must not run in parallel.
7. **Version single-source**: `cmd/scip/version.txt` (currently `0.9.0`) parity is asserted only for published binding manifests (rust/haskell/java/kotlin/typescript package manifests and `reprolang/package.json`) [VERIFIED: checks.nix asserts, lines 67-69, 81-85, 124-128, 139-144]. The `swift/` module has none of these → **no version.txt sync needed in Phase 1** (verified by inspection; see Module Wiring).
8. **GSD workflow enforcement**: repo edits go through GSD commands [VERIFIED: AGENTS.md:384-395].
9. **CLI pattern (later phases)**: three-tier `xxxCommand()/xxxMain()/xxxMainPure()`; not needed for Phase 1's namer/spike code but applies to `cmd/scip-swift` from Phase 2 on.

## Standard Stack

Exact tools/versions for this phase (Go-only — **no cgo, no tree-sitter, no Swift toolchain at build time in Phase 1**; the grammar lands in Phase 2):

| Component | Version | Role in Phase 1 |
|---|---|---|
| Go | 1.25.0 (repo pin: `go 1.25.0` in go.work) [VERIFIED: go.work:1] | `swift/` module + bindings fix |
| `github.com/scip-code/scip/bindings/go/scip` | in-repo via `replace` (protobuf runtime `google.golang.org/protobuf v1.36.12`) | namer builds `*scip.Symbol`; `ParseSymbol`/`VerboseSymbolFormatter` round-trip; **also the site of the panic fix** |
| `pgregory.net/rapid` | **v1.3.0** (already pinned in `bindings/go/scip/go.mod:16`) | property-based round-trip tests for the namer; use the same version in `swift/go.mod` |
| `github.com/stretchr/testify` | v1.11.1 | assertions (repo-wide) |
| `github.com/sourcegraph/jsonrpc2` | v0.2.2 | JSON-RPC wire for the Phase-1 LSP spike driver (the project-decided LSP client lib — use it in the spike, not a hand-rolled framer) [CITED: project STACK.md] |
| Go native fuzzing (`testing.F`) | stdlib | `FuzzParseSymbol` regression for the panic (modern replacement for the `github.com/google/go-fuzz` usage in `parse_test.go`) |
| Nix | nixos-26.05 (flake.lock) | `nix flake check`, `nix develop` for gofmt/goimports/prettier/nixfmt |
| sourcekit-lsp (runtime only) | bundled with local Xcode 26.3 / Swift 6.2.4; require ≥ 6.1 | spike target only — **never a build-time dependency** |

## Architecture Patterns

### Module layout (plan 01-01)

Follow the reprolang precedent exactly [VERIFIED: reprolang/go.mod — `module github.com/scip-code/scip/reprolang`, `go 1.25.0`, `replace github.com/scip-code/scip/bindings/go/scip => ../bindings/go/scip`]:

```
swift/                                  # NEW Go module: github.com/scip-code/scip/swift
├── go.mod                              # module + replace => ../bindings/go/scip; GOWORK=off tidy
├── go.sum
├── README.md                           # prettier-clean; states runtime prerequisites
├── cmd/
│   └── scip-swift/
│       └── main.go                     # stub main in Phase 1 (real CLI surface Phase 2+)
├── internal/
│   └── symbol/                         # THE shared namer (plan 01-02)
│       ├── scheme.go                   # frozen spec constants (scheme, managers, rules doc)
│       ├── namer.go                    # SymbolInput -> *scip.Symbol -> string
│       ├── namer_test.go               # table tests (mapping table below, one case per row)
│       └── roundtrip_test.go           # rapid property tests vs scip.ParseSymbol/FormatSymbol
└── spikes/                             # plan 01-03 deliverables (see Spike Protocol)
    ├── lsp_probe/                      # runnable Go driver (cmd-style main or _test)
    ├── fixture/                        # SwiftPM capability fixture (Package.swift + sources)
    ├── perf/                           # generated ~500-file perf fixture
    └── 2026-08-sourcekit-lsp-findings.md   # evidence + go/no-go verdict
```

Deliberate deviations from the full Phase-2+ layout (project ARCHITECTURE.md): no `grammar/`, no `internal/fallback/`, no `internal/emit/` in Phase 1 — they belong to Phase 2. Keep `internal/` (not a flat package like reprolang's `repro/`) per project research.

### Namer API shape (plan 01-02)

Prescriptive design — the namer is a **pure function library** (no I/O, no USR parsing yet; the USR mapper grows on top in Phase 3):

```go
// Package symbol defines the frozen scip-swift SCIP symbol scheme and the
// shared namer both indexing paths use to produce symbol strings.
package symbol

const (
	Scheme        = "scip-swift"
	ManagerSwiftPM = "swiftpm" // SwiftPM targets
	ManagerSystem  = "swift"   // system/SDK modules (stdlib etc.), mirrors scip-java's "jdk"
)

// DeclKind enumerates the Swift declaration categories the scheme distinguishes.
type DeclKind int // Type, Struct?, ... — one value per mapping-table row below

// SymbolInput is everything needed to name a symbol. Both the semantic path
// (Phase 3, from symbolInfo/container chains) and the fallback path (Phase 2,
// from syntax) must be able to populate it.
type SymbolInput struct {
	Module          string      // owning Swift module (target) name
	IsSystemModule  bool        // true for stdlib/SDK symbols (manager "swift")
	ContainerPath   []Container // extended-type-aware ancestry, outermost first
	Name            string      // source name, may be an operator or Unicode
	Kind            DeclKind
	OverloadIndex   int         // 0 = no disambiguator; N>0 -> "(+N)" (scip-java style)
}

// Symbol returns the canonical SCIP symbol string. It constructs a
// *scip.Symbol and formats it with scip.VerboseSymbolFormatter — it never
// hand-concatenates, so backtick escaping comes from the bindings for free.
func Symbol(in SymbolInput) (string, error)

// LocalSymbol returns a document-scoped local symbol string "local <id>".
// id is sanitized to <simple-identifier> with a per-document counter.
func LocalSymbol(sourceName string, ordinal int) string
```

Key rules:

- **Build `*scip.Symbol`, format with `scip.VerboseSymbolFormatter.FormatSymbol`** [VERIFIED: bindings/go/scip/symbol_formatter.go:65-91]. `writeSuffixedDescriptor` (lines 139-159) auto-backtick-escapes any identifier containing characters outside the simple set — operators (`==`, `<->`) and Unicode names (`🚀`, `π`) are escaped correctly with zero custom code. `writeEscapedPackage` (lines 131-137) maps empty version `""` → `"."`, and the parser's `sw.normalize(".", "")` maps it back — the `"."` placeholder round-trips by construction [VERIFIED: symbol_parser.go:88,95,102 and symbol_formatter.go:131-137].
- Locals go through the `local` form: `Symbol{Scheme: "local", Descriptors: [{Name: id, Suffix: Descriptor_Local}]}` — formats as `local <id>` and parses back [VERIFIED: symbol_parser.go:120-144 `tryParseLocalSymbolV2`; symbol_test.go:16-19].
- Determinism: `OverloadIndex` is assigned in **source declaration order** within `(module, container, name)` groups; both paths must derive it the same way (semantic path: from USR-distinct decls sorted by position; fallback: syntax order).

### Test patterns to copy (all verified in-repo)

- **Table-driven parse/format**: `bindings/go/scip/symbol_test.go:10-134` (`TestParseSymbol` — input string → expected `*Symbol`, compared via `.String()` with `cmp.Diff`).
- **Round-trip**: `bindings/go/scip/symbol_formatter_test.go:31-50` (`TestSymbolFormatterRoundTrip` — `VerboseSymbolFormatter.Format(in) == in`).
- **No-panic error assertions**: `bindings/go/scip/symbol_test.go:136-155` (`TestParseSymbolError` — `require.NotPanics` + non-nil err) — this is exactly where the panic regression cases go.
- **rapid property tests**: `bindings/go/scip/position_test.go:64-90` pattern — `rapid.Custom` generator + `rapid.Check(t, func(t *rapid.T) { ... })`; also `sort_test.go:129-146`.
- **Module-scaffold tests**: `reprolang/repro/snapshot_test.go` (testutil.SnapshotTest wiring) — not needed until Phase 2, but the `swift/testdata/snapshots/{input,output}` layout must be reserved if used later.

## Symbol Scheme Mapping Table

**This is the normative spec skeleton for plan 01-02.** Grammar facts cited from `scip.proto` [VERIFIED: scip.proto:148-230]:

```
<symbol>               ::= <scheme> ' ' <package> ' ' (<descriptor>)+ | 'local ' <local-id>
<package>              ::= <manager> ' ' <package-name> ' ' <version>
<descriptor>           ::= <namespace> | <type> | <term> | <method> | <type-parameter> | <parameter> | <meta> | <macro>
<namespace>            ::= <name> '/'        <type> ::= <name> '#'      <term> ::= <name> '.'
<meta>                 ::= <name> ':'         <macro> ::= <name> '!'
<method>               ::= <name> '(' (<method-disambiguator>)? ').'
<type-parameter>       ::= '[' <name> ']'     <parameter> ::= '(' <name> ')'
<identifier-character> ::= '_' | '+' | '-' | '$' | ASCII letter or digit
<escaped-identifier>   ::= '`' … '`'          <local-id> ::= <simple-identifier>
```

Header form: **`scip-swift swiftpm <ModuleName> <version>`** with `<version> = "."` (SwiftPM `Package.swift` carries no version; git-tag derivation deferred — ASSUMED decision, grounded in REQUIREMENTS SYM-01 "modules as `swiftpm` packages"). System/SDK modules: `scip-swift swift <ModuleName> <swift-version>` — mirrors scip-java's `scip-java maven jdk 25 java/util/List#` [CITED: https://github.com/scip-code/scip-java — verified from scip-snapshots expected files]. Scheme precedents: scip-python emits `scip-python python <name> <version>` [CITED: https://github.com/sourcegraph/scip-python — `ScipSymbol.ts` line 14: `` new TypescriptScipSymbol(`scip-python python ${name} ${version} `) ``]. Scheme must not start with `local` [VERIFIED: scip.proto:159] — `scip-swift` is safe.

Mapping (module `M` used in examples; `Kind` = `SymbolInformation.Kind`, all verified present in scip.proto with Swift-specific comments):

| # | Swift construct | Symbol string | Descriptor(s) | Kind | Notes |
|---|---|---|---|---|---|
| 1 | SwiftPM module (target) `M` | `scip-swift swiftpm M . M/` | Namespace `M/` | Module=29 | The module's own symbol — target of `import M` occurrences (SYM-04, Phase 3). Follows scip-python's module-as-namespace precedent. |
| 2 | System module (e.g. `Swift`) | `scip-swift swift Swift <ver> Swift/` | Namespace | Module=29 | External symbols only when actually referenced (Phase 3). |
| 3 | struct / class / actor | `… M . Shape#` | Type `Shape#` | Struct=49 / Class=7 / Class=7 | No Actor kind exists; use Class. |
| 4 | enum | `… M . Color#` | Type | Enum=11 | |
| 5 | protocol | `… M . Drawable#` | Type | Protocol=42 | Proto comment: "Analogous to 'Trait' and 'TypeClass', for Swift and Objective-C" [VERIFIED: scip.proto:365-366]. |
| 6 | typealias | `… M . FooAlias#` | Type | TypeAlias=55 | |
| 7 | Nested type | `… M . Outer#Inner#` | chained Type | per inner kind | Descriptors "include one descriptor for every node in the AST (along the ancestry path)" [VERIFIED: scip.proto:182-186]. |
| 8 | Top-level func | `… M . parse().` | Method `parse().` | Function=17 | scip grammar has no Function descriptor; Method suffix `()` is the family convention (scip-java `CompactMain#message().`). |
| 9 | Method | `… M . Shape#area().` | Method | Method=26 | |
| 10 | Overloads (2nd, 3rd… same-name in same container) | `… M . Shape#resize(+1).`, `resize(+2).` | Method + disambiguator | Function/Method | **scip-java precedent: `java/util/List#of(+2).`** [CITED: scip-code/scip-java scip-snapshots]. Rule: sort decls by source position; index 0 → no disambiguator; index N → `(+N)`. Disambiguator is a `<simple-identifier>` [VERIFIED: scip.proto:173]. |
| 11 | Operator func (`+`, `==`, `<->`) | `… M . Vec#+().` and `… M . Vec#`==`().` | Method (escaped when needed) | Operator=34 | `+`/`-` ARE identifier characters [VERIFIED: scip.proto:176]; `= < > * / % & ?` are NOT → formatter backtick-escapes automatically. Round-trip test must include both forms. |
| 12 | `init` | `… M . Vec#init().`, `init(+1).` | Method `init` | Constructor=9 | Swift-native name `init` (what SourceKit reports); overload disambiguation as row 10. `init?`/`init(from:)` are all `init` + disambiguator. |
| 13 | `deinit` | `… M . Vec#deinit().` | Method | Method=26 | |
| 14 | Property getter | `… M . Shape#area().` — see note | Method `area().` | Getter=18 | Kind reserved "For 'get' in Swift" [VERIFIED: scip.proto:321-322]. **ASSUMED/flagged:** getter reuses the property's Method-shaped descriptor; distinguish from a zero-arg method `area()` via Kind only — collisions with a same-named method are resolved by the overload index. |
| 15 | Property setter | `… M . Shape#`area=`().` | Method (backtick-escaped `` `area=` ``) | Setter=45 | Ruby `attr_writer` `foo=` precedent; `=` ∉ identifier set → backticks. Kind "For 'set' in Swift" [VERIFIED: scip.proto:375-376]. Flagged ASSUMED (alternative: defer accessor symbols). |
| 16 | Stored/computed property (var) | `… M . Shape#origin.` | Term `origin.` | Property=41 | |
| 17 | `let` member / global | `… M . origin.` | Term | Constant=8 | |
| 18 | Global var | `… M . config.` | Term | Variable=61 | |
| 19 | Subscript | `… M . Vec#subscript().`, `subscript(+1).` | Method | Subscript=47 | Kind "For Swift" [VERIFIED: scip.proto:397-398]. |
| 20 | Enum case | `… M . Color#red.` | Term | EnumMember=12 | Cases are value terms, scip-java/scip-python enum-member precedent. |
| 21 | **Extension member (same module, any file) — SYM-02** | `… M . Shape#area2().` | identical to row 9 | per member kind | **The extension introduces NO descriptor.** A member of `extension Shape` gets exactly the symbol it would have if declared inside `Shape`'s body — same file or different file. |
| 22 | **Extension member (retroactive, other module)** | `scip-swift swiftpm String <ver> String#spike().` — i.e. package of the module OWNING the extended type | owner's path | per member kind | Qualified form: the member lives under the extended type's full (module, type) identity so findReferences/typeHierarchy never miss it [VERIFIED requirement: REQUIREMENTS.md SYM-02]. Collision of same-name retroactive members declared in different modules → overload-disambiguator mechanism (row 10) extended to Terms. **ASSUMED/flagged**: owner-module attribution vs declaring-module. |
| 23 | Protocol requirement method | `… M . Drawable#draw().` | Method | ProtocolMethod=68 | "Analogous to 'AbstractMethod', for Swift and Objective-C" [VERIFIED: scip.proto:367-368]. |
| 24 | Conformance witness | `… M . Circle#draw().` + `relationships: [{Drawable#draw(), is_implementation, is_reference}]` | own Method | Method=26 | Exactly the proto's TypeScript `Animal#sound()`/`Dog#sound()` semantics [VERIFIED: scip.proto:465-501]. Relationships are Phase 4 — but the **symbol paths** are frozen now. |
| 25 | Class override | `… M . Dog#speak().` (own path) | Method | Method=26 | Override relationships Phase 4. |
| 26 | Generic type parameter `T` of `Box<T>` | `… M . Box#.[T]` | TypeParameter `[T]` | TypeParameter=58 | Trailing descriptor, scip-java test symbol shape `method(+1).(param)[TypeParam]` [VERIFIED: symbol_test.go:50]. Generic *specialization* resolution is Out of Scope [VERIFIED: REQUIREMENTS.md:60]. |
| 27 | Function parameter `x` of `f` | `… M . f().(x)` | Parameter `(x)` | Parameter=37 | scip-java precedent (same test symbol). Parameters stay global-form, not `local`. |
| 28 | Function local `let i` | `local i` (sanitized + counter) | local form | (locals carry no SymbolInformation kind; use `enclosing_symbol` if needed) | `local <id>` with `<id>` a `<simple-identifier>` [VERIFIED: scip.proto:157,179]. **Unicode local names cannot be local ids** — sanitize: non-identifier chars → `_`, append `_2`, `_3`… on collision within the document; `display_name` keeps the source name. ASSUMED scheme (planner may simplify to `v<n>` ordinals). |
| 29 | Macro (`#Preview`) | `… M . Preview!` | Macro `Preview!` | Macro=25 | Spec entry only; tree-sitter sees macros as ERROR nodes (Phase 2 problem) and semantic support is unknown — spike inventory item. |
| 30 | Property wrapper (`@Foo var x`) | wrapper type = row 3/9 as normal; wrapped property row 16 | — | — | Synthesized `_x`/`$x` accessors NOT emitted in v1 (deferred; `$` is an identifier char so the option stays open). |
| 31 | `self` / `Self` | not emitted | — | (SelfParameter=44 exists) | Skip — no navigation value in v1. |

**Frozen-spec extras the spec doc must state** (plan 01-02 deliverable, `swift/internal/symbol/scheme.go` doc comment + `swift/README.md` section):
- Determinism rule: overload indices and local ordinals derive from source order only.
- Canonical `#if` policy: the semantic path indexes the canonical build configuration (macOS arm64, host Swift version, DEBUG); the fallback path must select the same canonical branch rather than all branches [VERIFIED: PITFALLS.md Pitfall 11 — policy decided Phase 1].
- Dual-path identity rule: both paths produce identical strings for the same construct; the fallback never merges output with semantic output.

## ParseSymbol Fix

**Empirically verified this session** (test run against this tree, then reverted — working tree left clean):

- `"a b c d fooΩ"` → **panics**: `runtime error: index out of range [13] with length 13`
- `"test . pkg . barΩ"` → **panics**: `index out of range [18] with length 18`
- `"scip swift MyMod . `🚀`."` → parses fine (valid suffix after the multi-byte rune) — the form the namer emits for escaped Unicode names is safe.

**Root cause** [VERIFIED: bindings/go/scip/symbol_parser.go:374-379]:

```go
func (z *symbolParserV2) peekNext() (rune, int32) {
	if z.byteIndex+1 < len(z.SymbolString) {
		return findRuneAtIndex(z.SymbolString, z.byteIndex+int(z.bytesToNextRune))
	}
	return 0, 0
}
```

The guard tests `byteIndex+1` but the read index is `byteIndex + bytesToNextRune`. When the current rune is the final rune of the string and is multi-byte (e.g. `Ω` = 2 bytes at byteIndex 11, len 13): `11+1 < 13` passes, `findRuneAtIndex(s, 13)` executes `b1 := s[byteIndex]` → OOB panic inside `findRuneAtIndex` (line 282). Reach path: `parseDescriptor` line 418-419 (`suffixRune := z.currentRune; z.advanceRune()`) after `acceptIdentifier` stops at the non-ASCII rune; also `acceptTerminatedIdentifier`'s unconditional `advanceRune` (line 529). CONCERNS.md documents the same diagnosis and trigger set [VERIFIED: .planning/codebase/CONCERNS.md:59-64].

**Minimal fix (verified: all existing symbol tests pass, panic inputs now return errors):**

```go
func (z *symbolParserV2) peekNext() (rune, int32) {
	if z.byteIndex+int(z.bytesToNextRune) < len(z.SymbolString) {
		return findRuneAtIndex(z.SymbolString, z.byteIndex+int(z.bytesToNextRune))
	}
	return 0, 0
}
```

One line: `z.byteIndex+1` → `z.byteIndex+int(z.bytesToNextRune)`. After the fix, the trailing-rune case falls through to the `default:` branch of the suffix switch and returns `unrecognizedDescriptorError` (symbol_parser.go:448-456) — an error, not a panic, exactly the CLI-compatible behavior. Post-fix verification run: `TestParseSymbol`, `TestSymbolFormatter*` green; the three panic inputs above return errors.

**Tests to add in `bindings/go/scip/symbol_test.go`:**
1. Append the regression inputs to `TestParseSymbolError`'s list (pattern verified at symbol_test.go:136-155): `"a b c d fooΩ"`, `"test . pkg . barΩ"`, `` "a b c d `foo`Ω" `` — `require.NotPanics` + error non-nil (the existing harness already asserts both).
2. Add a native Go fuzz target `FuzzParseSymbol(f *testing.F)` in a new `symbol_fuzz_test.go`: seed corpus = every string in `TestParseSymbol`/`TestParseSymbolError` + the multi-byte tails + `` `🚀` `` escaped forms; property = `ParseSymbol` never panics (error is fine) and on success `VerboseSymbolFormatter.Format(s) == input`. Seed corpus runs in normal `go test`; `-fuzz` runs are a local/optional exercise (do not wire `-fuzz` into CI).
3. A rapid property: random `rapid.String()` (or rune-soup generator biased to multi-byte runes at string tail) never panics — cheap addition in the same file using the position_test.go pattern.

**Upstream status (verified via GitHub search + `gh`)**: no open issue in scip-code/scip describes this multi-byte panic. Issue #50 ("Symbol parser panics on symbol `scip-go . . . `", 2022, closed) was a *different* panic in the old v0.1.0 parser; PR #258 ("Speed up symbol parsing by minimizing allocations") introduced `symbolParserV2` where this bug lives. **Action**: the fix PR is upstream-worthy — open a scip-code/scip issue with the repro + one-line fix, land the fix locally first. [CITED: https://github.com/scip-code/scip/issues/50, https://github.com/scip-code/scip/pull/258]

**Blast-radius guardrails**: run the full `bindings/go/scip` suite (`go test ./...` and `-tags asserts`) plus `./memtest` (the parser is allocation-optimized; the memtest low-memory test protects the slot-reuse logic — do not refactor beyond the one line). `scip lint`/`scip snapshot` call `ParseSymbol` per occurrence symbol [VERIFIED: CONCERNS.md:61 — cmd/scip/lint.go:253, symbol_formatter.go:57-63], so this fix unblocks Swift Unicode emission repo-wide.

## Module Wiring Checklist

Exact files to touch for plan 01-01 (all precedents verified this session):

1. **Create `swift/go.mod`** — copy the reprolang shape verbatim [VERIFIED: reprolang/go.mod]:
   ```
   module github.com/scip-code/scip/swift
   go 1.25.0
   replace github.com/scip-code/scip/bindings/go/scip => ../bindings/go/scip
   require (
       github.com/scip-code/scip/bindings/go/scip v0.0.0-00010101000000-000000000000
       github.com/stretchr/testify v1.11.1
       pgregory.net/rapid v1.3.0
   )
   ```
   Then `cd swift && GOWORK=off go mod tidy` (fetches go.sum). Add `github.com/sourcegraph/jsonrpc2 v0.2.2` when the spike driver lands (01-03).
2. **`go.work`** — run `go work use ./swift` from repo root (or hand-add `swift` to the `use (...)` block). Current content is exactly `go 1.25.0` + `use ( . bindings/go/scip reprolang )` [VERIFIED: go.work:1-7]. Commit the resulting `go.work.sum` additions.
3. **`.github/workflows/ci.yaml`** — three edits in the `fix` job:
   - paths-filter `relevant` list: add `- 'swift/go.mod'` and `- 'swift/go.sum'` (current list at lines 34-43 covers the other three modules' manifests) [VERIFIED: ci.yaml:34-43].
   - tidy loop: `for dir in bindings/go/scip . reprolang swift; do` (currently `bindings/go/scip . reprolang`, line 78; note `GOWORK: 'off'` env already set) [VERIFIED: ci.yaml:71-80].
   - nix-update loop: add `checks.x86_64-linux.swift` to the attr list (lines 90-94) because the new check uses `buildGoModule` with a `vendorHash`.
   - **No new job needed for the check itself**: the `list` job enumerates `nix eval .#checks.x86_64-linux --apply builtins.attrNames` and the `checks` matrix builds each [VERIFIED: ci.yaml:132-157] — a new checks.nix attribute is auto-discovered.
4. **`checks.nix`** — add a `swift` attribute mirroring `go-bindings` (the simplest Go entry; reprolang's entry additionally asserts package.json version + tree-sitter, neither applies) [VERIFIED: checks.nix:47-61]:
   ```nix
   swift = pkgs.buildGoModule {
     pname = "scip-swift";
     inherit version;
     src = ./.;
     modRoot = "./swift";
     vendorHash = "sha256-…"; # first build prints the correct hash; paste it
     env.GOWORK = "off";
     subPackages = [ "." ];            # + "internal/…" packages as added
     installPhase = "touch $out";
   };
   ```
   Phase 1 needs **no** `pkgs.tree-sitter` buildInput (grammar is Phase 2) and **no version assert** (no published manifest — verified: all existing asserts read binding manifests that swift/ won't have).
5. **`flake.nix`** — no change required: `checks = import ./checks.nix { inherit pkgs version; }` picks the new attribute up automatically [VERIFIED: flake.nix:88-90]. A `packages.scip-swift` entry is Phase 5 (release) work.
6. **`swift/README.md`** — prettier must pass on it (`nix develop` provides prettier).
7. **Sanity sequence for the executor**: from repo root `go build ./... && go test ./...` (workspace mode covers swift/); inside `swift/` `GOWORK=off go test ./...`; then `nix flake check` (expects: formatting, all existing checks, new `swift` check).

Gotchas: (a) `buildGoModule` under Nix builds with `GOWORK=off` — the `replace` directive must be relative (`../bindings/go/scip`) exactly as reprolang does, or the Nix build breaks; (b) gofmt/goimports run over the whole repo — new files must be formatted before committing or the `formatting` check fails; (c) `version.txt` sync: **not needed** — verified no assert reads `swift/`; do not invent one; (d) do not add `swift/**` to the release workflow's paths (releases trigger on `cmd/scip/version.txt` only).

## SourceKit-LSP Spike Protocol

Plan 01-03 must produce **measured evidence + an explicit go/no-go verdict** on the LSP semantic path for v1 (success criterion 5). Concrete, runnable design:

### Capability inventory spike

**Driver**: a small Go program (`swift/spikes/lsp_probe/`) using `sourcegraph/jsonrpc2` v0.2.2 over stdio. Sequence:

1. **Discover & record toolchain**: `xcrun -f sourcekit-lsp` (PATH fallback), capture `sourcekit-lsp --version`, `swift --version`, `xcodebuild -version` into the evidence JSON.
2. **Fixture** `swift/spikes/fixture/` — a SwiftPM package (`swift package init --type library` then add) exercising every hard case:
   - overloads: `func f(_ i: Int)`, `f(_ s: String)`, `f(_ i: Int, _ j: Int)` — assert distinct USRs;
   - cross-file same-module extension: `extension Shape { func area2() }` in a second file — `references`/`symbolInfo` must attribute to `Shape`;
   - retroactive extension: `extension String { func spikeFlag() }`;
   - generics: `func id<T>(_ t: T) -> T`, `struct Box<T>`, generic method;
   - protocol + witness: `protocol Drawable { func draw() }`, `struct Circle: Drawable { func draw() }`, call through the protocol existential (exercises `isDynamic`/`receiverUsrs`);
   - operator: `static func + (lhs: Vec, rhs: Vec) -> Vec`;
   - accessors: computed `var` with `get`/`set`;
   - Unicode: `func 🚀()`, `var π`, a string literal with emoji (position-encoding probe);
   - class inheritance + override.
3. **Handshake**: `initialize` with `rootUri` = fixture, client capabilities including `window.workDoneProgress`; **record the full `InitializeResult.capabilities`** (does it advertise callHierarchyProvider/typeHierarchyProvider? negotiated positionEncoding?). Send `initialized`.
4. **Readiness gate**: poll the `sourcekit/isIndexing` extension request (empty params; result `{indexing: bool}`) until false or timeout, then **verify by probing**: `textDocument/symbolInfo` at a known definition must return a non-empty USR before harvesting (guards the cold-index trap). Record time-to-ready cold vs warm.
5. **Harvest per file**: `didOpen` (text from disk, version 1) → `textDocument/documentSymbol` → for each def range: `textDocument/symbolInfo` (record `name`, `containerName`, `usr`, `kind`, `isDynamic`, `receiverUsrs`, `systemModule.moduleName`), `textDocument/definition`, `textDocument/references {includeDeclaration: true}`. For one call site and one type: `prepareCallHierarchy`+`incomingCalls`/`outgoingCalls`, `prepareTypeHierarchy`+`supertypes`/`subtypes`.
6. **Output**: JSONL evidence (one record per request/response with latency) + a findings document `swift/spikes/2026-08-sourcekit-lsp-findings.md` with the capability inventory table.

Extension request shapes verified against the official contributor docs [CITED: https://github.com/swiftlang/sourcekit-lsp/blob/main/Contributor%20Documentation/LSP%20Extensions.md]:
- `textDocument/symbolInfo` — params `{textDocument, position}` → `SymbolDetails[]` with `name`, `containerName`, `usr`, `bestLocalDeclaration`, `kind`, `isDynamic`, `receiverUsrs`, `isSystem`, `systemModule: {moduleName, groupName?}`. Documented as the machine-to-machine surface ("to allow one LSP backend to query another LSP backend") — exactly our use case.
- `sourcekit/isIndexing` — params `{}` → `{indexing: bool}` (experimental; renamed from `sourceKit/_isIndexing`, old name still accepted).
- `sourcekit/workspace/symbolNames` — params `{}` → `{names: string[]}` (flat deduplicated workspace symbol names).
- `sourcekit/workspace/triggerReindex` — recovery path if results look stale.

### Performance spike

- **Fixture**: `swift/spikes/perf/` — a generator script emitting a ~500-file SwiftPM package (e.g. 500 files × ~15 defs × ~10 refs each; committed generator + Package.swift, `.build/` git-ignored).
- **Measure** (all into evidence JSON): cold time-to-ready (background indexing from clean clone), warm time-to-ready, total harvest wall time (documentSymbol + per-unique-def references), request count, requests/sec, p50/p95 request latency, server restarts/crashes, symbols+occurrences harvested, RSS of the server at end.
- **Compare against the harvest discipline** the architecture prescribes: one documentSymbol pass per file + one references pass per unique definition, bounded concurrency — the spike driver must implement exactly that discipline so the numbers are predictive, not adversarial.

### Go/no-go criteria (thresholds ASSUMED — planner/user to confirm)

- **GO** if: (a) symbolInfo returns distinct USRs for ≥95% of fixture defs including every overload; (b) cross-file extension members resolve to the extended type's container; (c) retroactive extension member containerName reflects the extended type; (d) full perf-fixture harvest ≤ 30 min cold / ≤ 10 min warm with no server crash requiring restart; (e) call/type-hierarchy requests return usable data (not hard errors).
- **NO-GO → index-store escape hatch** (v2 design, per project research): the symbol/emission layers survive unchanged; the bulk harvest swaps to reading the compiler index store (`.build/index/store`) via a Swift helper subprocess or cgo libIndexStore, keeping LSP for live queries only.
- Record the verdict + numbers in the findings doc; STATE.md gets the decision.

## Don't Hand-Roll

1. **Symbol string formatting & escaping** — never `fmt.Sprintf` descriptors; build `*scip.Symbol` and use `scip.VerboseSymbolFormatter.FormatSymbol` (backtick escaping, `"."` empty-placeholder handled) [VERIFIED: symbol_formatter.go:139-180].
2. **Symbol parsing/validation** — `scip.ParseSymbol` / `ParseSymbolUTF8With` / `ValidateSymbolUTF8`; fix, don't fork [VERIFIED: symbol.go:30-74].
3. **Property testing** — `pgregory.net/rapid` v1.3.0, already the repo's pattern; do not pull a second property lib.
4. **Module scaffolding** — copy `reprolang/go.mod` + `go.work`/`checks.nix`/`ci.yaml` patterns verbatim (see wiring checklist).
5. **JSON-RPC/LSP framing** — `sourcegraph/jsonrpc2` v0.2.2 for the spike (project-decided; go.lsp.dev is stale per project research).
6. **CI/Nix check registration** — the flake auto-discovers `checks` attributes; do not add a bespoke CI job for Go tests.
7. **Snapshot harness** (Phase 2 preview) — `testutil.SnapshotTest` with `testdata/snapshots/{input,output}`; do not build a parallel golden system.
8. **Toolchain discovery** — `xcrun -f sourcekit-lsp`, never hardcode `/usr/bin/sourcekit-lsp`.

## Common Pitfalls

| Pitfall | Prevention |
|---|---|
| Parser fix regresses allocation optimizations or breaks `assert` preconditions under `-tags asserts` | Keep the fix to the single guard line; run `go test ./... ./memtest -tags asserts` in bindings/go/scip; memtest not in parallel. |
| Hand-concatenated symbol strings drift from the grammar (escaping bugs the formatter already solved) | Namer constructs `*scip.Symbol`; forbid string concatenation of descriptor suffixes in review. |
| Overload indices non-deterministic between the two paths (semantic vs fallback) | Rule frozen in the spec: source-position order within (module, container, name); property/golden tests in Phase 2 assert dual-path parity. |
| Unicode local names break the `local <simple-identifier>` form | Sanitize + ordinal counter (mapping row 28); round-trip test with emoji/CJK local names. |
| Retroactive same-name extension members from two modules collide on one symbol string | Disambiguator mechanism applies to Terms too (row 22); add a spec test case. |
| Getter/setter descriptor collides with a same-named zero-arg method | Overload index treats accessor and method as one ordering group (row 14/15); flagged ASSUMED — planner to confirm. |
| `swift/` module invisible to the CI fix-job → module bumps break Nix builds | ci.yaml paths-filter + tidy loop + nix-update attr updated in the SAME commit as go.mod (Pitfall 13 of project research). |
| Nix vendorHash churn on first build | Expected buildGoModule flow: first `nix build` prints the hash; paste and re-run; the fix-job keeps it fresh afterwards. |
| gofmt/prettier/nixfmt failures on new files | Run `nix develop` formatters before commit; `swift/README.md` must be prettier-clean (semi:false, singleQuote). |
| go.work.sum drift surprises | Run `go work use ./swift`, commit go.work + go.work.sum together. |
| `Descriptor_Package` alias (`=1` deprecated) used instead of `Descriptor_Namespace` | Use `Descriptor_Namespace` [VERIFIED: scip.proto:209-214]. |
| Spike queries a cold index → silently-empty results recorded as "LSP can't do X" | Readiness gate + known-symbol probe before any capability conclusion; cold and warm runs recorded separately. |
| Spike becomes a product | It's throwaway evidence tooling under `swift/spikes/`; the Phase-3 client is built fresh against the findings. |

## Code Examples

**Rapid round-trip test (pattern from `bindings/go/scip/position_test.go:64-90` + `sort_test.go:129-146`)** — the core of plan 01-02's validation:

```go
package symbol

import (
	"testing"

	"github.com/scip-code/scip"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func genName() *rapid.Generator[string] {
	// Bias toward names that stress the grammar: operators, emoji, backticks.
	return rapid.SampledFrom(
		"foo", "init", "`==`", "🚀", "π", "F⃗", "back``tick", "$0", "a-b", "_x",
	).Filter(func(s string) bool { return len(s) > 0 })
}

func genInput() *rapid.Generator[SymbolInput] {
	return rapid.Custom(func(t *rapid.T) SymbolInput {
		return SymbolInput{
			Module:        rapid.SampledFrom("MyMod", "App", "🄼odule").Draw(t, "module"),
			ContainerPath: rapid.SliceOfN(genContainer(), 0, 3).Draw(t, "path"),
			Name:          genName().Draw(t, "name"),
			Kind:          rapid.SampledFrom(KindMethod, KindType, KindOperator).Draw(t, "kind"),
			OverloadIndex: rapid.IntRange(0, 5).Draw(t, "overload"),
		}
	})
}

func TestNamerRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		input := genInput().Draw(t, "input")
		s, err := Symbol(input)
		require.NoError(t, err)

		parsed, err := scip.ParseSymbol(s)
		require.NoError(t, err, "symbol %q must parse", s)

		formatted := scip.VerboseSymbolFormatter.FormatSymbol(parsed)
		require.Equal(t, s, formatted, "parse->format must be the identity")

		require.Equal(t, scip.Scheme("scip-swift"), parsed.Scheme)
	})
}
```

**Formatter round-trip precedent** [VERIFIED: bindings/go/scip/symbol_formatter_test.go:31-50]:

```go
func TestSymbolFormatterRoundTrip(t *testing.T) {
	tests := []struct{ Symbol string }{
		{"lsif-java maven package 1.0.0 java/io/File#Entry.method(+1).(param)[TypeParam]"},
		{"rust-analyzer cargo std 1.0.0 macros/println!"},
		{"zeroth  escape first  escape second  escape third  escape `github.com/foo/``bar/n2/n3/n4`/T#f()."},
	}
	for _, test := range tests {
		t.Run(test.Symbol, func(t *testing.T) {
			formatted, err := VerboseSymbolFormatter.Format(test.Symbol)
			require.Nil(t, err)
			if diff := cmp.Diff(test.Symbol, formatted); diff != "" {
				t.Fatalf("unexpected response (-want +got):\n%s", diff)
			}
		})
	}
}
```

**go.work entry** (after `go work use ./swift`) [current VERIFIED: go.work:1-7]:

```
go 1.25.0

use (
	.
	bindings/go/scip
	reprolang
	swift
)
```

**checks.nix entry** — see Module Wiring item 4 (mirror of `go-bindings`, lines 47-61).

**Namer core (illustrative)**:

```go
func Symbol(in SymbolInput) (string, error) {
	sym := &scip.Symbol{
		Scheme: Scheme,
		Package: &scip.Package{
			Manager: ManagerSwiftPM, // or ManagerSystem
			Name:    in.Module,
			Version: ".",            // writeEscapedPackage maps "" -> "."; parser maps back
		},
	}
	sym.Descriptors = appendDescriptors(sym.Descriptors, in) // Type '#' / Method '().'/'+N' / Term '.'
	// Verify before returning — cheap and catches spec drift at the source:
	s := scip.VerboseSymbolFormatter.FormatSymbol(sym)
	if _, err := scip.ParseSymbol(s); err != nil {
		return "", fmt.Errorf("namer produced unparseable symbol %q: %w", s, err)
	}
	return s, nil
}
```

## Validation Architecture

*(This section is the source for VALIDATION.md.)*

**What to validate, how, and the dimensions:**

### Dimension A — Symbol scheme correctness (SYM-01, SYM-02)

- **Property tests** (`swift/internal/symbol/roundtrip_test.go`): namer → `scip.ParseSymbol` → `VerboseSymbolFormatter.FormatSymbol` == identity, over rapid-generated inputs including emoji, operators, backticks, multi-byte tails; plus local-symbol sanitization round-trips (`IsLocalSymbol` true, `ParseSymbol` succeeds).
- **Golden table tests** (`namer_test.go`): one case per mapping-table row above (all 31 rows where a string is defined), asserting the exact symbol string; extension cases include same-file, cross-file, and retroactive variants (SYM-02 success criterion 2).
- **Check**: `go test ./internal/symbol/...` green from both repo root (workspace) and `swift/` (`GOWORK=off`).

### Dimension B — Parser robustness (SYM-01 prerequisite)

- **Regression table**: the three multi-byte-tail inputs appended to `TestParseSymbolError` — no panic, error returned.
- **Fuzz**: `FuzzParseSymbol` seed corpus (ASCII grammar samples + multi-byte tails + escaped Unicode) passes under plain `go test`; optional local `-fuzz=.` soak.
- **Non-regression**: full `bindings/go/scip` suite incl. `-tags asserts` and `./memtest` green (success criterion 3: "all existing module tests stay green").

### Dimension C — Repo health / wiring (success criterion 4)

- `go build ./... && go test ./...` at repo root (workspace mode) — existing modules + `swift/` all pass.
- `nix flake check` — formatting, all pre-existing checks, and the new `swift` check green; the CI `list`/`checks` matrix picks the `swift` attribute up automatically (verify locally via `nix eval .#checks.x86_64-linux --apply builtins.attrNames`).
- ci.yaml diff review: paths-filter + tidy loop + nix-update attr contain `swift`.

### Dimension D — Spike evidence (success criterion 5)

- `swift/spikes/` contains the fixture, the generator, JSONL evidence with measured latencies/counts, and the findings doc with the capability inventory (overloads, retroactive extensions, generics, witnesses, hierarchies) and the explicit go/no-go verdict against the pre-agreed thresholds.
- Evidence is reproducible: a "how to re-run" section in the findings doc (macOS + Xcode required; documented as runtime-only).

**Boundary conditions to test explicitly**: symbol strings ending in multi-byte runes (post-fix: parse error is *correct* only for invalid tails — valid escaped forms like `` `🚀`. `` must parse); `"."`-placeholder version round-trip; scheme does not start with `local`; `Descriptor_Namespace` (not deprecated `Package`).

---

## Confidence Gaps (ASSUMED items needing planner/user confirmation)

1. **Perf GO/NO-GO thresholds** (≤30 min cold / ≤10 min warm for ~500 files, ≥95% USR coverage) — proposed numbers; no measured baseline exists yet.
2. **Package header**: manager `"swiftpm"` (requirement wording) + version `"."` placeholder; system modules manager `"swift"` — grounded in scip-java/scip-python precedents but a naming decision.
3. **Accessor symbols** (getter = property-shaped Method + Kind.Getter; setter = `` `foo=`() ``) — Ruby-style prescription; alternative is deferring accessor symbols to a later phase.
4. **Retroactive extension attribution to the owner module's package** — matches SYM-02's find-References rationale; alternative is declaring-module attribution.
5. **Local id sanitization scheme** (sanitized name + `_n` counter vs pure `v<n>` ordinals).
6. **Spike deliverable location** (`swift/spikes/` in-repo vs `.planning/`) — prescribed in-repo for team visibility.

## Sources

**In-repo (all read + verified this session; line numbers cited inline):** `scip.proto` (symbol grammar 148-230, Kinds 284-427 with Swift comments at 321/365/367/373/375/397, Relationships 465-517, `Language.Swift = 2` at 939, PositionEncoding 108-146), `bindings/go/scip/{symbol.go, symbol_parser.go, symbol_formatter.go, identifier.go, symbol_test.go, symbol_formatter_test.go, position_test.go, sort_test.go, parse_test.go, go.mod}`, `bindings/go/scip/testutil/snapshot_testing.go`, `go.work`, `reprolang/{go.mod, repro/namer.go, repro/snapshot_test.go}`, `checks.nix`, `flake.nix`, `.github/workflows/ci.yaml`, `cmd/scip/version.txt`, `.planning/codebase/CONCERNS.md`, `.planning/{PROJECT,STATE,REQUIREMENTS}.md`, `.planning/research/{SUMMARY,STACK,ARCHITECTURE,PITFALLS}.md`, `AGENTS.md`. Empirical: panic reproduced and one-line fix verified+reverted on this tree (2026-08-16).

**External:** [scip-java snapshots (real symbol strings incl. `List#of(+2).`)](https://github.com/scip-code/scip-java), [scip-java issue trail](https://github.com/scip-code/scip/issues/50), [scip-python (ScipSymbol.ts: `scip-python python ${name} ${version}`)](https://github.com/sourcegraph/scip-python), [SourceKit-LSP LSP Extensions (symbolInfo, isIndexing, symbolNames, triggerReindex)](https://github.com/swiftlang/sourcekit-lsp/blob/main/Contributor%20Documentation/LSP%20Extensions.md), [LSP 3.17 spec](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/).
