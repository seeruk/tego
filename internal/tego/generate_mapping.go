package tego

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

const structpbImportPath = "google.golang.org/protobuf/types/known/structpb"

func generateMapping(g *protogen.GeneratedFile, mapping MappingPlan) error {
	if err := generateFromProtoMapping(g, mapping); err != nil {
		return err
	}
	if err := generateToProtoMapping(g, mapping); err != nil {
		return err
	}
	return generateToProtoMethod(g, mapping)
}

func generateFromProtoMapping(g *protogen.GeneratedFile, mapping MappingPlan) error {
	sourceType, err := generateType(g, mapping.FromProto.Source)
	if err != nil {
		return fmt.Errorf("from proto source: %w", err)
	}
	targetType, err := generateType(g, mapping.FromProto.Target)
	if err != nil {
		return fmt.Errorf("from proto target: %w", err)
	}

	g.P("func ", mapping.FromProto.Name, "(source ", sourceType, ") ", mappingResults(targetType, mapping.FromProto.CanError), " {")
	g.P("\tvar target ", targetType)
	if mapping.FromProto.CanError {
		g.P("\tif source == nil {")
		g.P("\t\treturn target, nil")
		g.P("\t}")
	} else {
		g.P("\tif source == nil {")
		g.P("\t\treturn target")
		g.P("\t}")
	}

	ctx := newMappingRenderContext(g, mapping.FromProto.CanError, "target, err")
	for _, field := range mapping.Fields {
		if err := generateFromProtoField(ctx, field); err != nil {
			return fmt.Errorf("from proto field %s: %w", field.ProtoName, err)
		}
	}

	if mapping.FromProto.CanError {
		g.P("\treturn target, nil")
	} else {
		g.P("\treturn target")
	}
	g.P("}")
	g.P()
	return nil
}

func generateToProtoMapping(g *protogen.GeneratedFile, mapping MappingPlan) error {
	sourceType, err := generateType(g, mapping.ToProto.Source)
	if err != nil {
		return fmt.Errorf("to proto source: %w", err)
	}
	targetType, err := generateType(g, mapping.ToProto.Target)
	if err != nil {
		return fmt.Errorf("to proto target: %w", err)
	}

	g.P("func ", mapping.ToProto.Name, "(source ", sourceType, ") ", mappingResults(targetType, mapping.ToProto.CanError), " {")
	targetExpr, err := newValueExpr(g, mapping.ToProto.Target)
	if err != nil {
		return fmt.Errorf("to proto target: %w", err)
	}
	g.P("\ttarget := ", targetExpr)

	ctx := newMappingRenderContext(g, mapping.ToProto.CanError, "nil, err")
	for _, field := range mapping.Fields {
		if err := generateToProtoField(ctx, field); err != nil {
			return fmt.Errorf("to proto field %s: %w", field.ProtoName, err)
		}
	}

	if mapping.ToProto.CanError {
		g.P("\treturn target, nil")
	} else {
		g.P("\treturn target")
	}
	g.P("}")
	g.P()
	return nil
}

func generateToProtoMethod(g *protogen.GeneratedFile, mapping MappingPlan) error {
	sourceType, err := generateType(g, mapping.ToProto.Source)
	if err != nil {
		return fmt.Errorf("to proto method source: %w", err)
	}
	targetType, err := generateType(g, mapping.ToProto.Target)
	if err != nil {
		return fmt.Errorf("to proto method target: %w", err)
	}

	receiver := mapping.ToProto.ReceiverName
	if receiver == "" {
		return fmt.Errorf("to proto method receiver name is empty")
	}

	g.P("func (", receiver, " ", sourceType, ") ToProto() ", mappingResults(targetType, mapping.ToProto.CanError), " {")
	g.P("\treturn ", mapping.ToProto.Name, "(", receiver, ")")
	g.P("}")
	g.P()
	return nil
}

func mappingResults(typ string, canError bool) string {
	if canError {
		return "(" + typ + ", error)"
	}
	return typ
}

func generateFromProtoField(ctx *mappingRenderContext, field FieldMappingPlan) error {
	if field.FromProto.Kind == MappingValueKindOneof {
		return generateFromProtoOneofField(ctx, field)
	}
	if field.FromProto.Kind == MappingValueKindOmittable {
		return generateFromProtoOmittableField(ctx, field)
	}

	expr, err := ctx.renderValue(field.FromProto, "source."+field.Proto.Getter+"()")
	if err != nil {
		return err
	}
	ctx.line("target." + field.Name + " = " + expr)
	return nil
}

