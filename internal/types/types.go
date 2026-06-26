package types

import "go/types"

type Type struct {
	Ref        string
	ImportPath string
	Name       string
	TypeName   *types.TypeName
	Type       types.Type
	Named      *types.Named
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
