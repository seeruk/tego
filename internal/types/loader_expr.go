package types

import (
	"fmt"
	"strings"
	"unicode"

	gotypes "go/types"
)

// TypeExpr resolves a Go type expression using Tego's supported subset: named types, type
// parameters, pointers, slices, and generic instantiations.
func (l *Loader) TypeExpr(ref string, typeArgs map[string]string) (*TypeExpr, error) {
	parsed, err := parseTypeExpr(ref)
	if err != nil {
		return nil, err
	}

	used := make(map[string]bool)
	expr, err := l.resolveTypeExpr(parsed, typeArgs, used, nil)
	if err != nil {
		return nil, err
	}
	for name := range typeArgs {
		if !used[name] {
			return nil, fmt.Errorf("type argument %q is unused", name)
		}
	}
	return expr, nil
}

type parsedTypeExprKind uint

const (
	parsedTypeExprKindNamed parsedTypeExprKind = iota
	parsedTypeExprKindParam
	parsedTypeExprKindPointer
	parsedTypeExprKindSlice
)

type parsedTypeExpr struct {
	kind parsedTypeExprKind
	name string
	args []parsedTypeExpr
	elem *parsedTypeExpr
}

func (l *Loader) resolveTypeExpr(
	parsed parsedTypeExpr,
	typeArgs map[string]string,
	used map[string]bool,
	active map[string]bool,
) (*TypeExpr, error) {
	switch parsed.kind {
	case parsedTypeExprKindParam:
		arg, ok := typeArgs[parsed.name]
		if !ok {
			return nil, fmt.Errorf("type parameter %q has no type argument", parsed.name)
		}
		if active[parsed.name] {
			return nil, fmt.Errorf("type argument %q is recursive", parsed.name)
		}
		if active == nil {
			active = make(map[string]bool)
		}
		active[parsed.name] = true
		defer delete(active, parsed.name)

		used[parsed.name] = true
		argParsed, err := parseTypeExpr(arg)
		if err != nil {
			return nil, fmt.Errorf("type argument %q: %w", parsed.name, err)
		}
		return l.resolveTypeExpr(argParsed, typeArgs, used, active)
	case parsedTypeExprKindPointer:
		elem, err := l.resolveTypeExpr(*parsed.elem, typeArgs, used, active)
		if err != nil {
			return nil, err
		}
		return &TypeExpr{
			Kind: TypeExprKindPointer,
			Type: gotypes.NewPointer(elem.Type),
			Elem: elem,
		}, nil
	case parsedTypeExprKindSlice:
		elem, err := l.resolveTypeExpr(*parsed.elem, typeArgs, used, active)
		if err != nil {
			return nil, err
		}
		return &TypeExpr{
			Kind: TypeExprKindSlice,
			Type: gotypes.NewSlice(elem.Type),
			Elem: elem,
		}, nil
	default:
		return l.resolveNamedTypeExpr(parsed, typeArgs, used, active)
	}
}

func (l *Loader) resolveNamedTypeExpr(
	parsed parsedTypeExpr,
	typeArgs map[string]string,
	used map[string]bool,
	active map[string]bool,
) (*TypeExpr, error) {
	typ, err := l.Type(parsed.name)
	if err != nil {
		return nil, err
	}
	if typ.Named == nil {
		return nil, fmt.Errorf("%q is not a named type", parsed.name)
	}

	var args []TypeExpr
	var argTypes []gotypes.Type
	for _, parsedArg := range parsed.args {
		arg, err := l.resolveTypeExpr(parsedArg, typeArgs, used, active)
		if err != nil {
			return nil, err
		}
		args = append(args, *arg)
		argTypes = append(argTypes, arg.Type)
	}

	typeParamCount := typ.Named.TypeParams().Len()
	if typeParamCount != len(args) {
		return nil, fmt.Errorf("type %q requires %d type arguments, got %d", parsed.name, typeParamCount, len(args))
	}

	exprType := typ.Type
	named := typ.Named
	if len(argTypes) > 0 {
		instantiated, err := gotypes.Instantiate(nil, typ.Named, argTypes, true)
		if err != nil {
			return nil, fmt.Errorf("instantiate type %q: %w", parsed.name, err)
		}
		exprType = instantiated
		named, _ = instantiated.(*gotypes.Named)
	}

	return &TypeExpr{
		Ref:        parsed.name,
		Kind:       TypeExprKindNamed,
		ImportPath: typ.ImportPath,
		Name:       typ.Name,
		TypeName:   typ.TypeName,
		Type:       exprType,
		Named:      named,
		Args:       args,
	}, nil
}

type typeExprParser struct {
	input string
	pos   int
}

func parseTypeExpr(input string) (parsedTypeExpr, error) {
	parser := typeExprParser{input: strings.TrimSpace(input)}
	expr, err := parser.parseExpr()
	if err != nil {
		return parsedTypeExpr{}, err
	}
	parser.skipSpace()
	if parser.pos != len(parser.input) {
		return parsedTypeExpr{}, fmt.Errorf("unexpected %q", parser.input[parser.pos:])
	}
	return expr, nil
}

func (p *typeExprParser) parseExpr() (parsedTypeExpr, error) {
	p.skipSpace()
	switch {
	case p.consume("*"):
		elem, err := p.parseExpr()
		if err != nil {
			return parsedTypeExpr{}, err
		}
		return parsedTypeExpr{kind: parsedTypeExprKindPointer, elem: &elem}, nil
	case p.consume("[]"):
		elem, err := p.parseExpr()
		if err != nil {
			return parsedTypeExpr{}, err
		}
		return parsedTypeExpr{kind: parsedTypeExprKindSlice, elem: &elem}, nil
	default:
		return p.parseNamedOrParam()
	}
}

func (p *typeExprParser) parseNamedOrParam() (parsedTypeExpr, error) {
	name := p.parseName()
	if name == "" {
		return parsedTypeExpr{}, fmt.Errorf("expected type expression")
	}

	kind := parsedTypeExprKindNamed
	if tokenIsIdentifier(name) {
		kind = parsedTypeExprKindParam
	}

	expr := parsedTypeExpr{kind: kind, name: name}
	p.skipSpace()
	if !p.consume("[") {
		return expr, nil
	}
	if kind == parsedTypeExprKindParam {
		return parsedTypeExpr{}, fmt.Errorf("type parameter %q cannot have type arguments", name)
	}

	for {
		arg, err := p.parseExpr()
		if err != nil {
			return parsedTypeExpr{}, err
		}
		expr.args = append(expr.args, arg)

		p.skipSpace()
		if p.consume("]") {
			break
		}
		if !p.consume(",") {
			return parsedTypeExpr{}, fmt.Errorf("expected comma or closing bracket")
		}
	}
	return expr, nil
}

func (p *typeExprParser) parseName() string {
	p.skipSpace()
	start := p.pos
	for p.pos < len(p.input) {
		r := rune(p.input[p.pos])
		if unicode.IsSpace(r) || strings.ContainsRune("*[],", r) {
			break
		}
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *typeExprParser) consume(value string) bool {
	if strings.HasPrefix(p.input[p.pos:], value) {
		p.pos += len(value)
		return true
	}
	return false
}

func (p *typeExprParser) skipSpace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func tokenIsIdentifier(value string) bool {
	for i, r := range value {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return value != ""
}
