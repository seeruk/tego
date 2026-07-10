package types

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	gotypes "go/types"
)

// TypeExpr resolves a Go type expression using Tego's supported subset: named and predeclared
// types, type parameters, pointers, slices, arrays, maps, and generic instantiations.
func (l *Loader) TypeExpr(ref string, typeArgs map[string]string) (*TypeExpr, error) {
	parsed, err := parseTypeExpr(ref)
	if err != nil {
		return nil, err
	}

	used := make(map[string]bool)
	expr, err := l.resolveTypeExpr(parsed.expr, parsed.refs, typeArgs, used, nil)
	if err != nil {
		return nil, fmt.Errorf("type expression %q: %w", ref, err)
	}
	for name := range typeArgs {
		if !used[name] {
			return nil, fmt.Errorf("type argument %q is unused", name)
		}
	}
	return expr, nil
}

type parsedTypeExpr struct {
	expr ast.Expr
	refs map[string]string
}

func (l *Loader) resolveTypeExpr(
	expr ast.Expr,
	refs map[string]string,
	typeArgs map[string]string,
	used map[string]bool,
	active map[string]bool,
) (*TypeExpr, error) {
	switch expr := expr.(type) {
	case *ast.ParenExpr:
		return l.resolveTypeExpr(expr.X, refs, typeArgs, used, active)
	case *ast.Ident:
		if ref, ok := refs[expr.Name]; ok {
			return l.resolveNamedTypeExpr(ref, nil, refs, typeArgs, used, active)
		}
		if expr.Name == "comparable" {
			return nil, fmt.Errorf("constraint-only type %q is not supported as a value type", expr.Name)
		}
		if typ, ok := predeclaredType(expr.Name); ok {
			return &TypeExpr{
				Kind: TypeExprKindPredeclared,
				Name: expr.Name,
				Type: typ,
			}, nil
		}
		return l.resolveTypeParam(expr.Name, typeArgs, used, active)
	case *ast.StarExpr:
		elem, err := l.resolveTypeExpr(expr.X, refs, typeArgs, used, active)
		if err != nil {
			return nil, err
		}
		return &TypeExpr{
			Kind: TypeExprKindPointer,
			Type: gotypes.NewPointer(elem.Type),
			Elem: elem,
		}, nil
	case *ast.ArrayType:
		return l.resolveArrayTypeExpr(expr, refs, typeArgs, used, active)
	case *ast.MapType:
		return l.resolveMapTypeExpr(expr, refs, typeArgs, used, active)
	case *ast.IndexExpr:
		return l.resolveGenericTypeExpr(expr.X, []ast.Expr{expr.Index}, refs, typeArgs, used, active)
	case *ast.IndexListExpr:
		return l.resolveGenericTypeExpr(expr.X, expr.Indices, refs, typeArgs, used, active)
	case *ast.ChanType:
		return nil, fmt.Errorf("channel types are not supported")
	case *ast.FuncType:
		return nil, fmt.Errorf("function types are not supported")
	case *ast.StructType:
		return nil, fmt.Errorf("anonymous struct types are not supported")
	case *ast.InterfaceType:
		return nil, fmt.Errorf("anonymous interface types are not supported")
	case *ast.UnaryExpr:
		return nil, fmt.Errorf("type constraints are not supported")
	default:
		return nil, fmt.Errorf("unsupported type expression")
	}
}

func (l *Loader) resolveTypeParam(
	name string,
	typeArgs map[string]string,
	used map[string]bool,
	active map[string]bool,
) (*TypeExpr, error) {
	arg, ok := typeArgs[name]
	if !ok {
		return nil, fmt.Errorf("type parameter %q has no type argument", name)
	}
	if active[name] {
		return nil, fmt.Errorf("type argument %q is recursive", name)
	}
	if active == nil {
		active = make(map[string]bool)
	}
	active[name] = true
	defer delete(active, name)

	used[name] = true
	parsed, err := parseTypeExpr(arg)
	if err != nil {
		return nil, fmt.Errorf("type argument %q: %w", name, err)
	}
	resolved, err := l.resolveTypeExpr(parsed.expr, parsed.refs, typeArgs, used, active)
	if err != nil {
		return nil, fmt.Errorf("type argument %q: %w", name, err)
	}
	return resolved, nil
}

