package symbol

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scip-code/scip/bindings/go/scip"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestSymbolGoldenTable is the executable spec for the frozen scip-swift
// symbol scheme: one case per row of the RESEARCH Symbol Scheme Mapping
// Table (rows 1-29 where a string is defined), plus the 01-01 tracer case.
// The module is "MyMod" wherever a SwiftPM module name is needed.
func TestSymbolGoldenTable(t *testing.T) {
	type test struct {
		Input    SymbolInput
		Expected string
	}
	tests := []test{
		// Row 1: SwiftPM module's own symbol (the target of `import MyMod`).
		{Input: SymbolInput{Module: "MyMod", Name: "MyMod", Kind: DeclKindModule},
			Expected: "scip-swift swiftpm MyMod . MyMod/"},
		// Row 2: system module (manager "swift", toolchain version).
		{Input: SymbolInput{Module: "Swift", IsSystemModule: true, SwiftToolchainVersion: "6.2.4", Name: "Swift", Kind: DeclKindModule},
			Expected: "scip-swift swift Swift 6.2.4 Swift/"},
		// Rows 3-6: top-level struct, enum, protocol, typealias.
		{Input: SymbolInput{Module: "MyMod", Name: "Shape", Kind: DeclKindStruct},
			Expected: "scip-swift swiftpm MyMod . Shape#"},
		{Input: SymbolInput{Module: "MyMod", Name: "Color", Kind: DeclKindEnum},
			Expected: "scip-swift swiftpm MyMod . Color#"},
		{Input: SymbolInput{Module: "MyMod", Name: "Drawable", Kind: DeclKindProtocol},
			Expected: "scip-swift swiftpm MyMod . Drawable#"},
		{Input: SymbolInput{Module: "MyMod", Name: "FooAlias", Kind: DeclKindTypeAlias},
			Expected: "scip-swift swiftpm MyMod . FooAlias#"},
		// Row 7: nested type — ContainerPath carries the outer ancestry,
		// Name is the nested type itself.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Outer", Kind: DeclKindStruct}}, Name: "Inner", Kind: DeclKindStruct},
			Expected: "scip-swift swiftpm MyMod . Outer#Inner#"},
		// Row 8: top-level func.
		{Input: SymbolInput{Module: "MyMod", Name: "parse", Kind: DeclKindFunc},
			Expected: "scip-swift swiftpm MyMod . parse()."},
		// Row 9: method.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}}, Name: "area", Kind: DeclKindMethod},
			Expected: "scip-swift swiftpm MyMod . Shape#area()."},
		// Row 10: overloads — index 0 renders no disambiguator, N>0 renders (+N).
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}}, Name: "resize", Kind: DeclKindMethod, OverloadIndex: 0},
			Expected: "scip-swift swiftpm MyMod . Shape#resize()."},
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}}, Name: "resize", Kind: DeclKindMethod, OverloadIndex: 1},
			Expected: "scip-swift swiftpm MyMod . Shape#resize(+1)."},
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}}, Name: "resize", Kind: DeclKindMethod, OverloadIndex: 2},
			Expected: "scip-swift swiftpm MyMod . Shape#resize(+2)."},
		// Row 11: operators — "+" IS an identifier character (no escaping),
		// "==" is not (the formatter backtick-escapes it).
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Vec", Kind: DeclKindStruct}}, Name: "+", Kind: DeclKindOperator},
			Expected: "scip-swift swiftpm MyMod . Vec#+()."},
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Vec", Kind: DeclKindStruct}}, Name: "==", Kind: DeclKindOperator},
			Expected: "scip-swift swiftpm MyMod . Vec#`==`()."},
		// Row 12: init — the Swift-native name SourceKit reports; overload
		// disambiguation as row 10.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Vec", Kind: DeclKindStruct}}, Name: "init", Kind: DeclKindConstructor},
			Expected: "scip-swift swiftpm MyMod . Vec#init()."},
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Vec", Kind: DeclKindStruct}}, Name: "init", Kind: DeclKindConstructor, OverloadIndex: 1},
			Expected: "scip-swift swiftpm MyMod . Vec#init(+1)."},
		// Row 13: deinit.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Vec", Kind: DeclKindStruct}}, Name: "deinit", Kind: DeclKindDestructor},
			Expected: "scip-swift swiftpm MyMod . Vec#deinit()."},
		// Row 14: getter — reuses the property's Method-shaped descriptor;
		// distinguished from a zero-arg method `area()` by Kind only, and a
		// same-named method forms one ordering group with it (row 10).
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}}, Name: "area", Kind: DeclKindGetter},
			Expected: "scip-swift swiftpm MyMod . Shape#area()."},
		// Row 15: setter — the source name is "area="; "=" is outside the
		// simple-identifier set so the formatter backtick-escapes it.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}}, Name: "area=", Kind: DeclKindSetter},
			Expected: "scip-swift swiftpm MyMod . Shape#`area=`()."},
		// Rows 16-18: terms — property, let, global var.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}}, Name: "origin", Kind: DeclKindProperty},
			Expected: "scip-swift swiftpm MyMod . Shape#origin."},
		{Input: SymbolInput{Module: "MyMod", Name: "origin", Kind: DeclKindConstant},
			Expected: "scip-swift swiftpm MyMod . origin."},
		{Input: SymbolInput{Module: "MyMod", Name: "config", Kind: DeclKindVariable},
			Expected: "scip-swift swiftpm MyMod . config."},
		// Row 19: subscript with overload indices.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Vec", Kind: DeclKindStruct}}, Name: "subscript", Kind: DeclKindSubscript},
			Expected: "scip-swift swiftpm MyMod . Vec#subscript()."},
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Vec", Kind: DeclKindStruct}}, Name: "subscript", Kind: DeclKindSubscript, OverloadIndex: 1},
			Expected: "scip-swift swiftpm MyMod . Vec#subscript(+1)."},
		// Row 20: enum case — a value term.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Color", Kind: DeclKindEnum}}, Name: "red", Kind: DeclKindEnumCase},
			Expected: "scip-swift swiftpm MyMod . Color#red."},
		// Row 21: extension member, same module, any file (SYM-02) — the
		// namer API has no extension flag; ContainerPath IS the extended
		// type's ancestry, so the input shape is identical to row 9.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}}, Name: "area2", Kind: DeclKindMethod},
			Expected: "scip-swift swiftpm MyMod . Shape#area2()."},
		// Row 22: retroactive extension (SYM-02) — the member declared in
		// module Utils on Shape owned by module App: the caller passes the
		// OWNER module ("App") so the member lives under the extended type's
		// full identity; a second same-named retroactive method from another
		// module disambiguates as (+1).
		{Input: SymbolInput{Module: "App", ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}}, Name: "spike", Kind: DeclKindMethod},
			Expected: "scip-swift swiftpm App . Shape#spike()."},
		{Input: SymbolInput{Module: "App", ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}}, Name: "spike", Kind: DeclKindMethod, OverloadIndex: 1},
			Expected: "scip-swift swiftpm App . Shape#spike(+1)."},
		// Row 23: protocol requirement method.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Drawable", Kind: DeclKindProtocol}}, Name: "draw", Kind: DeclKindProtocolMethod},
			Expected: "scip-swift swiftpm MyMod . Drawable#draw()."},
		// Rows 24-25: conformance witness and class override use their OWN
		// paths — relationship edges are Phase 4; the paths are frozen now.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Circle", Kind: DeclKindStruct}}, Name: "draw", Kind: DeclKindMethod},
			Expected: "scip-swift swiftpm MyMod . Circle#draw()."},
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Dog", Kind: DeclKindClass}}, Name: "speak", Kind: DeclKindMethod},
			Expected: "scip-swift swiftpm MyMod . Dog#speak()."},
		// Row 26: generic type parameter — descriptors chain without separator.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "Box", Kind: DeclKindStruct}}, Name: "T", Kind: DeclKindTypeParameter},
			Expected: "scip-swift swiftpm MyMod . Box#[T]"},
		// Row 27: function parameter — the container is the Func.
		{Input: SymbolInput{Module: "MyMod", ContainerPath: []Container{{Name: "f", Kind: DeclKindFunc}}, Name: "x", Kind: DeclKindParameter},
			Expected: "scip-swift swiftpm MyMod . f().(x)"},
		// Row 29: macro.
		{Input: SymbolInput{Module: "MyMod", Name: "Preview", Kind: DeclKindMacro},
			Expected: "scip-swift swiftpm MyMod . Preview!"},
		// The 01-01 walking-skeleton tracer case, kept as a table row.
		{Input: SymbolInput{Module: "MyApp", ContainerPath: []Container{{Name: "Shape", Kind: DeclKindStruct}}, Name: "area", Kind: DeclKindMethod, OverloadIndex: 0},
			Expected: "scip-swift swiftpm MyApp . Shape#area()."},
		// System module with an empty toolchain version: the version renders
		// as "." (deterministic placeholder; the formatter maps "" to "." and
		// the parser maps it back).
		{Input: SymbolInput{Module: "Swift", IsSystemModule: true, Name: "Swift", Kind: DeclKindModule},
			Expected: "scip-swift swift Swift . Swift/"},
	}
	for _, test := range tests {
		t.Run(test.Expected, func(t *testing.T) {
			got, err := Symbol(test.Input)
			require.NoError(t, err)
			if diff := cmp.Diff(test.Expected, got); diff != "" {
				t.Fatalf("unexpected response (-want +got):\n%s", diff)
			}
		})
	}
}

