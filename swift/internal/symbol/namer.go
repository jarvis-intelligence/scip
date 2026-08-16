// Package symbol defines the scip-swift SCIP symbol scheme and the shared
// namer both indexing paths (SourceKit-LSP semantic and tree-sitter fallback)
// use to produce symbol strings.
//
// The namer is a pure function library: it constructs protobuf *scip.Symbol
// values and formats them with scip.VerboseSymbolFormatter. Descriptor
// suffixes and backtick escaping come only from the bindings formatter —
// symbols are never assembled with fmt.Sprintf or string concatenation.
package symbol

import (
	"fmt"

	"github.com/scip-code/scip/bindings/go/scip"
)

// Scheme is the SCIP symbol scheme prefix for all scip-swift global symbols.
//
// CAUTION: The scheme must not start with "local" — that prefix is reserved
// by the SCIP symbol grammar for document-scoped symbols.
const Scheme = "scip-swift"

// ManagerSwiftPM is the package manager for SwiftPM target modules.
const ManagerSwiftPM = "swiftpm"

// DeclKind enumerates the Swift declaration categories the scheme
// distinguishes. This tracer slice carries only the two kinds the
// walking-skeleton test needs; plan 01-02 expands the set.
type DeclKind int

const (
	// DeclKindStruct is a struct (or class/actor/enum) declaration.
	DeclKindStruct DeclKind = iota
	// DeclKindMethod is a method or free function declaration.
	DeclKindMethod
)

// Container is one node of a symbol's extended-type-aware ancestry,
// outermost first.
type Container struct {
	Name string
	Kind DeclKind
}

// SymbolInput is everything needed to name a symbol. Both the semantic path
// (from symbolInfo/container chains) and the fallback path (from syntax)
// must be able to populate it.
type SymbolInput struct {
	// Module is the owning Swift module (target) name.
	Module string
	// IsSystemModule is true for stdlib/SDK symbols (manager "swift").
	IsSystemModule bool
	// SwiftToolchainVersion is the toolchain version reported for system
	// modules; unused for SwiftPM targets.
	SwiftToolchainVersion string
	// ContainerPath is the extended-type-aware ancestry, outermost first.
	ContainerPath []Container
	// Name is the source name; may be an operator or Unicode.
	Name string
	// Kind is the declaration category of Name.
	Kind DeclKind
	// OverloadIndex is 0 for no disambiguator; N>0 renders "(+N)"
	// (scip-java style).
	OverloadIndex int
}

// Symbol returns the canonical scip-swift SCIP symbol string for the input.
//
// The string is produced by building a *scip.Symbol (package manager
// ManagerSwiftPM, package name in.Module, version ".") with Type descriptors
// for the container path and a suffixed descriptor for Name, then formatting
// via scip.VerboseSymbolFormatter.FormatSymbol. The result is validated with
// scip.ParseSymbol before it is returned: a namer bug surfaces as an error,
// never as a malformed string reaching the index.
func Symbol(in SymbolInput) (string, error) {
	sym := &scip.Symbol{
		Scheme: Scheme,
		Package: &scip.Package{
			Manager: ManagerSwiftPM,
			Name:    in.Module,
			Version: ".",
		},
	}
	for _, container := range in.ContainerPath {
		sym.Descriptors = append(sym.Descriptors, &scip.Descriptor{
			Name:   container.Name,
			Suffix: scip.Descriptor_Type,
		})
	}
	switch in.Kind {
	case DeclKindStruct:
		sym.Descriptors = append(sym.Descriptors, &scip.Descriptor{
			Name:   in.Name,
			Suffix: scip.Descriptor_Type,
		})
	case DeclKindMethod:
		method := &scip.Descriptor{
			Name:   in.Name,
			Suffix: scip.Descriptor_Method,
		}
		if in.OverloadIndex > 0 {
			method.Disambiguator = fmt.Sprintf("+%d", in.OverloadIndex)
		}
		sym.Descriptors = append(sym.Descriptors, method)
	default:
		return "", fmt.Errorf("namer cannot map unsupported DeclKind %d", in.Kind)
	}

	s := scip.VerboseSymbolFormatter.FormatSymbol(sym)
	if _, err := scip.ParseSymbol(s); err != nil {
		return "", fmt.Errorf("namer produced unparseable symbol %q: %w", s, err)
	}
	return s, nil
}