func generateToProtoField(ctx *mappingRenderContext, field FieldMappingPlan) error {
	if field.ToProto.Kind == MappingValueKindOneof {
		return generateToProtoOneofField(ctx, field)
	}
	if field.ToProto.Kind == MappingValueKindOmittable {
		return generateToProtoOmittableField(ctx, field)
	}

	expr, err := ctx.renderValue(field.ToProto, "source."+field.Name)
	if err != nil {
		return err
	}
	ctx.line("target." + field.Proto.Setter + "(" + expr + ")")
	return nil
}

func generateFromProtoOneofField(ctx *mappingRenderContext, field FieldMappingPlan) error {
	oneof := field.FromProto.Oneof
	if oneof == nil {
		return fmt.Errorf("oneof mapping is missing metadata")
	}

	ctx.line("switch source." + oneof.Which + "() {")
	ctx.indent++
	for _, variant := range oneof.Variants {
		ctx.line("case " + generateNamedType(ctx.g, variant.Case) + ":")
		ctx.indent++
		expr, err := ctx.renderValue(variant.Value, "source."+variant.Proto.Getter+"()")
		if err != nil {
			return fmt.Errorf("variant %s: %w", variant.ProtoName, err)
		}
		ctx.line("target." + field.Name + " = " + variant.Name + "{" + variant.FieldName + ": " + expr + "}")
		ctx.indent--
	}
	ctx.indent--
	ctx.line("}")
	return nil
}

func generateToProtoOneofField(ctx *mappingRenderContext, field FieldMappingPlan) error {
	oneof := field.ToProto.Oneof
	if oneof == nil {
		return fmt.Errorf("oneof mapping is missing metadata")
	}
	if !ctx.canError {
		return fmt.Errorf("oneof to proto mapping requires an erroring mapper")
	}

	value := ctx.tempName("value")
	ctx.line("switch " + value + " := source." + field.Name + ".(type) {")
	ctx.indent++
	ctx.line("case nil:")
	for _, variant := range oneof.Variants {
		if err := generateToProtoOneofVariant(ctx, variant.Name, value, variant); err != nil {
			return err
		}
		if err := generateToProtoOneofVariant(ctx, "*"+variant.Name, value, variant); err != nil {
			return err
		}
	}
	ctx.line("default:")
	ctx.indent++
	ctx.line("return nil, " + errorsNew(ctx.g) + "(" + fmt.Sprintf("%q", "unsupported oneof implementation") + ")")
	ctx.indent--
	ctx.indent--
	ctx.line("}")
	return nil
}

func generateToProtoOneofVariant(
	ctx *mappingRenderContext,
	caseType string,
	value string,
	variant MappingOneofVariantPlan,
) error {
	ctx.line("case " + caseType + ":")
	ctx.indent++
	if strings.HasPrefix(caseType, "*") {
		ctx.line("if " + value + " != nil {")
		ctx.indent++
	}
	expr, err := ctx.renderValue(variant.Value, value+"."+variant.FieldName)
	if err != nil {
		return fmt.Errorf("variant %s: %w", variant.ProtoName, err)
	}
	ctx.line("target." + variant.Proto.Setter + "(" + expr + ")")
	if strings.HasPrefix(caseType, "*") {
		ctx.indent--
		ctx.line("}")
	}
	ctx.indent--
	return nil
}

func generateFromProtoOmittableField(ctx *mappingRenderContext, field FieldMappingPlan) error {
	if field.FromProto.Elem == nil {
		return fmt.Errorf("omittable mapping is missing an element")
	}

	if field.FromProto.Target.Elem == nil {
		return fmt.Errorf("omittable target is missing an element")
	}

	elemType, err := generateType(ctx.g, *field.FromProto.Target.Elem)
	if err != nil {
		return err
	}

	some := generateNamedType(ctx.g, GoTypeRef{ImportPath: omittableImportPath, Name: "Some"})
	none := generateNamedType(ctx.g, GoTypeRef{ImportPath: omittableImportPath, Name: "None"})

	ctx.line("if source." + field.Proto.Has + "() {")
	ctx.indent++
	expr, err := ctx.renderValue(*field.FromProto.Elem, "source."+field.Proto.Getter+"()")
	if err != nil {
		return err
	}
	ctx.line("target." + field.Name + " = " + some + "(" + expr + ")")
	ctx.indent--
	ctx.line("} else {")
	ctx.indent++
	ctx.line("target." + field.Name + " = " + none + "[" + elemType + "]()")
	ctx.indent--
	ctx.line("}")
	return nil
}