func (l *Loader) resolveArrayTypeExpr(
	expr *ast.ArrayType,
	refs map[string]string,
	typeArgs map[string]string,
	used map[string]bool,
	active map[string]bool,
) (*TypeExpr, error) {
	elem, err := l.resolveTypeExpr(expr.Elt, refs, typeArgs, used, active)
	if err != nil {
		return nil, err
	}
	if expr.Len == nil {
		return &TypeExpr{
			Kind: TypeExprKindSlice,
			Type: gotypes.NewSlice(elem.Type),
			Elem: elem,
		}, nil
	}

	literal, ok := expr.Len.(*ast.BasicLit)
	if !ok || literal.Kind != token.INT {
		if _, inferred := expr.Len.(*ast.Ellipsis); inferred {
			return nil, fmt.Errorf("inferred-length arrays are not supported")
		}
		return nil, fmt.Errorf("array length must be a non-negative integer literal")
	}
	length, ok := constant.Int64Val(constant.MakeFromLiteral(literal.Value, token.INT, 0))
	if !ok || length < 0 {
		return nil, fmt.Errorf("array length %q is invalid or overflows int64", literal.Value)
	}

	return &TypeExpr{
		Kind:   TypeExprKindArray,
		Type:   gotypes.NewArray(elem.Type, length),
		Elem:   elem,
		Length: length,
	}, nil
}

func (l *Loader) resolveMapTypeExpr(
	expr *ast.MapType,
	refs map[string]string,
	typeArgs map[string]string,
	used map[string]bool,
	active map[string]bool,
) (*TypeExpr, error) {
	key, err := l.resolveTypeExpr(expr.Key, refs, typeArgs, used, active)
	if err != nil {
		return nil, fmt.Errorf("map key: %w", err)
	}
	if !gotypes.Comparable(key.Type) {
		return nil, fmt.Errorf("map key type is not comparable")
	}
	value, err := l.resolveTypeExpr(expr.Value, refs, typeArgs, used, active)
	if err != nil {
		return nil, fmt.Errorf("map value: %w", err)
	}

	return &TypeExpr{
		Kind:  TypeExprKindMap,
		Type:  gotypes.NewMap(key.Type, value.Type),
		Key:   key,
		Value: value,
	}, nil
}

func (l *Loader) resolveGenericTypeExpr(
	base ast.Expr,
	args []ast.Expr,
	refs map[string]string,
	typeArgs map[string]string,
	used map[string]bool,
	active map[string]bool,
) (*TypeExpr, error) {
	for {
		paren, ok := base.(*ast.ParenExpr)
		if !ok {
			break
		}
		base = paren.X
	}

	ident, ok := base.(*ast.Ident)
	if !ok {
		return nil, fmt.Errorf("generic base must be a named type")
	}
	ref, ok := refs[ident.Name]
	if !ok {
		if ident.Name == "comparable" {
			return nil, fmt.Errorf("constraint-only type %q is not supported as a value type", ident.Name)
		}
		if _, predeclared := predeclaredType(ident.Name); predeclared {
			return nil, fmt.Errorf("predeclared type %q cannot have type arguments", ident.Name)
		}
		return nil, fmt.Errorf("type parameter %q cannot have type arguments", ident.Name)
	}
	return l.resolveNamedTypeExpr(ref, args, refs, typeArgs, used, active)
}

