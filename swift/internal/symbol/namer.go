package symbol

import (
	"errors"
	"fmt"
	"strings"

	"github.com/scip-code/scip/bindings/go/scip"
)

// ErrEmptyModule is returned by Symbol when SymbolInput.Module is empty: a
// global symbol cannot be named without its owning module.
var ErrEmptyModule = errors.New("symbol input has empty Module")

// ErrEmptyName is returned by Symbol when SymbolInput.Name is empty.
var ErrEmptyName = errors.New("symbol input has empty Name")

// localScheme is the reserved scheme prefix for document-scoped symbols.
const localScheme = "local"

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
	// Module is the owning Swift module (target) name. For a retroactive
	// extension member this is the module OWNING the extended type (SYM-02).
	Module string
	// IsSystemModule is true for stdlib/SDK symbols (manager "swift").
	IsSystemModule bool
	// SwiftToolchainVersion is the toolchain version reported for system
	// modules; unused for SwiftPM targets.
	SwiftToolchainVersion string
	// ContainerPath is the extended-type-aware ancestry, outermost first.
	// An empty ContainerPath means a top-level symbol.
	ContainerPath []Container
	// Name is the source name; may be an operator or Unicode.
	Name string
	// Kind is the declaration category of Name.
	Kind DeclKind
	// OverloadIndex is 0 for no disambiguator; N>0 renders "(+N)"
	// (scip-java style), derived from source declaration order.
	OverloadIndex int
}

// Symbol returns the canonical scip-swift SCIP symbol string for the input.
//
// The string is produced by building a *scip.Symbol (SwiftPM packages use
// ManagerSwiftPM with version "."; system modules use ManagerSystem with
// SwiftToolchainVersion) with one descriptor per ancestry node plus a
// suffixed descriptor for Name, then formatting via
// scip.VerboseSymbolFormatter. Escaping is the formatter's job — the namer
// adds no escaping code of its own. The result is validated with
// scip.ParseSymbol before it is returned: a namer bug surfaces as an error,
// never as a malformed string reaching the index.
func Symbol(in SymbolInput) (string, error) {
	if in.Module == "" {
		return "", ErrEmptyModule
	}
	if in.Name == "" {
		return "", ErrEmptyName
	}

	pkg := &scip.Package{Manager: ManagerSwiftPM, Name: in.Module, Version: "."}
	if in.IsSystemModule {
		pkg.Manager = ManagerSystem
		// An empty version renders as the "." placeholder (the formatter
		// maps "" to "." and the parser maps it back).
		pkg.Version = in.SwiftToolchainVersion
	}

	sym := &scip.Symbol{Scheme: Scheme, Package: pkg}
	descriptors, err := appendDescriptors(in)
	if err != nil {
		return "", err
	}
	sym.Descriptors = descriptors

	s := scip.VerboseSymbolFormatter.FormatSymbol(sym)
	if _, err := scip.ParseSymbol(s); err != nil {
		return "", fmt.Errorf("namer produced unparseable symbol %q: %w", s, err)
	}
	return s, nil
}

// appendDescriptors builds the descriptor chain for the input: one
// descriptor per ContainerPath node (outermost first) followed by the
// descriptor for Name. It is the single place the frozen scheme chooses
// descriptor suffixes.
func appendDescriptors(in SymbolInput) ([]*scip.Descriptor, error) {
	descriptors := make([]*scip.Descriptor, 0, len(in.ContainerPath)+1)
	for _, container := range in.ContainerPath {
		descriptor, err := newDescriptor(container.Name, container.Kind, 0)
		if err != nil {
			return nil, err
		}
		descriptors = append(descriptors, descriptor)
	}
	descriptor, err := newDescriptor(in.Name, in.Kind, in.OverloadIndex)
	if err != nil {
		return nil, err
	}
	return append(descriptors, descriptor), nil
}

// newDescriptor maps (name, kind) onto one descriptor with the suffix the
// frozen scheme prescribes for the kind. Overload indices apply only to the
// Method family: the formatter renders disambiguators solely for
// Descriptor_Method, so an OverloadIndex greater than 0 on any other family
// is ignored rather than corrupted into the string.
func newDescriptor(name string, kind DeclKind, overloadIndex int) (*scip.Descriptor, error) {
	descriptor := &scip.Descriptor{Name: name}
	switch kind {
	case DeclKindModule:
		descriptor.Suffix = scip.Descriptor_Namespace
	case DeclKindStruct, DeclKindClass, DeclKindEnum, DeclKindProtocol, DeclKindTypeAlias:
		descriptor.Suffix = scip.Descriptor_Type
	case DeclKindFunc, DeclKindMethod, DeclKindOperator, DeclKindConstructor,
		DeclKindDestructor, DeclKindGetter, DeclKindSetter, DeclKindSubscript, DeclKindProtocolMethod:
		descriptor.Suffix = scip.Descriptor_Method
		if overloadIndex > 0 {
			descriptor.Disambiguator = fmt.Sprintf("+%d", overloadIndex)
		}
	case DeclKindProperty, DeclKindConstant, DeclKindVariable, DeclKindEnumCase:
		descriptor.Suffix = scip.Descriptor_Term
	case DeclKindTypeParameter:
		descriptor.Suffix = scip.Descriptor_TypeParameter
	case DeclKindParameter:
		descriptor.Suffix = scip.Descriptor_Parameter
	case DeclKindMacro:
		descriptor.Suffix = scip.Descriptor_Macro
	default:
		return nil, fmt.Errorf("namer cannot map unsupported DeclKind %d", kind)
	}
	return descriptor, nil
}

// LocalSymbol returns a document-scoped local symbol string "local <id>"
// where id is the source name sanitized to a simple identifier — every rune
// outside the simple-identifier set collapses to "_" — with the ordinal
// appended as "_N" when it is greater than zero. Unicode source names
// (emoji, CJK) never appear raw in a local id; the display name keeps the
// source spelling. The returned string is the formatter's rendering of the
// local-form *scip.Symbol, never assembled by hand.
func LocalSymbol(sourceName string, ordinal int) string {
	id := sanitizeLocalID(sourceName)
	if ordinal > 0 {
		id = fmt.Sprintf("%s_%d", id, ordinal)
	}
	sym := &scip.Symbol{
		Scheme:      localScheme,
		Descriptors: []*scip.Descriptor{{Name: id, Suffix: scip.Descriptor_Local}},
	}
	return scip.VerboseSymbolFormatter.FormatSymbol(sym)
}

// sanitizeLocalID maps a source name onto the <simple-identifier> charset
// the local form requires, collapsing every other rune to "_". A name with
// no identifier characters at all sanitizes to "_".
func sanitizeLocalID(sourceName string) string {
	var b strings.Builder
	for _, r := range sourceName {
		if isSimpleIdentifierCharacter(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
}

// isSimpleIdentifierCharacter mirrors the <identifier-character> set of the
// SCIP symbol grammar (scip.proto): '_', '+', '-', '$', and ASCII letters
// and digits.
func isSimpleIdentifierCharacter(r rune) bool {
	return r == '_' || r == '+' || r == '-' || r == '$' ||
		('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9')
}