func generateToProtoOmittableField(ctx *mappingRenderContext, field FieldMappingPlan) error {
	if field.ToProto.Elem == nil {
		return fmt.Errorf("omittable mapping is missing an element")
	}

	ctx.line("if source." + field.Name + ".Valid {")
	ctx.indent++
	expr, err := ctx.renderValue(*field.ToProto.Elem, "source."+field.Name+".Value")
	if err != nil {
		return err
	}
	ctx.line("target." + field.Proto.Setter + "(" + expr + ")")
	ctx.indent--
	ctx.line("}")
	return nil
}

type mappingRenderContext struct {
	g           *protogen.GeneratedFile
	canError    bool
	errorReturn string
	temp        int
	indent      int
}

func newMappingRenderContext(g *protogen.GeneratedFile, canError bool, errorReturn string) *mappingRenderContext {
	return &mappingRenderContext{
		g:           g,
		canError:    canError,
		errorReturn: errorReturn,
		indent:      1,
	}
}

func (ctx *mappingRenderContext) line(line string) {
	ctx.g.P(strings.Repeat("\t", ctx.indent), line)
}

func (ctx *mappingRenderContext) tempName(prefix string) string {
	ctx.temp++
	return fmt.Sprintf("%s%d", prefix, ctx.temp)
}

