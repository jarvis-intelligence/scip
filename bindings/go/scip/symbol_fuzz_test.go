package scip

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// FuzzParseSymbol asserts two properties over arbitrary symbol strings:
//
//  1. ParseSymbol never panics — malformed input must surface as a returned
//     error, never as an out-of-bounds read (the trailing-multi-byte-rune
//     panic this target was created for fails the run via the panic itself).
//  2. Every successfully parsed symbol round-trips: re-formatting the parsed
//     value with VerboseSymbolFormatter reproduces the input identically.
//
// The seed corpus runs under plain `go test` (and thus under the Nix
// go-bindings check with -tags asserts); `-fuzz` soaks are a local-only
// exercise and are deliberately not wired into CI.
func FuzzParseSymbol(f *testing.F) {
	for _, seed := range []string{
		// Seeds from TestParseSymbol.
		"local a",
		"a b c d method().",
		"a b c d `e f`.",
		"a b  c d e f.",
		"lsif-java maven package 1.0.0 java/io/File#Entry.method(+1).(param)[TypeParam]",
		"rust-analyzer cargo std 1.0.0 macros/println!",
		"cxx . todo-pkg todo-version gfx/Rect#x(455f465bc33b4cdf).",
		"cxx . . $ `<external>/4398592474888995393/assert.h:92:11`!",
		"cxx . . $ llvm/itanium_demangle/OutputBuffer#empty(50ce9a9e25b4a850).",
		"a b c d `F⃗`.",
		// Seeds from TestParseSymbolError.
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
		// Trailing-multi-byte-rune panic repros (fixed by the peekNext guard).
		"a b c d fooΩ",
		"test . pkg . barΩ",
		"a b c d `foo`Ω",
		// Valid escaped-Unicode forms: these must parse and round-trip.
		"scip swift MyMod . `🚀`.",
		"a b c d `π`.",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		parsed, err := ParseSymbol(input)
		if err != nil {
			return
		}
		if formatted := VerboseSymbolFormatter.FormatSymbol(parsed); formatted != input {
			t.Errorf("parse->format is not the identity: input %q, formatted %q", input, formatted)
		}
	})
}

// genRuneSoup generates short strings from ASCII identifier/grammar bytes
// with a strong bias toward a multi-byte rune at the tail of the string —
// the shape that triggered the peekNext out-of-bounds read.
func genRuneSoup() *rapid.Generator[string] {
	multiByte := rapid.SampledFrom([]string{"Ω", "π", "λ", "🚀", "🧠", "⃗"})
	asciiChunks := rapid.SampledFrom([]string{
		"a", "b", "foo", "init", ".", "#", "(", ")", "[", "]", "`", "/", "!", ":", " ", "+", "$", "@", "&", "-",
	})
	return rapid.Custom(func(t *rapid.T) string {
		n := rapid.IntRange(0, 6).Draw(t, "n")
		var b strings.Builder
		for i := 0; i < n; i++ {
			if i == n-1 && rapid.Bool().Draw(t, "tailMultiByte") {
				b.WriteString(multiByte.Draw(t, "tailRune"))
			} else {
				b.WriteString(asciiChunks.Draw(t, "chunk"))
			}
		}
		return b.String()
	})
}

// TestParseSymbolRuneSoupNeverPanics feeds rune-soup strings (biased to
// multi-byte tails) into ParseSymbol and asserts only the never-panic
// invariant: an error or a successful parse are both acceptable outcomes.
func TestParseSymbolRuneSoupNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		input := genRuneSoup().Draw(t, "input")
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseSymbol panicked on %q: %v", input, r)
			}
		}()
		_, _ = ParseSymbol(input)
	})
}
