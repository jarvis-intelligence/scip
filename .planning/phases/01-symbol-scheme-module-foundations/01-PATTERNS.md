# Pattern Mapping: Phase 1 — Symbol Scheme & Module Foundations

**Source:** derived from `01-RESEARCH.md` (Module Wiring Checklist, Architecture Patterns, Code Examples) verified against the tree on 2026-08-16. Every excerpt below is verbatim with its real `file:line` range — planners can point executors at the cited lines directly.

---

## File Classification

| # | File | Status | Role | Data flow | Analog |
|---|------|--------|------|-----------|--------|
| 1 | `swift/go.mod` | NEW | Module manifest for `github.com/scip-code/scip/swift` | Declares deps; `replace` → `../bindings/go/scip` | `reprolang/go.mod:1-12` |
| 2 | `swift/go.sum` | NEW (generated) | Lockfile | Produced by `GOWORK=off go mod tidy` | `reprolang/go.sum` (never hand-written) |
| 3 | `swift/README.md` | NEW | Module docs, runtime prerequisites | Docs; must be prettier-clean | `reprolang/README.md` |
| 4 | `swift/cmd/scip-swift/main.go` | NEW | Stub CLI entry (real surface Phase 2+) | Process entry point; imports nothing yet | `cmd/scip/main.go:15-20` (minimal `main`) |
| 5 | `swift/internal/symbol/scheme.go` | NEW | Frozen spec constants (`Scheme`, managers, `DeclKind`) + normative doc comment | Pure data; no imports beyond stdlib | `bindings/go/scip/symbol.go:9-21` + `bindings/go/scip/identifier.go:1-15` |
| 6 | `swift/internal/symbol/namer.go` | NEW | Pure namer: `SymbolInput` → `*scip.Symbol` → string | Consumes `bindings/go/scip` (Symbol, VerboseSymbolFormatter, ParseSymbol) | `bindings/go/scip/symbol_formatter.go:65-91` (build + format path); `reprolang/repro/namer.go:87-103` (role precedent — **its Sprintf is the anti-pattern, see flag below**) |
| 7 | `swift/internal/symbol/namer_test.go` | NEW | Golden table tests, one case per mapping-table row | Test → namer → exact string | `bindings/go/scip/symbol_test.go:10-134` |
| 8 | `swift/internal/symbol/roundtrip_test.go` | NEW | rapid property round-trip tests | Test → namer → `scip.ParseSymbol` → `VerboseSymbolFormatter.FormatSymbol` == identity | `bindings/go/scip/position_test.go:64-101` + `sort_test.go:129-158` |
| 9 | `bindings/go/scip/symbol_fuzz_test.go` | NEW | Native Go fuzz `FuzzParseSymbol` (seed corpus runs in plain `go test`) | Fuzz → `ParseSymbol` (never panics; success ⇒ round-trips) | `bindings/go/scip/symbol_test.go:136-155` (seed corpus source); replaces the legacy `gofuzz` import at `parse_test.go:13` |
| 10 | `swift/spikes/lsp_probe/` (main.go) | NEW | SourceKit-LSP probe driver (`sourcegraph/jsonrpc2` over stdio) | stdio JSON-RPC → sourcekit-lsp → JSONL evidence | `cmd/scip/main.go:15-20` (main skeleton); **no in-repo jsonrpc2 precedent — follow RESEARCH.md Spike Protocol** |
| 11 | `swift/spikes/fixture/` (Package.swift + Sources) | NEW | SwiftPM capability fixture (overloads, extensions, generics, Unicode) | Read-only input for the probe | `reprolang/testdata/` (committed fixture-data layout precedent) |
| 12 | `swift/spikes/perf/` (generator + Package.swift) | NEW | ~500-file perf fixture generator; `.build/` git-ignored | Generator → fixture tree → probe | Same as #11 |
| 13 | `swift/spikes/2026-08-sourcekit-lsp-findings.md` | NEW | Evidence + go/no-go verdict doc | Docs; consumed by STATE.md decision | `.planning/research/*.md` / `01-RESEARCH.md` doc style |
| 14 | `go.work` | MODIFIED | Workspace manifest | Add 4th module to `use` block | `go.work:1-7` (itself — append `swift`) |
| 15 | `.github/workflows/ci.yaml` | MODIFIED | CI wiring: paths-filter, tidy loop, nix-update attr list | Build/CI | Its own existing entries (cited below) |
| 16 | `checks.nix` | MODIFIED | Nix check matrix: new `swift` buildGoModule attribute | Build/Nix; auto-discovered by flake + CI `list` job | `checks.nix:47-61` (`go-bindings`) |
| 17 | `bindings/go/scip/symbol_parser.go` | MODIFIED | One-line panic fix in `peekNext` | Library hot path | `bindings/go/scip/symbol_parser.go:374-379` (the bug site itself) |
| 18 | `bindings/go/scip/symbol_test.go` | MODIFIED | Append multi-byte-tail regression inputs to `TestParseSymbolError` | Test → `ParseSymbol` | `bindings/go/scip/symbol_test.go:136-155` (existing harness) |
| 19 | `go.work.sum` | MODIFIED (generated) | Workspace lockfile | Updated by `go work use ./swift`; commit together with `go.work` | — |

