// Package symbol defines the scip-swift SCIP symbol scheme and the shared
// namer both indexing paths (SourceKit-LSP semantic and tree-sitter
// fallback) use to produce symbol strings.
//
// # Grammar
//
// Every scip-swift global symbol has the header form
//
//	scip-swift swiftpm <Module> . <descriptors>   (SwiftPM target modules)
//	scip-swift swift <Module> <swift-version> <descriptors>   (system/SDK modules)
//
// followed by descriptors chained without separators, one per ancestry node:
// namespace `M/`, type `Shape#`, method `area().` (optionally with the
// `(+N)` disambiguator), term `origin.`, type parameter `[T]`, parameter
// `(x)`, macro `Preview!`. Backtick escaping of identifiers outside the
// simple-identifier set (for example `==` or `🚀`) is performed by
// scip.VerboseSymbolFormatter — the namer never escapes by hand. Document-
// scoped locals use the reserved `local <simple-identifier>` form.
//
// # Frozen-spec rules
//
// These rules are frozen for v1; changing them invalidates stored indexes
// and golden tests downstream.
//
//   - Determinism: overload indices and local ordinals derive from source
//     declaration order only — within a (module, container, name) group,
//     declarations are sorted by source position; index 0 renders no
//     disambiguator and index N renders (+N). Both indexing paths must
//     derive these indices the same way.
//
//   - Canonical #if policy: the semantic path indexes the canonical build
//     configuration — macOS arm64, host Swift version, DEBUG — and the
//     fallback selects the same single canonical branch. The fallback never
//     indexes all branches of a conditional compilation block.
//
//   - Dual-path identity: both paths must emit the identical symbol string
//     for the same declaration. This is the navigation-trust contract —
//     goToDefinition/findReferences correctness depends on the two paths
//     never diverging, and the fallback never merges output with semantic
//     output.
//
// # Extension attribution (SYM-02)
//
// A member of `extension Foo` — same file, cross-file, or retroactive (the
// extended type owned by another module) — produces exactly the symbol it
// would have if declared inside the type body: the extended type's owner-
// module package and Foo# path. A retroactive method-family collision from
// a second declaring module disambiguates as `Foo#name(+1)`.
//
// # Known limitation — Term-family retroactive collisions
//
// The grammar/formatter supports disambiguators only on Method descriptors
// (bindings/go/scip/symbol_formatter.go renders them solely for
// Descriptor_Method), so retroactive Term-family (property/let/case)
// same-name collisions across declaring modules cannot carry (+N). Phase 1
// freezes: Method-family retroactive collisions disambiguate via (+N);
// Term-family collisions are a documented known limitation, to be resolved
// with Phase-3 USR evidence if real-world cases appear (recorded fallback:
// synthetic simple-identifier suffix).
//
// # Constructs that emit no symbol
//
// Property wrappers: the wrapper type itself is named like any type/member;
// the synthesized `_x`/`$x` storage accessors are not emitted in v1.
// `self` and `Self` never receive symbols (no navigation value).
package symbol

import (
	"github.com/scip-code/scip/bindings/go/scip"
)

// Scheme is the SCIP symbol scheme prefix for all scip-swift global symbols.
//
// CAUTION: The scheme must not start with "local" — that prefix is reserved
// by the SCIP symbol grammar for document-scoped symbols.
const Scheme = "scip-swift"

// ManagerSwiftPM is the package manager for SwiftPM target modules.
const ManagerSwiftPM = "swiftpm"

// ManagerSystem is the package manager for system/SDK modules (the Swift
// standard library and SDK overlays), mirroring scip-java's "jdk".
const ManagerSystem = "swift"

// DeclKind enumerates the Swift declaration categories the scheme
// distinguishes, one value per family in the symbol scheme mapping table.
type DeclKind int