// TestSymbolInputErrors covers the empty-input edge: an empty Module or an
// empty Name returns an error and emits no symbol string. An empty
// ContainerPath is valid (top-level symbol).
func TestSymbolInputErrors(t *testing.T) {
	for name, input := range map[string]SymbolInput{
		"empty module": {Module: "", Name: "area", Kind: DeclKindMethod},
		"empty name":   {Module: "MyMod", Name: "", Kind: DeclKindStruct},
	} {
		input := input
		t.Run(name, func(t *testing.T) {
			got, err := Symbol(input)
			require.Error(t, err)
			require.Empty(t, got)
		})
	}
}

// TestLocalSymbolGolden covers the local-id sanitization rule (RESEARCH
// row 28): non-identifier characters collapse to "_", ordinal N>0 appends
// "_N", and Unicode source names never appear raw in a local id.
func TestLocalSymbolGolden(t *testing.T) {
	for _, test := range []struct {
		SourceName string
		Ordinal    int
		Expected   string
	}{
		{"i", 0, "local i"},
		{"count", 2, "local count_2"},
		{"🚀", 0, "local _"},
		{"x", 1, "local x_1"},
	} {
		t.Run(test.Expected, func(t *testing.T) {
			require.Equal(t, test.Expected, LocalSymbol(test.SourceName, test.Ordinal))
		})
	}
}

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
	require.Equal(t, Scheme, parsed.Scheme)

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
