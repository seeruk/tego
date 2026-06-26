package types

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Loader is a simplified, caching Go type loader that is built to load the symbols required by Tego
// for things like tegopb.GoType's type ref, and to and from proto callables.
type Loader struct {
	config *packages.Config

	packages  map[string]*packages.Package
	types     map[string]*Type
	functions map[string]*Function
	methods   map[string]*Method
}

// LoaderOption allows this Loader to be customized during construction.
type LoaderOption func(*Loader)

// WithPackagesConfig allows custom packages.Config to be specified.
func WithPackagesConfig(config *packages.Config) LoaderOption {
	return func(loader *Loader) {
		loader.config = config
	}
}

// NewLoader returns a new Loader instance.
func NewLoader(opts ...LoaderOption) *Loader {
	loader := &Loader{
		packages:  make(map[string]*packages.Package),
		types:     make(map[string]*Type),
		functions: make(map[string]*Function),
		methods:   make(map[string]*Method),
	}
	for _, opt := range opts {
		opt(loader)
	}
	return loader
}

// Type attempts to load the type with the given ref, returning a Type instance if successful.
func (l *Loader) Type(ref string) (*Type, error) {
	if typ, ok := l.types[ref]; ok {
		return typ, nil
	}

	candidates, err := refCandidates(ref, 1)
	if err != nil {
		return nil, err
	}

	for _, candidate := range candidates {
		pkg, err := l.loadPackage(candidate.importPath)
		if err != nil {
			if len(candidates) == 1 {
				return nil, err
			}
			continue
		}

		obj := pkg.Types.Scope().Lookup(candidate.symbols[0])
		if obj == nil {
			continue
		}
		typeName, ok := obj.(*types.TypeName)
		if !ok {
			return nil, fmt.Errorf("%q is not a type", ref)
		}

		typ := &Type{
			Ref:        ref,
			ImportPath: candidate.importPath,
			Name:       candidate.symbols[0],
			TypeName:   typeName,
			Type:       typeName.Type(),
		}
		typ.Named, _ = typeName.Type().(*types.Named)
		l.types[ref] = typ
		return typ, nil
	}

	return nil, fmt.Errorf("type %q not found", ref)
}

// Function attempts to load the function with the given ref, returning a Function instance if
// successful.
func (l *Loader) Function(ref string) (*Function, error) {
	if function, ok := l.functions[ref]; ok {
		return function, nil
	}

	candidates, err := refCandidates(ref, 1)
	if err != nil {
		return nil, err
	}

	for _, candidate := range candidates {
		pkg, err := l.loadPackage(candidate.importPath)
		if err != nil {
			if len(candidates) == 1 {
				return nil, err
			}
			continue
		}

		obj := pkg.Types.Scope().Lookup(candidate.symbols[0])
		if obj == nil {
			continue
		}
		fn, ok := obj.(*types.Func)
		if !ok {
			return nil, fmt.Errorf("%q is not a function", ref)
		}

		function := &Function{
			Ref:        ref,
			ImportPath: candidate.importPath,
			Name:       candidate.symbols[0],
			Func:       fn,
			Signature:  funcSignature(fn),
		}
		l.functions[ref] = function
		return function, nil
	}

	return nil, fmt.Errorf("function %q not found", ref)
}

// Method attempts to load the method with the given ref, returning a Method instance if successful.
func (l *Loader) Method(ref string) (*Method, error) {
	if method, ok := l.methods[ref]; ok {
		return method, nil
	}

	candidates, err := refCandidates(ref, 2)
	if err != nil {
		return nil, err
	}

	for _, candidate := range candidates {
		pkg, err := l.loadPackage(candidate.importPath)
		if err != nil {
			if len(candidates) == 1 {
				return nil, err
			}
			continue
		}

		obj := pkg.Types.Scope().Lookup(candidate.symbols[0])
		if obj == nil {
			continue
		}

		typeName, ok := obj.(*types.TypeName)
		if !ok {
			return nil, fmt.Errorf("%q receiver is not a type", ref)
		}

		fn := lookupMethod(typeName.Type(), candidate.symbols[1])
		if fn == nil {
			continue
		}

		method := &Method{
			Ref:        ref,
			ImportPath: candidate.importPath,
			Receiver:   candidate.symbols[0],
			Name:       candidate.symbols[1],
			Func:       fn,
			Signature:  funcSignature(fn),
		}
		l.methods[ref] = method
		return method, nil
	}

	return nil, fmt.Errorf("method %q not found", ref)
}

func (l *Loader) loadPackage(importPath string) (*packages.Package, error) {
	if pkg, ok := l.packages[importPath]; ok {
		return pkg, nil
	}

	config := l.config
	if config == nil {
		config = &packages.Config{}
	}

	if config.Mode == 0 {
		config.Mode = packages.NeedName |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedDeps
	}

	pkgs, err := packages.Load(config, importPath)
	if err != nil {
		return nil, fmt.Errorf("load package %q: %w", importPath, err)
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("load package %q: got %d packages", importPath, len(pkgs))
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return nil, fmt.Errorf("load package %q: %s", importPath, pkg.Errors[0].Msg)
	}
	if pkg.Types == nil {
		return nil, fmt.Errorf("load package %q: missing type information", importPath)
	}

	l.packages[importPath] = pkg
	return pkg, nil
}

type refCandidate struct {
	importPath string
	symbols    []string
}

func refCandidates(ref string, symbolCount int) ([]refCandidate, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("ref must not be empty")
	}

	lastSlash := strings.LastIndex(ref, "/")

	var candidates []refCandidate
	for i := len(ref) - 1; i > lastSlash; i-- {
		if ref[i] != '.' {
			continue
		}

		importPath := ref[:i]
		symbols := strings.Split(ref[i+1:], ".")
		if importPath == "" || len(symbols) != symbolCount || !validSymbols(symbols) {
			continue
		}

		candidates = append(candidates, refCandidate{
			importPath: importPath,
			symbols:    symbols,
		})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("malformed ref %q", ref)
	}

	return candidates, nil
}

func validSymbols(symbols []string) bool {
	for _, symbol := range symbols {
		if !token.IsIdentifier(symbol) {
			return false
		}
	}
	return true
}

func lookupMethod(typ types.Type, name string) *types.Func {
	for _, methodSet := range []*types.MethodSet{
		types.NewMethodSet(typ),
		types.NewMethodSet(types.NewPointer(typ)),
	} {
		for i := 0; i < methodSet.Len(); i++ {
			selection := methodSet.At(i)
			if selection.Obj().Name() == name {
				return selection.Obj().(*types.Func)
			}
		}
	}
	return nil
}

func funcSignature(fn *types.Func) *types.Signature {
	signature, ok := fn.Type().(*types.Signature)
	if !ok {
		panic("function's type is not a signature")
	}
	return signature
}