const (
	// DeclKindModule is a Swift module (target) — the module's own symbol.
	DeclKindModule DeclKind = iota
	// DeclKindStruct is a struct declaration.
	DeclKindStruct
	// DeclKindClass is a class declaration (also used for actors; no actor
	// kind exists in scip.SymbolInformation.Kind).
	DeclKindClass
	// DeclKindEnum is an enum declaration.
	DeclKindEnum
	// DeclKindProtocol is a protocol declaration.
	DeclKindProtocol
	// DeclKindTypeAlias is a typealias declaration.
	DeclKindTypeAlias
	// DeclKindFunc is a top-level free function.
	DeclKindFunc
	// DeclKindMethod is a method declaration.
	DeclKindMethod
	// DeclKindOperator is an operator function (for example + or ==).
	DeclKindOperator
	// DeclKindConstructor is an init declaration (including init? and
	// init(from:) forms; overload indices separate them).
	DeclKindConstructor
	// DeclKindDestructor is a deinit declaration.
	DeclKindDestructor
	// DeclKindGetter is a property get accessor.
	DeclKindGetter
	// DeclKindSetter is a property set accessor (source name "name=").
	DeclKindSetter
	// DeclKindProperty is a stored or computed property.
	DeclKindProperty
	// DeclKindConstant is a let member or global.
	DeclKindConstant
	// DeclKindVariable is a global (or member) var without accessor
	// granularity.
	DeclKindVariable
	// DeclKindSubscript is a subscript declaration.
	DeclKindSubscript
	// DeclKindEnumCase is an enum case.
	DeclKindEnumCase
	// DeclKindProtocolMethod is a protocol requirement method.
	DeclKindProtocolMethod
	// DeclKindTypeParameter is a generic type parameter.
	DeclKindTypeParameter
	// DeclKindParameter is a function parameter.
	DeclKindParameter
	// DeclKindMacro is a freestanding or attached macro.
	DeclKindMacro
)

// ScipKind returns the frozen scip.SymbolInformation.Kind for the
// declaration kind. It is the single kind-mapping source shared by the
// Phase 2 fallback and Phase 3 semantic indexing paths.
//
// Mapping (RESEARCH mapping table): Module=29, Struct=49, Class=7 (also
// actor), Enum=11, Protocol=42, TypeAlias=55, Function=17, Method=26,
// Operator=34, Constructor=9 (init), Destructor=26 (deinit has no dedicated
// kind), Getter=18, Setter=45, Property=41, Constant=8, Variable=61,
// Subscript=47, EnumCase=12 (EnumMember), ProtocolMethod=68,
// TypeParameter=58, Parameter=37, Macro=25. Unknown kinds map to
// UnspecifiedKind.
func (k DeclKind) ScipKind() scip.SymbolInformation_Kind {
	switch k {
	case DeclKindModule:
		return scip.SymbolInformation_Module
	case DeclKindStruct:
		return scip.SymbolInformation_Struct
	case DeclKindClass:
		return scip.SymbolInformation_Class
	case DeclKindEnum:
		return scip.SymbolInformation_Enum
	case DeclKindProtocol:
		return scip.SymbolInformation_Protocol
	case DeclKindTypeAlias:
		return scip.SymbolInformation_TypeAlias
	case DeclKindFunc:
		return scip.SymbolInformation_Function
	case DeclKindMethod:
		return scip.SymbolInformation_Method
	case DeclKindOperator:
		return scip.SymbolInformation_Operator
	case DeclKindConstructor:
		return scip.SymbolInformation_Constructor
	case DeclKindDestructor:
		return scip.SymbolInformation_Method
	case DeclKindGetter:
		return scip.SymbolInformation_Getter
	case DeclKindSetter:
		return scip.SymbolInformation_Setter
	case DeclKindProperty:
		return scip.SymbolInformation_Property
	case DeclKindConstant:
		return scip.SymbolInformation_Constant
	case DeclKindVariable:
		return scip.SymbolInformation_Variable
	case DeclKindSubscript:
		return scip.SymbolInformation_Subscript
	case DeclKindEnumCase:
		return scip.SymbolInformation_EnumMember
	case DeclKindProtocolMethod:
		return scip.SymbolInformation_ProtocolMethod
	case DeclKindTypeParameter:
		return scip.SymbolInformation_TypeParameter
	case DeclKindParameter:
		return scip.SymbolInformation_Parameter
	case DeclKindMacro:
		return scip.SymbolInformation_Macro
	default:
		return scip.SymbolInformation_UnspecifiedKind
	}
}