**No change needed:** `flake.nix` — `checks = import ./checks.nix { inherit pkgs version; }` (`flake.nix:88-90`) picks the new attribute up automatically. No `version.txt` sync (asserts read only published binding manifests; `swift/` has none — verified in RESEARCH.md item 7).

---

## Pattern Assignments

### 1. `swift/go.mod` ← `reprolang/go.mod:1-12`

Copy the shape verbatim; swap module path and deps (rapid instead of tree-sitter/autogold; add `sourcegraph/jsonrpc2 v0.2.2` only when the spike driver lands — plan 01-03).

**Excerpt — `/Users/ddphuong/Projects/jarvis-ai/scip/reprolang/go.mod:1-12`:**

```go
module github.com/scip-code/scip/reprolang

go 1.25.0

replace github.com/scip-code/scip/bindings/go/scip => ../bindings/go/scip

require (
	github.com/hexops/autogold/v2 v2.3.1
	github.com/scip-code/scip/bindings/go/scip v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.11.1
	github.com/tree-sitter/go-tree-sitter v0.25.0
)
```

What to copy:
- Module path style `github.com/scip-code/scip/<dir>` (→ `github.com/scip-code/scip/swift`).
- `go 1.25.0` pin — must match `go.work:1`.
- The **relative** `replace ... => ../bindings/go/scip` — mandatory: Nix `buildGoModule` and CI both build with `GOWORK=off`, an absolute path breaks both (RESEARCH gotcha (a)).
- The placeholder version `v0.0.0-00010101000000-000000000000` on the replaced dep.
- Dependency versions already pinned in-repo: `pgregory.net/rapid v1.3.0` and `testify v1.11.1` — see `bindings/go/scip/go.mod:12-17`.
- Do NOT hand-write `go.sum` — `cd swift && GOWORK=off go mod tidy` generates it (never commit a module without it; CI's tidy loop expects it present).

### 2. `swift/README.md` ← `reprolang/README.md`

`reprolang/README.md` is the repo's only module-level README and is prettier-clean under the repo `.prettierrc` (`semi: false, trailingComma: "es5", singleQuote: true, useTabs: false, arrowParens: "avoid"` — `/Users/ddphuong/Projects/jarvis-ai/scip/.prettierrc:1-5`). Copy its tone/structure: intro paragraph stating what the module is for and why it exists, short sections, fenced `bash` blocks for commands, and a "Running tests" section. The `swift/README.md` must state runtime prerequisites (Xcode ≥ 6.1 / sourcekit-lsp is runtime-only, never a build dep). Prettier runs in the `formatting` Nix check (`checks.nix:38`: `prettier --check '**/*.{ts,js(on)?,md,yml}'`) — no `;`, single quotes in shell examples.

### 3. `swift/cmd/scip-swift/main.go` ← `cmd/scip/main.go:15-20`

Phase 1 needs only a stub so `buildGoModule` has something to build. Copy the minimal main:

**Excerpt — `/Users/ddphuong/Projects/jarvis-ai/scip/cmd/scip/main.go:15-20`:**

```go
func main() {
	app := scipApp()
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
```

For the stub, reduce to `package main` + `func main() {}` (or a one-line `fmt.Println` placeholder). The full three-tier `xxxCommand()/xxxMain()/xxxMainPure()` urfave/cli pattern (AGENTS.md item 9) applies from Phase 2 — do not scaffold it now.

### 4. `swift/internal/symbol/scheme.go` ← `bindings/go/scip/symbol.go:9-21` + `identifier.go:1-15`

Small constants/predicate file, one concern, heavy godoc. Copy the comment discipline:

**Excerpt — `/Users/ddphuong/Projects/jarvis-ai/scip/bindings/go/scip/symbol.go:9-21`:**

```go
// IsGlobalSymbol returns true if the symbol is obviously not a local symbol.
//
// CAUTION: Does not perform full validation of the symbol string's contents.
func IsGlobalSymbol(symbol string) bool {
	return !IsLocalSymbol(symbol)
}

// IsLocalSymbol returns true if the symbol is obviously not a global symbol.
//
// CAUTION: Does not perform full validation of the symbol string's contents.
func IsLocalSymbol(symbol string) bool {
	return strings.HasPrefix(symbol, "local ")
}
```

What to copy: every exported identifier (`Scheme`, `ManagerSwiftPM`, `ManagerSystem`, `DeclKind` values) gets a godoc comment starting with the identifier name; `CAUTION:` lines for sharp edges (e.g. "scheme must not start with `local`" — scip.proto:159). File holds only the frozen spec constants + the `DeclKind` enum + the normative scheme doc comment (determinism rule, canonical `#if` policy, dual-path identity rule — RESEARCH "Frozen-spec extras"). No logic beyond predicates.

### 5. `swift/internal/symbol/namer.go` ← `bindings/go/scip/symbol_formatter.go:65-91` (positive) + `reprolang/repro/namer.go:87-103` (role precedent)

The namer builds `*scip.Symbol` values and formats via `scip.VerboseSymbolFormatter.FormatSymbol` — never string concatenation.

**Excerpt A — build-then-format path, `/Users/ddphuong/Projects/jarvis-ai/scip/bindings/go/scip/symbol_formatter.go:65-91`:**

```go
func (f *SymbolFormatter) FormatSymbol(symbol *Symbol) string {
	b := &strings.Builder{}
	if f.IncludeScheme(symbol.Scheme) {
		writeEscapedPackage(b, symbol.Scheme)
	}
	if symbol.Package != nil {
		if f.IncludePackageManager(symbol.Package.Manager) {
			buffer(b)
			writeEscapedPackage(b, symbol.Package.Manager)
		}
		if f.IncludePackageName(symbol.Package.Name) {
			buffer(b)
			writeEscapedPackage(b, symbol.Package.Name)
		}
		if f.IncludePackageVersion(symbol.Package.Version) {
			buffer(b)
			writeEscapedPackage(b, symbol.Package.Version)
		}
	}
	...
	return b.String()
}
```

**Excerpt B — why empty Version must be `"."`, `symbol_formatter.go:131-137`:**

```go
func writeEscapedPackage(b *strings.Builder, name string) {
	if name == "" {
		name = "."
	}

	writeGenericEscapedIdentifier(b, name, ' ')
}
```

The parser maps it back (`symbol_parser.go:88,95,102` — `sw.normalize(".", "")`), so `Version: "."` round-trips by construction.

**Excerpt C — escaping is free, `symbol_formatter.go:139-159`:**

```go
func writeSuffixedDescriptor(b *strings.Builder, identifier string, suffixes ...rune) {
	escape := false
	for _, ch := range identifier {
		if !isSimpleIdentifierCharacter(ch) {
			escape = true
			break
		}
	}

	if escape {
		b.WriteRune('`')
		writeGenericEscapedIdentifier(b, identifier, '`')
		b.WriteRune('`')
	} else {
		b.WriteString(identifier)
	}

	for _, suffix := range suffixes {
		b.WriteRune(suffix)
	}
}
```

with the charset (`identifier.go:3-5`): `c == '_' || c == '+' || c == '-' || c == '$' || ASCII letters/digits`. `+`/`-`/`$` never escape; `==`, `<->`, `=`, `🚀`, `π` auto-backtick. The namer adds zero escaping code of its own.

**Excerpt D — role precedent + ANTI-PATTERN flag, `/Users/ddphuong/Projects/jarvis-ai/scip/reprolang/repro/namer.go:87-96`:**

```go
// newGlobalSymbol returns an SCIP symbol for the given definition.
func newGlobalSymbol(pkg *scip.Package, document *reproSourceFile, name *identifier) string {
	return fmt.Sprintf(
		"reprolang repro_manager %v %v %v/%v",
		pkg.Name,
		pkg.Version,
		document.Source.RelativePath,
		name.value,
	)
}
```

Copy the *file role* (a `namer.go` in the indexer package that owns all symbol-string construction, with a `newGlobalSymbol`-style entry point) and the local-symbol form below — **do NOT copy the `fmt.Sprintf` construction**; that is exactly the hand-rolling RESEARCH forbids ("Don't Hand-Roll" #1). Also copy the validate-before-use habit from the same file (`namer.go:19-23`: every produced symbol goes through `scip.ParseSymbol`, errors joined with `errors.Join` + `%w` context).

**Excerpt E — local symbol form, `symbol_parser.go:120-127` + the canonical construction (`symbol_test.go:16-19`):**

```go
	suffix := strings.TrimPrefix(s, "local ")
	if len(suffix) == len(s) {
		return false, nil
	}
	if len(suffix) == 0 || !isSimpleIdentifier(suffix) {
		return false, expectedSimpleIdentifierError{""}
	}
```

```go
	{Symbol: "local a", Expected: &Symbol{
		Scheme:      "local",
		Descriptors: []*Descriptor{{Name: "a", Suffix: Descriptor_Local}},
	}},
```

`LocalSymbol(sourceName, ordinal)` builds exactly `{Scheme: "local", Descriptors: [{Name: sanitizedID, Suffix: Descriptor_Local}]}` and lets the formatter render `local <id>`. Error handling in namer.go: `fmt.Errorf("namer produced unparseable symbol %q: %w", s, err)` wrap style (matches `cmd/scip/option_from.go` / RESEARCH's illustrative core).

### 6. `swift/internal/symbol/namer_test.go` ← `bindings/go/scip/symbol_test.go:10-134`

Golden table, one anonymous `type test struct` declared inside the test func, subtests named by the symbol string, `cmp.Diff` on strings.

**Excerpt — `/Users/ddphuong/Projects/jarvis-ai/scip/bindings/go/scip/symbol_test.go:10-28` (table head) and `125-134` (loop):**

```go
func TestParseSymbol(t *testing.T) {
	type test struct {
		Symbol   string
		Expected *Symbol
	}
	tests := []test{
		{Symbol: "local a", Expected: &Symbol{
			Scheme:      "local",
			Descriptors: []*Descriptor{{Name: "a", Suffix: Descriptor_Local}},
		}},
```

```go
	for _, test := range tests {
		t.Run(test.Symbol, func(t *testing.T) {
			obtained, err := ParseSymbol(test.Symbol)
			require.Nil(t, err)
			if diff := cmp.Diff(test.Expected.String(), obtained.String()); diff != "" {
				t.Fatalf("unexpected response (-want +got):\n%s", diff)
			}
		})
	}
}
```

What to copy: table struct `{ Input SymbolInput; Expected string }`, one row per mapping-table case from RESEARCH (all 31 rows where a string is defined — including `init(+1).`, escaped `` `area=`() ``, `[T]`, `(x)`, retroactive-extension variants), `t.Run` keyed on the expected string, `require` + `cmp.Diff` imports exactly as in `symbol_test.go:3-8`. Constructor-from-`*Symbol` literal style comes from `symbol_formatter_test.go:12-24`.

### 7. `swift/internal/symbol/roundtrip_test.go` ← `bindings/go/scip/position_test.go:64-101` + `sort_test.go:129-158`

Generator functions return `*rapid.Generator[T]` built with `rapid.Custom` + `Draw(t, "name")`; tests use `rapid.Check(t, func(t *rapid.T) {...})`.

**Excerpt — `/Users/ddphuong/Projects/jarvis-ai/scip/bindings/go/scip/position_test.go:64-71`:**

```go
func genPosition() *rapid.Generator[Position] {
	return rapid.Custom(func(t *rapid.T) Position {
		return Position{
			Line:      rapid.Int32Range(0, 10).Draw(t, "Line"),
			Character: rapid.Int32Range(0, 10).Draw(t, "Character"),
		}
	})
}
```

**Excerpt — `/Users/ddphuong/Projects/jarvis-ai/scip/bindings/go/scip/position_test.go:89-101` (property + failure messages):**

```go
func TestComparePositionTransitive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		p1 := genPosition().Draw(t, "p1")
		p2 := genPosition().Draw(t, "p2")
		p3 := genPosition().Draw(t, "p3")

		if p1.Compare(p2) < 0 && p2.Compare(p3) < 0 {
			if !(p1.Compare(p3) < 0) {
				t.Errorf("%+v < %+v < %+v but !(%+v < %+v)", p1, p2, p3, p1, p3)
			}
		}
	})
}
```

**Excerpt — `/Users/ddphuong/Projects/jarvis-ai/scip/bindings/go/scip/sort_test.go:129-133` (string-field generator):**

```go
func genSymbolInfo() *rapid.Generator[*SymbolInformation] {
	return rapid.Custom(func(t *rapid.T) *SymbolInformation {
		return &SymbolInformation{Symbol: rapid.String().Draw(t, "symbol")}
	})
}
```

What to copy: naming `genX()` for generators; `rapid.SampledFrom(...)` / `rapid.SliceOfN(gen, 0, N)` / `rapid.IntRange` for the input space (RESEARCH's Code Examples block gives the filled-in `genName`/`genInput` — bias toward operators, emoji, backticks); assertion core = `Symbol(in)` → `ParseSymbol` → `VerboseSymbolFormatter.FormatSymbol` == identity (mirrors `TestSymbolFormatterRoundTrip`, `symbol_formatter_test.go:31-50`); `require.NoError(t, err, "symbol %q must parse", s)` message style. Import block: `testing`, `require`, `rapid` exactly as `position_test.go:3-8`. `pgregory.net/rapid v1.3.0` is already the repo pin — no new property lib.

### 8. `bindings/go/scip/symbol_parser.go` (fix) ← itself, `symbol_parser.go:374-379`

**Excerpt — the bug site, verbatim today:**

```go
func (z *symbolParserV2) peekNext() (rune, int32) {
	if z.byteIndex+1 < len(z.SymbolString) {
		return findRuneAtIndex(z.SymbolString, z.byteIndex+int(z.bytesToNextRune))
	}
	return 0, 0
}
```

The one-line change (`z.byteIndex+1` → `z.byteIndex+int(z.bytesToNextRune)` on the guard line only) was empirically verified on this tree by RESEARCH. Constraint: the guard's read path (`findRuneAtIndex`, lines 281-296) indexes `s[byteIndex]`..`s[byteIndex+3]` with documented pre-conditions — do not touch anything else; the allocation-optimized `descriptorsWriter` slot-reuse logic is protected by `./memtest`. `advanceOneByte` (line 469-476) documents that `z.byteIndex` may equal `len(z.SymbolString)` — the fixed guard must keep returning `(0, 0)` in that case.

### 9. `bindings/go/scip/symbol_test.go` (regression) ← its own `TestParseSymbolError`, `symbol_test.go:136-155`

**Excerpt — existing harness (append to this list, change nothing else):**

```go
func TestParseSymbolError(t *testing.T) {
	for _, symbolName := range []string{
		"",
		"lsif-java maven package 1.0.0",
		"lsif-java maven package 1.0.0 java/io/File#Entry.trailingstring",
		"lsif-java maven package 1.0.0 java/io/File#Entry.unrecognizedSuffix@",
		"lsif-java maven package 1.0.0 java/io/File#Entry.nonSimpλeIdentifier.",
		"lsif-java maven package 1.0.0 java/io/File#Entry.`unterminatedEscapedIdentifier",
		"lsif-java maven package 1.0.0 java/io/File#Entry.[UnterminatedDescriptorSuffix",
		"local 🧠",
		"local ",
		"local &&&",
	} {
		require.NotPanics(t, func() {
			if _, err := ParseSymbol(symbolName); err == nil {
				t.Fatalf("expected error from parsing %q", symbolName)
			}
		}, "panic when parsing %q", symbolName)
	}
}
```

Append the three panic repros: `"a b c d fooΩ"`, `"test . pkg . barΩ"`, `` "a b c d `foo`Ω" ``. The existing `require.NotPanics` + error-non-nil wrapper already asserts both properties. **Do not** add anything under `bindings/go/scip/memtest/` (sentinel `DO_NOT_ADD_NEW_TEST_FILES_HERE`).

### 10. `bindings/go/scip/symbol_fuzz_test.go` (NEW) ← corpus from `symbol_test.go` + replaces `parse_test.go:13` legacy fuzz

No native `testing.F` target exists in the repo yet — this file establishes it. Seed corpus = every string in `TestParseSymbol` (`symbol_test.go:15-124`) + `TestParseSymbolError` (`symbol_test.go:137-148`) + the multi-byte tails + escaped `` `🚀` `` forms; property = never panics (error ok) and on success `VerboseSymbolFormatter.Format(parsed) == input`. Use stdlib `testing.F` only — the existing `fuzz "github.com/google/gofuzz"` import at `bindings/go/scip/parse_test.go:13` is the legacy external lib this replaces; do not extend it. Seed corpus runs under plain `go test` (and thus under the Nix `go-bindings` check with `buildTags = ["asserts"]`); `-fuzz` soaks are local-only, never wired into CI.

### 11. `go.work` ← `go.work:1-7` (append `swift`)

**Excerpt — current file, verbatim:**

```
go 1.25.0

use (
	.
	bindings/go/scip
	reprolang
)
```

Produce the change with `go work use ./swift` from the repo root (keeps alphabetical-ish ordering as displayed), then commit `go.work` + `go.work.sum` together. Never add a `replace` here — replaces live in each leaf module's `go.mod` (reprolang precedent).

### 12. `.github/workflows/ci.yaml` ← its own three blocks

**Excerpt A — paths-filter (lines 34-43): add `- 'swift/go.mod'` / `- 'swift/go.sum'`:**

```yaml
          filters: |
            relevant:
              - 'go.mod'
              - 'go.sum'
              - 'bindings/go/scip/go.mod'
              - 'bindings/go/scip/go.sum'
              - 'reprolang/go.mod'
              - 'reprolang/go.sum'
              - 'bindings/typescript/package.json'
              - 'bindings/typescript/package-lock.json'
```

**Excerpt B — tidy loop (lines 71-80): add `swift` to the `for dir in` list:**

```yaml
      - name: Tidy Go modules
        if: steps.filter.outputs.relevant == 'true'
        env:
          GOWORK: 'off'
        # https://github.com/golang/go/issues/63901
        run: |
          set -euo pipefail
          for dir in bindings/go/scip . reprolang; do
            (cd "$dir" && nix develop --command go mod tidy)
          done
```

**Excerpt C — nix-update attr list (lines 82-97): add `checks.x86_64-linux.swift \` (vendorHash check):**

```yaml
      - name: Recompute vendor hashes with nix-update
        if: steps.filter.outputs.relevant == 'true'
        run: |
          set -euo pipefail
          # nix-update only auto-resolves packages.<system>.<attr>; for
          # attributes under `checks` we must pass the full dotted path.
          # One attribute per invocation; sequential to avoid concurrent
          # writes to the same .nix files.
          for attr in \
            packages.x86_64-linux.scip \
            checks.x86_64-linux.go-bindings \
            checks.x86_64-linux.reprolang \
            checks.x86_64-linux.typescript-bindings; do
            nix run github:Mic92/nix-update -- \
              --flake --version=skip "$attr"
          done
```

**Why no new job** — the `list` job enumerates flake checks dynamically (`ci.yaml:132-136`):

```yaml
      - id: checks
        run: |
          CHECKS=$(nix eval .#checks.x86_64-linux \
            --apply builtins.attrNames --json)
          echo "result=$CHECKS" >> "$GITHUB_OUTPUT"
```

and the `checks` matrix (`ci.yaml:144-157`) builds each — the new `checks.nix` attribute is auto-discovered. All three edits must land in the SAME commit as `swift/go.mod` (RESEARCH Pitfall 13). Do not add `swift/**` to release triggers (release watches `cmd/scip/version.txt` only).

### 13. `checks.nix` ← `go-bindings` entry, `checks.nix:47-61`

**Excerpt — verbatim:**

```nix
  go-bindings = pkgs.buildGoModule {
    pname = "scip-bindings-go";
    inherit version;
    src = ./.;
    modRoot = "./bindings/go/scip";
    vendorHash = "sha256-9uPDs/MoITP8Q92HQEMoRUB7Dz30n242z4cSnxjiPCk=";
    env.GOWORK = "off";
    buildTags = [ "asserts" ];
    subPackages = [
      "."
      "memtest"
      "testutil"
    ];
    installPhase = "touch $out";
  };
```

New `swift` attribute mirrors this with: `pname = "scip-swift"`, `modRoot = "./swift"`, `vendorHash` placeholder on first build (paste what `nix build` prints), `env.GOWORK = "off"`, `subPackages` listing every package dir that has tests (`internal/symbol` etc. as added — buildGoModule's check phase runs `go test ./...` within modRoot for listed subpackages), `installPhase = "touch $out"`. Contrast with the reprolang entry (`checks.nix:79-100`) which adds a package.json version `assert` and `buildInputs = [ pkgs.tree-sitter ]` — **neither applies in Phase 1** (no published manifest; no grammar). Consider whether `buildTags = [ "asserts" ]` should be kept for parity — the `swift/` module has no `assert()` calls, so it is optional; keeping it matches house style. `nixfmt` formats this file (`checks.nix` is covered by `formatting`, line 42).

### 14. Spike artifacts (`swift/spikes/*`)

- **`lsp_probe/main.go`** — package `main`, same minimal skeleton as item 3, plus `github.com/sourcegraph/jsonrpc2 v0.2.2` (add to `swift/go.mod` at that point). No in-repo analog for the RPC loop; the request sequence, readiness gate, and outputs are fully prescribed in RESEARCH's Spike Protocol — follow it, not an in-repo pattern. Keep it throwaway: no `internal/` promotion, no CLI wiring.
- **`fixture/` + `perf/`** — committed input data like `reprolang/testdata/` (plain source files checked in, generated parser output diffed). `.build/` dirs must be git-ignored (mirrors how `reprolang` ignores nothing similar because it has no build dir — add a `.gitignore` inside `swift/spikes/perf/`).
- **`2026-08-sourcekit-lsp-findings.md`** — evidence document; follow the structure of `.planning/research/01-...md` docs: title + metadata header, measured-numbers tables, explicit go/no-go verdict, "how to re-run" section. Prettier applies (`.md` is in the formatting check's glob).

---

## Shared Patterns

### Test conventions (all from `bindings/go/scip/`)

- **Naming:** `TestXxx` table tests + `t.Run(test.Symbol, ...)` subtests keyed on the input string (`symbol_test.go:125-126`, `symbol_formatter_test.go:40-41`); property tests named for the law they check (`TestComparePositionTransitive`, `TestLessCompareConsistent` — `position_test.go:89,116`); generators named `genX()` returning `*rapid.Generator[T]` (`position_test.go:64,73`; `sort_test.go:129`).
- **Table shape:** anonymous `type test struct` inside the test function, slice literal of cases (`symbol_test.go:11-15`, `symbol_formatter_test.go:32-34`).
- **Diff idiom (copy verbatim):** `if diff := cmp.Diff(want, got); diff != "" { t.Fatalf("unexpected response (-want +got):\n%s", diff) }` — `symbol_test.go:129-131`, `symbol_formatter_test.go:45-47`.
- **No-panic + error assertions:** `require.NotPanics(t, func() { ... }, "panic when parsing %q", symbolName)` — `symbol_test.go:149-153`; `require.NoError(t, err, "... %q ...", x)` message style per RESEARCH.
- **rapid skeleton:** `rapid.Custom(func(t *rapid.T) T { ... rapid.X.Draw(t, "Field") ... })` + `rapid.Check(t, func(t *rapid.T) {...})`; compose generators with `rapid.SliceOfN`, `rapid.SampledFrom`, `rapid.IntRange`; failure messages with `%+v` field dumps (`position_test.go:64-101`).
- **Imports in tests:** `testing` / `cmp` / `require` / `rapid` only (`symbol_test.go:3-8`, `position_test.go:3-8`) — no gomega/ginkgo, no testify suites.
- **memtest boundary:** never add files under `bindings/go/scip/memtest/`; it must not run in parallel.

### Error handling

- Wrap with context: `fmt.Errorf("...: %w", err)` (namer: `"namer produced unparseable symbol %q: %w"`); aggregate with `errors.Join` + a `var errs error` accumulator (repro `namer.go:13-21,65-84`).
- Typed struct errors implementing `error` for user-facing diagnostics (`emptySymbolError` — `symbol.go:48-52`; `unrecognizedDescriptorError` — `symbol_parser.go:459-467`).
- `, ok` second returns for "no result" (AGENTS.md); `, error` last value always.
- Panic only for programmer errors (`symbol_parser.go:370`).

### godoc / comments

- Exported identifiers get comments starting with the name; `CAUTION:` lines for sharp edges (`symbol.go:9-13`); multi-paragraph godoc separated by blank comment lines; pre-condition comments on non-exported parsing helpers (`symbol_parser.go:267-280,298-299`).

### go.work / module conventions

- Workspace = `go 1.25.0` + `use (...)` listing module dirs; `replace` directives live in each leaf `go.mod` and must be **relative** (`../bindings/go/scip`) because Nix/CI build per-module with `GOWORK=off` (repro `go.mod:5`; `checks.nix:53`; `ci.yaml:73-74`).
- New module sanity: repo root `go build ./... && go test ./...` (workspace mode), inside `swift/` `GOWORK=off go test ./...`, then `nix flake check`.
- Commit `go.work` + `go.work.sum` together; commit `swift/go.mod` + `swift/go.sum` + ci.yaml edits + checks.nix `swift` entry in the same change.

### Nix / CI conventions

- Every Go module gets one `buildGoModule` check attribute: `pname`, `inherit version`, `src = ./.`, `modRoot`, `vendorHash`, `env.GOWORK = "off"`, `subPackages`, `installPhase = "touch $out"` (`checks.nix:47-61`). No bespoke CI job — flake checks are enumerated by the `list` job (`ci.yaml:132-136`) and matrix-built (`ci.yaml:144-157`).
- Formatting gate covers new files: gofmt/goimports on all Go, prettier on `swift/README.md` + findings `.md`, nixfmt on `checks.nix` (`checks.nix:37-43`). Run `nix develop` formatters before committing.
- vendorHash churn on first build is expected: first `nix build` prints the hash; paste and re-run; the `fix` job's nix-update loop keeps it fresh afterwards (`ci.yaml:82-97`).

### Code-format constraints for executors

- Go: tabs (gofmt), `snake_case.go` one concern per file, tests co-located `<file>_test.go`.
- Markdown/YAML: prettier — no semicolons, single quotes, es5 trailing commas (`.prettierrc:1-5`).
- Nix: nixfmt.
