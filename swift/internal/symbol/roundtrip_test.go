package symbol

import (
	"sync"
	"testing"

	"github.com/scip-code/scip/bindings/go/scip"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// genNames biases toward names that stress the symbol grammar: operators
// ("==", "<->", "+"), emoji and multi-byte tails ("🚀", "π", "F⃗"), embedded
// backticks, and plain simple identifiers.
var genNames = []string{
	"foo", "init", "`==`", "🚀", "π", "F⃗", "back``tick", "$0", "a-b", "_x", "<->", "==",
}

// genLocalNames biases toward source names that stress local-id
// sanitization: Unicode (emoji, CJK, multi-byte tails), embedded backticks
// and spaces, and identifier-shaped names.
var genLocalNames = []string{
	"i", "count", "x", "🚀", "π", "F⃗", "日本語", "a b", "back``tick", "$0", "n🄼ame", "==", "<->",
}

// genDeclKind samples across the full DeclKind set.
func genDeclKind() *rapid.Generator[DeclKind] {
	return rapid.SampledFrom([]DeclKind{
		DeclKindModule, DeclKindStruct, DeclKindClass, DeclKindEnum,
		DeclKindProtocol, DeclKindTypeAlias, DeclKindFunc, DeclKindMethod,
		DeclKindOperator, DeclKindConstructor, DeclKindDestructor,
		DeclKindGetter, DeclKindSetter, DeclKindProperty, DeclKindConstant,
		DeclKindVariable, DeclKindSubscript, DeclKindEnumCase,
		DeclKindProtocolMethod, DeclKindTypeParameter, DeclKindParameter,
		DeclKindMacro,
	})
}

// genName draws identifier-shaped and grammar-hostile names alike.
func genName() *rapid.Generator[string] {
	return rapid.SampledFrom(genNames)
}

// genContainer draws one ancestry node with a grammar-hostile name bias.
func genContainer() *rapid.Generator[Container] {
	return rapid.Custom(func(t *rapid.T) Container {
		return Container{
			Name: genName().Draw(t, "containerName"),
			Kind: genDeclKind().Draw(t, "containerKind"),
		}
	})
}

// genInput draws over the full SymbolInput space: module names including a
// multi-byte one, containers 0-3 deep, every DeclKind, overloads 0-5, and
// both SwiftPM and system-module package headers.
func genInput() *rapid.Generator[SymbolInput] {
	return rapid.Custom(func(t *rapid.T) SymbolInput {
		return SymbolInput{
			Module:                rapid.SampledFrom([]string{"MyMod", "App", "🄼odule"}).Draw(t, "module"),
			IsSystemModule:        rapid.Bool().Draw(t, "isSystemModule"),
			SwiftToolchainVersion: rapid.SampledFrom([]string{"6.2.4", "6.1.0"}).Draw(t, "swiftToolchainVersion"),
			ContainerPath:         rapid.SliceOfN(genContainer(), 0, 3).Draw(t, "containerPath"),
			Name:                  genName().Draw(t, "name"),
			Kind:                  genDeclKind().Draw(t, "kind"),
			OverloadIndex:         rapid.IntRange(0, 5).Draw(t, "overloadIndex"),
		}
	})
}

// This property checks the encoding edge over generated inputs: every symbol
// the namer produces parses and re-formats as the identity, with the parsed
// Scheme == "scip-swift". Identifiers are emitted UTF-8 byte-faithful with
// automatic backtick escaping by the bindings formatter — no Unicode
// normalization is applied.
func TestNamerRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		in := genInput().Draw(t, "input")
		s, err := Symbol(in)
		require.NoError(t, err, "input %+v", in)

		parsed, err := scip.ParseSymbol(s)
		require.NoError(t, err, "symbol %q must parse", s)

		formatted := scip.VerboseSymbolFormatter.FormatSymbol(parsed)
		require.Equal(t, s, formatted, "parse->format must be the identity")

		require.Equal(t, Scheme, parsed.Scheme)
	})
}

// TestLocalSymbolRoundTrip checks that sanitized local ids round-trip
// through the local form: IsLocalSymbol holds, ParseSymbol succeeds, and
// FormatSymbol is the identity.
func TestLocalSymbolRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sourceName := rapid.SampledFrom(genLocalNames).Draw(t, "sourceName")
		ordinal := rapid.IntRange(0, 5).Draw(t, "ordinal")

		s := LocalSymbol(sourceName, ordinal)
		require.True(t, scip.IsLocalSymbol(s), "local symbol %q must be local", s)

		parsed, err := scip.ParseSymbol(s)
		require.NoError(t, err, "local symbol %q must parse", s)

		require.Equal(t, s, scip.VerboseSymbolFormatter.FormatSymbol(parsed))
	})
}

// TestNamerPure checks the determinism edge: Symbol is a pure function of
// its input — identical input yields the identical string on every call and
// under parallel calls.
func TestNamerPure(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		in := genInput().Draw(t, "input")
		first, err := Symbol(in)
		require.NoError(t, err)

		second, err := Symbol(in)
		require.NoError(t, err)
		require.Equal(t, first, second, "identical input must yield the identical string")

		const workers = 8
		results := make([]string, workers)
		errs := make([]error, workers)
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				results[i], errs[i] = Symbol(in)
			}(i)
		}
		wg.Wait()
		for i := range results {
			require.NoError(t, errs[i], "worker %d", i)
			require.Equal(t, first, results[i], "worker %d diverged", i)
		}
	})
}