func (l *Loader) resolveNamedTypeExpr(
	ref string,
	parsedArgs []ast.Expr,
	refs map[string]string,
	typeArgs map[string]string,
	used map[string]bool,
	active map[string]bool,
) (*TypeExpr, error) {
	typ, err := l.Type(ref)
	if err != nil {
		return nil, err
	}
	if typ.Named == nil {
		return nil, fmt.Errorf("%q is not a named type", ref)
	}

	var args []TypeExpr
	var argTypes []gotypes.Type
	for _, parsedArg := range parsedArgs {
		arg, err := l.resolveTypeExpr(parsedArg, refs, typeArgs, used, active)
		if err != nil {
			return nil, err
		}
		args = append(args, *arg)
		argTypes = append(argTypes, arg.Type)
	}

	typeParamCount := typ.Named.TypeParams().Len()
	if typeParamCount != len(args) {
		return nil, fmt.Errorf("type %q requires %d type arguments, got %d", ref, typeParamCount, len(args))
	}

	exprType := typ.Type
	named := typ.Named
	if len(argTypes) > 0 {
		instantiated, err := gotypes.Instantiate(nil, typ.Named, argTypes, true)
		if err != nil {
			return nil, fmt.Errorf("instantiate type %q: %w", ref, err)
		}
		exprType = instantiated
		named, _ = instantiated.(*gotypes.Named)
	}

	return &TypeExpr{
		Ref:        ref,
		Kind:       TypeExprKindNamed,
		ImportPath: typ.ImportPath,
		Name:       typ.Name,
		TypeName:   typ.TypeName,
		Type:       exprType,
		Named:      named,
		Args:       args,
	}, nil
}

func predeclaredType(name string) (gotypes.Type, bool) {
	if name == "comparable" {
		return nil, false
	}
	obj, ok := gotypes.Universe.Lookup(name).(*gotypes.TypeName)
	if !ok {
		return nil, false
	}
	return obj.Type(), true
}

func parseTypeExpr(input string) (parsedTypeExpr, error) {
	normalized, refs := normalizeQualifiedTypeRefs(strings.TrimSpace(input))
	expr, err := parser.ParseExpr(normalized)
	if err != nil {
		message := restoreQualifiedTypeRefs(err.Error(), refs)
		return parsedTypeExpr{}, fmt.Errorf("invalid type expression %q: %s", input, message)
	}
	return parsedTypeExpr{expr: expr, refs: refs}, nil
}

func restoreQualifiedTypeRefs(value string, refs map[string]string) string {
	syntheticRefs := make([]string, 0, len(refs))
	for synthetic := range refs {
		syntheticRefs = append(syntheticRefs, synthetic)
	}
	sort.Slice(syntheticRefs, func(i, j int) bool {
		return len(syntheticRefs[i]) > len(syntheticRefs[j])
	})
	for _, synthetic := range syntheticRefs {
		value = strings.ReplaceAll(value, synthetic, refs[synthetic])
	}
	return value
}

func normalizeQualifiedTypeRefs(input string) (string, map[string]string) {
	refs := make(map[string]string)
	syntheticPrefix := "__tego_type_ref_"
	for strings.Contains(input, syntheticPrefix) {
		syntheticPrefix = "_" + syntheticPrefix
	}
	var normalized strings.Builder
	for pos := 0; pos < len(input); {
		r, size := rune(input[pos]), 1
		if r >= unicode.MaxASCII {
			r, size = utf8.DecodeRuneInString(input[pos:])
		}
		if typeExprDelimiter(r) {
			normalized.WriteString(input[pos : pos+size])
			pos += size
			continue
		}

		start := pos
		for pos < len(input) {
			r, size = rune(input[pos]), 1
			if r >= unicode.MaxASCII {
				r, size = utf8.DecodeRuneInString(input[pos:])
			}
			if typeExprDelimiter(r) {
				break
			}
			pos += size
		}
		atom := input[start:pos]
		if isQualifiedTypeRef(atom) {
			synthetic := fmt.Sprintf("%s%d", syntheticPrefix, len(refs))
			refs[synthetic] = atom
			normalized.WriteString(synthetic)
		} else {
			normalized.WriteString(atom)
		}
	}
	return normalized.String(), refs
}

func typeExprDelimiter(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune("*[],(){}~|<>;:", r)
}

func isQualifiedTypeRef(atom string) bool {
	if !strings.Contains(atom, ".") || atom == "..." {
		return false
	}
	for _, r := range atom {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
