package types

import "go/types"

// TypeExprKind enumerates the possible outer forms of a parsed Go type expression.
type TypeExprKind uint

const (
	// TypeExprKindNamed is a named Go type, possibly with type arguments.
	TypeExprKindNamed TypeExprKind = iota
	// TypeExprKindPointer is a pointer type expression.
	TypeExprKindPointer
	// TypeExprKindSlice is a slice type expression.
	TypeExprKindSlice
)

// Type is a loaded named Go type referenced by a Tego option.
type Type struct {
	Ref        string
	ImportPath string
	Name       string
	TypeName   *types.TypeName
	Type       types.Type
	Named      *types.Named
}

// TypeExpr is a parsed and loaded Go type expression used by custom go_type options.
type TypeExpr struct {
	Ref        string
	Kind       TypeExprKind
	ImportPath string
	Name       string
	TypeName   *types.TypeName
	Type       types.Type
	Named      *types.Named
	Args       []TypeExpr
	Elem       *TypeExpr
}

// Function is a loaded package-level Go function referenced by a Tego option.
type Function struct {
	Ref        string
	ImportPath string
	Name       string
	Func       *types.Func
	Signature  *types.Signature
}

// Method is a loaded Go method referenced by a Tego option.
type Method struct {
	Ref        string
	ImportPath string
	Receiver   string
	Name       string
	Func       *types.Func
	Signature  *types.Signature
}
