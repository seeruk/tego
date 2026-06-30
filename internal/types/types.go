package types

import "go/types"

type TypeExprKind uint

const (
	TypeExprKindNamed TypeExprKind = iota
	TypeExprKindPointer
	TypeExprKindSlice
)

type Type struct {
	Ref        string
	ImportPath string
	Name       string
	TypeName   *types.TypeName
	Type       types.Type
	Named      *types.Named
}

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

type Function struct {
	Ref        string
	ImportPath string
	Name       string
	Func       *types.Func
	Signature  *types.Signature
}

type Method struct {
	Ref        string
	ImportPath string
	Receiver   string
	Name       string
	Func       *types.Func
	Signature  *types.Signature
}