func (ctx *mappingRenderContext) renderValue(plan MappingValuePlan, source string) (string, error) {
	switch plan.Kind {
	case MappingValueKindDirect:
		return source, nil
	case MappingValueKindScalarCast, MappingValueKindEnum:
		target, err := generateType(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		return target + "(" + source + ")", nil
	case MappingValueKindStruct:
		if plan.Struct == nil {
			return "", fmt.Errorf("struct mapping is missing a ref")
		}
		return ctx.renderCall(plan.Struct.Name, source, plan.CanError)
	case MappingValueKindCustom:
		if plan.Custom == nil {
			return "", fmt.Errorf("custom mapping is missing conversion refs")
		}
		return ctx.renderCustom(plan, source)
	case MappingValueKindNullable:
		return ctx.renderNullable(plan, source)
	case MappingValueKindSlice:
		return ctx.renderSlice(plan, source)
	case MappingValueKindMap:
		return ctx.renderMap(plan, source)
	case MappingValueKindStructMap:
		return ctx.renderStructMap(plan, source)
	case MappingValueKindOmittable:
		return "", fmt.Errorf("omittable mapping must be rendered at field level")
	default:
		return "", fmt.Errorf("unsupported mapping node")
	}
}

func (ctx *mappingRenderContext) renderCustom(plan MappingValuePlan, source string) (string, error) {
	ref := plan.Custom.FromProto
	if top, ok := topCustomType(plan.Source); ok && top.ToProto.Name != "" {
		ref = plan.Custom.ToProto
	}

	if ref.Receiver != "" {
		return ctx.renderCall(source+"."+ref.Name, "", plan.CanError)
	}
	name := generateSymbol(ctx.g, ref)
	return ctx.renderCall(name, source, plan.CanError)
}

func (ctx *mappingRenderContext) renderCall(name string, source string, canError bool) (string, error) {
	call := name + "(" + source + ")"
	if source == "" {
		call = name + "()"
	}
	if !canError {
		return call, nil
	}

	tmp := ctx.tempName("mapped")
	ctx.line(tmp + ", err := " + call)
	ctx.line("if err != nil {")
	ctx.indent++
	ctx.line("return " + ctx.errorReturn)
	ctx.indent--
	ctx.line("}")
	return tmp, nil
}

func (ctx *mappingRenderContext) renderNullable(plan MappingValuePlan, source string) (string, error) {
	if plan.Elem == nil {
		return "", fmt.Errorf("nullable mapping is missing an element")
	}
	if isProtoPointer(plan.Source) {
		return ctx.renderNullableFromProto(plan, source)
	}
	return ctx.renderNullableToProto(plan, source)
}

func (ctx *mappingRenderContext) renderNullableFromProto(plan MappingValuePlan, source string) (string, error) {
	targetType, err := generateType(ctx.g, plan.Target)
	if err != nil {
		return "", err
	}
	tmp := ctx.tempName("nullable")
	ctx.line("var " + tmp + " " + targetType)

	switch plan.Access.NullableForm {
	case MappingNullableFormOneof:
		ctx.line("if " + source + " != nil && " + source + "." + plan.Access.Oneof.Which + "() == " + generateNamedType(ctx.g, plan.Access.Oneof.ValueRef) + " {")
		ctx.indent++
		if err := ctx.renderNullableFromValue(plan, tmp, source+"."+plan.Access.Oneof.Value.Getter+"()"); err != nil {
			return "", err
		}
		ctx.indent--
		ctx.line("}")
	case MappingNullableFormValue:
		ctx.line("if " + source + " != nil && " + source + "." + plan.Access.Valid.Getter + "() {")
		ctx.indent++
		if err := ctx.renderNullableFromValue(plan, tmp, source+"."+plan.Access.Value.Getter+"()"); err != nil {
			return "", err
		}
		ctx.indent--
		ctx.line("}")
	default:
		ctx.line("if " + source + " != nil {")
		ctx.indent++
		childSource := mappingChildSource(source, plan.Source, plan.Elem.Source)
		if err := ctx.renderNullableFromValue(plan, tmp, childSource); err != nil {
			return "", err
		}
		ctx.indent--
		ctx.line("}")
	}

	return tmp, nil
}

func (ctx *mappingRenderContext) renderNullableFromValue(plan MappingValuePlan, target, source string) error {
	child, err := ctx.renderValue(*plan.Elem, source)
	if err != nil {
		return err
	}
	if plan.Elem.Target.Kind == TypeKindPointer {
		ctx.line(target + " = " + child)
	} else {
		ctx.line(target + " = new(" + child + ")")
	}
	return nil
}

func (ctx *mappingRenderContext) renderNullableToProto(plan MappingValuePlan, source string) (string, error) {
	targetType, err := generateType(ctx.g, plan.Target)
	if err != nil {
		return "", err
	}
	tmp := ctx.tempName("nullable")

	if plan.Target.Kind == TypeKindPointer && plan.Target.Elem != nil && plan.Target.Elem.Kind == TypeKindExternal {
		// Nullable shape wrappers can encode explicit null; ordinary pointer mappings only omit it.
		empty, err := newValueExpr(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		ctx.line("var " + tmp + " " + targetType)
		ctx.line("if " + source + " != nil {")
		ctx.indent++
		ctx.line(tmp + " = " + empty)
		childSource := mappingChildSource(source, plan.Source, plan.Elem.Source)
		child, err := ctx.renderValue(*plan.Elem, childSource)
		if err != nil {
			return "", err
		}
		switch plan.Access.NullableForm {
		case MappingNullableFormOneof:
			ctx.line(tmp + "." + plan.Access.Oneof.Value.Setter + "(" + child + ")")
		case MappingNullableFormValue:
			ctx.line(tmp + "." + plan.Access.Value.Setter + "(" + child + ")")
			ctx.line(tmp + "." + plan.Access.Valid.Setter + "(true)")
		default:
			ctx.line(tmp + " = " + child)
		}
		ctx.indent--
		ctx.line("} else {")
		ctx.indent++
		switch plan.Access.NullableForm {
		case MappingNullableFormOneof:
			ctx.line(tmp + " = " + empty)
			ctx.line(tmp + "." + plan.Access.Oneof.Null.Setter + "(" + structpbNullValue(ctx.g) + ")")
		case MappingNullableFormValue:
			ctx.line(tmp + " = " + empty)
			ctx.line(tmp + "." + plan.Access.Valid.Setter + "(false)")
		}
		ctx.indent--
		ctx.line("}")
		return tmp, nil
	}

	ctx.line("var " + tmp + " " + targetType)
	ctx.line("if " + source + " != nil {")
	ctx.indent++
	childSource := mappingChildSource(source, plan.Source, plan.Elem.Source)
	child, err := ctx.renderValue(*plan.Elem, childSource)
	if err != nil {
		return "", err
	}
	ctx.line(tmp + " = " + child)
	ctx.indent--
	ctx.line("}")
	return tmp, nil
}

func (ctx *mappingRenderContext) renderSlice(plan MappingValuePlan, source string) (string, error) {
	if plan.Elem == nil {
		return "", fmt.Errorf("slice mapping is missing an element")
	}
	if plan.Source.Kind == TypeKindPointer || plan.Target.Kind == TypeKindPointer {
		return ctx.renderShapeSlice(plan, source)
	}
	return ctx.renderNativeSlice(plan, source, plan.Target, *plan.Elem)
}

func (ctx *mappingRenderContext) renderNativeSlice(
	plan MappingValuePlan,
	source string,
	target TypePlan,
	elem MappingValuePlan,
) (string, error) {
	targetType, err := generateType(ctx.g, target)
	if err != nil {
		return "", err
	}
	tmp := ctx.tempName("items")
	item := ctx.tempName("item")
	ctx.line(tmp + " := make(" + targetType + ", 0, len(" + source + "))")
	ctx.line("for _, " + item + " := range " + source + " {")
	ctx.indent++
	mapped, err := ctx.renderValue(elem, item)
	if err != nil {
		return "", err
	}
	ctx.line(tmp + " = append(" + tmp + ", " + mapped + ")")
	ctx.indent--
	ctx.line("}")
	return tmp, nil
}

func (ctx *mappingRenderContext) renderShapeSlice(plan MappingValuePlan, source string) (string, error) {
	if plan.Target.Kind == TypeKindSlice {
		inner := ctx.tempName("sourceItems")
		sourceElemType, err := generateType(ctx.g, plan.Elem.Source)
		if err != nil {
			return "", fmt.Errorf("shape slice source element: %w", err)
		}
		ctx.line("var " + inner + " []" + sourceElemType)
		ctx.line("if " + source + " != nil {")
		ctx.indent++
		ctx.line(inner + " = " + source + "." + plan.Access.Field.Getter + "()")
		ctx.indent--
		ctx.line("}")
		return ctx.renderNativeSlice(plan, inner, plan.Target, *plan.Elem)
	}

	inner, err := ctx.renderNativeSlice(plan, source, TypePlan{Kind: TypeKindSlice, Elem: &plan.Elem.Target}, *plan.Elem)
	if err != nil {
		return "", err
	}
	tmp := ctx.tempName("slice")
	empty, err := newValueExpr(ctx.g, plan.Target)
	if err != nil {
		return "", err
	}
	ctx.line(tmp + " := " + empty)
	ctx.line(tmp + "." + plan.Access.Field.Setter + "(" + inner + ")")
	return tmp, nil
}

func (ctx *mappingRenderContext) renderMap(plan MappingValuePlan, source string) (string, error) {
	if plan.Key == nil || plan.Value == nil {
		return "", fmt.Errorf("map mapping is missing key or value")
	}
	if plan.Source.Kind == TypeKindPointer || plan.Target.Kind == TypeKindPointer {
		return ctx.renderShapeMap(plan, source)
	}
	return ctx.renderNativeMap(plan, source, plan.Target, *plan.Key, *plan.Value)
}

func (ctx *mappingRenderContext) renderStructMap(plan MappingValuePlan, source string) (string, error) {
	if isStructpbStructPointer(plan.Source) && isStringAnyMap(plan.Target) {
		targetType, err := generateType(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		tmp := ctx.tempName("structMap")
		ctx.line("var " + tmp + " " + targetType)
		ctx.line("if " + source + " != nil {")
		ctx.indent++
		ctx.line(tmp + " = " + source + ".AsMap()")
		ctx.indent--
		ctx.line("}")
		return tmp, nil
	}

	if isStringAnyMap(plan.Source) && isStructpbStructPointer(plan.Target) {
		if !ctx.canError {
			return "", fmt.Errorf("struct map to proto mapping requires an erroring mapper")
		}
		targetType, err := generateType(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		tmp := ctx.tempName("struct")
		ctx.line("var " + tmp + " " + targetType)
		ctx.line("if " + source + " != nil {")
		ctx.indent++
		mapped := ctx.tempName("mapped")
		ctx.line(mapped + ", err := " + structpbNewStruct(ctx.g) + "(" + source + ")")
		ctx.line("if err != nil {")
		ctx.indent++
		ctx.line("return " + ctx.errorReturn)
		ctx.indent--
		ctx.line("}")
		ctx.line(tmp + " = " + mapped)
		ctx.indent--
		ctx.line("}")
		return tmp, nil
	}

	return "", fmt.Errorf("unsupported struct map mapping")
}

func (ctx *mappingRenderContext) renderNativeMap(
	plan MappingValuePlan,
	source string,
	target TypePlan,
	keyPlan MappingValuePlan,
	valuePlan MappingValuePlan,
) (string, error) {
	targetType, err := generateType(ctx.g, target)
	if err != nil {
		return "", err
	}
	tmp := ctx.tempName("items")
	key := ctx.tempName("key")
	value := ctx.tempName("value")
	ctx.line(tmp + " := make(" + targetType + ", len(" + source + "))")
	ctx.line("for " + key + ", " + value + " := range " + source + " {")
	ctx.indent++
	mappedKey, err := ctx.renderValue(keyPlan, key)
	if err != nil {
		return "", err
	}
	mappedValue, err := ctx.renderValue(valuePlan, value)
	if err != nil {
		return "", err
	}
	ctx.line(tmp + "[" + mappedKey + "] = " + mappedValue)
	ctx.indent--
	ctx.line("}")
	return tmp, nil
}

func (ctx *mappingRenderContext) renderShapeMap(plan MappingValuePlan, source string) (string, error) {
	if plan.Target.Kind == TypeKindMap {
		targetType, err := generateType(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		tmp := ctx.tempName("items")
		entry := ctx.tempName("entry")
		ctx.line(tmp + " := make(" + targetType + ")")
		ctx.line("if " + source + " != nil {")
		ctx.indent++
		ctx.line("for _, " + entry + " := range " + source + "." + plan.Access.Field.Getter + "() {")
		ctx.indent++
		mappedKey, err := ctx.renderValue(*plan.Key, entry+"."+plan.Access.Key.Getter+"()")
		if err != nil {
			return "", err
		}
		mappedValue, err := ctx.renderValue(*plan.Value, entry+"."+plan.Access.Value.Getter+"()")
		if err != nil {
			return "", err
		}
		ctx.line(tmp + "[" + mappedKey + "] = " + mappedValue)
		ctx.indent--
		ctx.line("}")
		ctx.indent--
		ctx.line("}")
		return tmp, nil
	}

	entryType, err := generateType(ctx.g, plan.Access.ProtoElemType)
	if err != nil {
		return "", err
	}
	entries := ctx.tempName("entries")
	key := ctx.tempName("key")
	value := ctx.tempName("value")
	ctx.line(entries + " := make([]" + entryType + ", 0, len(" + source + "))")
	ctx.line("for " + key + ", " + value + " := range " + source + " {")
	ctx.indent++
	entry := ctx.tempName("entry")
	empty, err := newValueExpr(ctx.g, plan.Access.ProtoElemType)
	if err != nil {
		return "", err
	}
	ctx.line(entry + " := " + empty)
	mappedKey, err := ctx.renderValue(*plan.Key, key)
	if err != nil {
		return "", err
	}
	mappedValue, err := ctx.renderValue(*plan.Value, value)
	if err != nil {
		return "", err
	}
	ctx.line(entry + "." + plan.Access.Key.Setter + "(" + mappedKey + ")")
	ctx.line(entry + "." + plan.Access.Value.Setter + "(" + mappedValue + ")")
	ctx.line(entries + " = append(" + entries + ", " + entry + ")")
	ctx.indent--
	ctx.line("}")

	tmp := ctx.tempName("mapping")
	wrapper, err := newValueExpr(ctx.g, plan.Target)
	if err != nil {
		return "", err
	}
	ctx.line(tmp + " := " + wrapper)
	ctx.line(tmp + "." + plan.Access.Field.Setter + "(" + entries + ")")
	return tmp, nil
}

func generateSymbol(g *protogen.GeneratedFile, ref GoSymbolRef) string {
	if ref.ImportPath == "" {
		return ref.Name
	}
	return g.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: protogen.GoImportPath(ref.ImportPath),
		GoName:       ref.Name,
	})
}

func structpbNullValue(g *protogen.GeneratedFile) string {
	return g.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: structpbImportPath,
		GoName:       "NullValue_NULL_VALUE",
	})
}

func structpbNewStruct(g *protogen.GeneratedFile) string {
	return g.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: structpbImportPath,
		GoName:       "NewStruct",
	})
}

func errorsNew(g *protogen.GeneratedFile) string {
	return g.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: "errors",
		GoName:       "New",
	})
}

func newValueExpr(g *protogen.GeneratedFile, plan TypePlan) (string, error) {
	if plan.Kind == TypeKindPointer && plan.Elem != nil {
		elem, err := generateType(g, *plan.Elem)
		if err != nil {
			return "", err
		}
		return "new(" + elem + ")", nil
	}
	typ, err := generateType(g, plan)
	if err != nil {
		return "", err
	}
	return typ + "{}", nil
}

func mappingChildSource(source string, sourceType TypePlan, childSource TypePlan) string {
	if sourceType.Kind == TypeKindPointer && childSource.Kind != TypeKindPointer {
		return "*" + source
	}
	return source
}

func isProtoPointer(plan TypePlan) bool {
	return plan.Kind == TypeKindPointer && plan.Elem != nil && plan.Elem.Kind == TypeKindExternal
}
