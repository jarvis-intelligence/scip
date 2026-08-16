package symbol

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
