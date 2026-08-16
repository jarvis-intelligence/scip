package symbol

import (
	"testing"

	"github.com/scip-code/scip/bindings/go/scip"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestSymbolMethodOnStructRoundTrips is the walking-skeleton slice for the
// scip-swift module: one real Swift symbol string (a method on a struct in a
// SwiftPM module) must be produced by the namer and must round-trip through
// scip.ParseSymbol + scip.VerboseSymbolFormatter as the identity.
func TestSymbolMethodOnStructRoundTrips(t *testing.T) {
	input := SymbolInput{
		Module:        "MyApp",
		ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}},
		Name:          "area",
		Kind:          DeclKindMethod,
		OverloadIndex: 0,
	}

	got, err := Symbol(input)
	require.NoError(t, err)

	want := "scip-swift swiftpm MyApp . Shape#area()."
	require.Equal(t, want, got)

	parsed, err := scip.ParseSymbol(want)
	require.NoError(t, err)
	require.Equal(t, scip.Scheme(Scheme), parsed.Scheme)

	formatted := scip.VerboseSymbolFormatter.FormatSymbol(parsed)
	require.Equal(t, want, formatted, "parse->format must be the identity")
}

// TestSymbolRoundTripProperty checks the namer invariant over sampled inputs:
// every produced symbol parses and re-formats to the identical string.
func TestSymbolRoundTripProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		in := SymbolInput{
			Module: rapid.SampledFrom([]string{"MyApp", "App", "Core"}).Draw(t, "module"),
			ContainerPath: []Container{{
				Name: rapid.SampledFrom([]string{"Shape", "Vec", "Color"}).Draw(t, "container"),
				Kind: DeclKindStruct,
			}},
			Name:          rapid.SampledFrom([]string{"area", "draw", "parse"}).Draw(t, "name"),
			Kind:          DeclKindMethod,
			OverloadIndex: rapid.IntRange(0, 3).Draw(t, "overloadIndex"),
		}
		s, err := Symbol(in)
		require.NoError(t, err)

		parsed, err := scip.ParseSymbol(s)
		require.NoError(t, err, "symbol %q must parse", s)

		require.Equal(t, s, scip.VerboseSymbolFormatter.FormatSymbol(parsed))
	})
}
