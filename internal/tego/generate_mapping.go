package tego

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/danielgtaylor/casing"
	"google.golang.org/protobuf/compiler/protogen"
)

const (
	emptypbImportPath  = "google.golang.org/protobuf/types/known/emptypb"
	structpbImportPath = "google.golang.org/protobuf/types/known/structpb"
)

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
	return ctx.withTempNameHint(field.Name, func() error {
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
	})
}

func generateToProtoField(ctx *mappingRenderContext, field FieldMappingPlan) error {
	return ctx.withTempNameHint(field.Name, func() error {
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
	})
}

func generateFromProtoOneofField(ctx *mappingRenderContext, field FieldMappingPlan) error {
	oneof := field.FromProto.Oneof
	if oneof == nil {
		return fmt.Errorf("oneof mapping is missing metadata")
	}

	ctx.line("switch source." + oneof.Which + "() {")
	for _, variant := range oneof.Variants {
		ctx.line("case " + generateNamedType(ctx.g, variant.Case) + ":")
		expr, err := ctx.renderValueWithTempNameHint(variant.FieldName, variant.Value, "source."+variant.Proto.Getter+"()")
		if err != nil {
			return fmt.Errorf("variant %s: %w", variant.ProtoName, err)
		}
		ctx.line("target." + field.Name + " = " + variant.Name + "{" + variant.FieldName + ": " + expr + "}")
	}
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
	ctx.line("return nil, " + errorsNew(ctx.g) + "(" + fmt.Sprintf("%q", "unsupported oneof implementation") + ")")
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
	if strings.HasPrefix(caseType, "*") {
		ctx.line("if " + value + " != nil {")
	}
	expr, err := ctx.renderValueWithTempNameHint(variant.FieldName, variant.Value, value+"."+variant.FieldName)
	if err != nil {
		return fmt.Errorf("variant %s: %w", variant.ProtoName, err)
	}
	ctx.line("target." + variant.Proto.Setter + "(" + expr + ")")
	if strings.HasPrefix(caseType, "*") {
		ctx.line("}")
	}
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
	expr, err := ctx.renderValue(*field.FromProto.Elem, "source."+field.Proto.Getter+"()")
	if err != nil {
		return err
	}
	ctx.line("target." + field.Name + " = " + some + "(" + expr + ")")
	ctx.line("} else {")
	ctx.line("target." + field.Name + " = " + none + "[" + elemType + "]()")
	ctx.line("}")
	return nil
}

func generateToProtoOmittableField(ctx *mappingRenderContext, field FieldMappingPlan) error {
	if field.ToProto.Elem == nil {
		return fmt.Errorf("omittable mapping is missing an element")
	}

	ctx.line("if source." + field.Name + ".Valid {")
	expr, err := ctx.renderValue(*field.ToProto.Elem, "source."+field.Name+".Value")
	if err != nil {
		return err
	}
	ctx.line("target." + field.Proto.Setter + "(" + expr + ")")
	ctx.line("}")
	return nil
}

type mappingRenderContext struct {
	g           *protogen.GeneratedFile
	canError    bool
	errorReturn string
	tempNames   *tempNameAllocator
	tempHint    string
}

func newMappingRenderContext(g *protogen.GeneratedFile, canError bool, errorReturn string) *mappingRenderContext {
	return &mappingRenderContext{
		g:           g,
		canError:    canError,
		errorReturn: errorReturn,
		tempNames:   newTempNameAllocator("source", "target", "err"),
	}
}

func (ctx *mappingRenderContext) line(line string) {
	// protogen formats generated Go files when building the plugin response, so this
	// renderer only needs to preserve statement ordering and line breaks.
	ctx.g.P(line)
}

func (ctx *mappingRenderContext) tempName(role string) string {
	return ctx.tempNameWithRole(role, false)
}

func (ctx *mappingRenderContext) tempPartName(role string) string {
	return ctx.tempNameWithRole(role, true)
}

func (ctx *mappingRenderContext) tempNameWithRole(role string, suffixRole bool) string {
	base := tempIdentifierBase(role)
	if ctx.tempHint != "" {
		if suffixRole && ctx.tempHint != base {
			base = ctx.tempHint + goName(role)
		} else {
			base = ctx.tempHint
		}
	}
	return ctx.tempNames.name(base)
}

func (ctx *mappingRenderContext) withTempNameHint(name string, fn func() error) error {
	previous := ctx.tempHint
	ctx.tempHint = tempIdentifierBase(name)
	defer func() {
		ctx.tempHint = previous
	}()
	return fn()
}

func (ctx *mappingRenderContext) renderValueWithTempNameHint(
	name string,
	plan MappingValuePlan,
	source string,
) (string, error) {
	var expr string
	err := ctx.withTempNameHint(name, func() error {
		var err error
		expr, err = ctx.renderValue(plan, source)
		return err
	})
	return expr, err
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
	case MappingValueKindDynamic:
		return ctx.renderDynamic(plan, source)
	case MappingValueKindEmptyStruct:
		return ctx.renderEmptyStruct(plan)
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
	ctx.line("return " + ctx.errorReturn)
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
		if err := ctx.renderNullableFromValue(plan, tmp, source+"."+plan.Access.Oneof.Value.Getter+"()"); err != nil {
			return "", err
		}
		ctx.line("}")
	case MappingNullableFormValue:
		ctx.line("if " + source + " != nil && " + source + "." + plan.Access.Valid.Getter + "() {")
		if err := ctx.renderNullableFromValue(plan, tmp, source+"."+plan.Access.Value.Getter+"()"); err != nil {
			return "", err
		}
		ctx.line("}")
	default:
		ctx.line("if " + source + " != nil {")
		childSource := mappingChildSource(source, plan.Source, plan.Elem.Source)
		if err := ctx.renderNullableFromValue(plan, tmp, childSource); err != nil {
			return "", err
		}
		ctx.line("}")
	}

	return tmp, nil
}

func (ctx *mappingRenderContext) renderNullableFromValue(plan MappingValuePlan, target, source string) error {
	child, err := ctx.renderValue(*plan.Elem, source)
	if err != nil {
		return err
	}
	switch plan.Elem.Target.Kind {
	case TypeKindPointer:
		ctx.line(target + " = " + child)
	case TypeKindEmptyStruct:
		ctx.line(target + " = new(struct{})")
	default:
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

	if isEmptypbEmptyPointer(plan.Target) && plan.Elem != nil && plan.Elem.Kind == MappingValueKindEmptyStruct {
		ctx.line("var " + tmp + " " + targetType)
		ctx.line("if " + source + " != nil {")
		childSource := mappingChildSource(source, plan.Source, plan.Elem.Source)
		child, err := ctx.renderValue(*plan.Elem, childSource)
		if err != nil {
			return "", err
		}
		ctx.line(tmp + " = " + child)
		ctx.line("}")
		return tmp, nil
	}

	if plan.Target.Kind == TypeKindPointer && plan.Target.Elem != nil && plan.Target.Elem.Kind == TypeKindExternal {
		// Nullable shape wrappers can encode explicit null; ordinary pointer mappings only omit it.
		empty, err := newValueExpr(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		ctx.line("var " + tmp + " " + targetType)
		ctx.line("if " + source + " != nil {")
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
		ctx.line("} else {")
		switch plan.Access.NullableForm {
		case MappingNullableFormOneof:
			ctx.line(tmp + " = " + empty)
			ctx.line(tmp + "." + plan.Access.Oneof.Null.Setter + "(" + structpbNullValue(ctx.g) + ")")
		case MappingNullableFormValue:
			ctx.line(tmp + " = " + empty)
			ctx.line(tmp + "." + plan.Access.Valid.Setter + "(false)")
		}
		ctx.line("}")
		return tmp, nil
	}

	ctx.line("var " + tmp + " " + targetType)
	ctx.line("if " + source + " != nil {")
	childSource := mappingChildSource(source, plan.Source, plan.Elem.Source)
	child, err := ctx.renderValue(*plan.Elem, childSource)
	if err != nil {
		return "", err
	}
	ctx.line(tmp + " = " + child)
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
	item := ctx.tempPartName("item")
	ctx.line(tmp + " := make(" + targetType + ", 0, len(" + source + "))")
	ctx.line("for _, " + item + " := range " + source + " {")
	mapped, err := ctx.renderValue(elem, item)
	if err != nil {
		return "", err
	}
	ctx.line(tmp + " = append(" + tmp + ", " + mapped + ")")
	ctx.line("}")
	return tmp, nil
}

func (ctx *mappingRenderContext) renderShapeSlice(plan MappingValuePlan, source string) (string, error) {
	if plan.Target.Kind == TypeKindSlice {
		inner := ctx.tempPartName("sourceItems")
		sourceElemType, err := generateType(ctx.g, plan.Elem.Source)
		if err != nil {
			return "", fmt.Errorf("shape slice source element: %w", err)
		}
		ctx.line("var " + inner + " []" + sourceElemType)
		ctx.line("if " + source + " != nil {")
		ctx.line(inner + " = " + source + "." + plan.Access.Field.Getter + "()")
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

func (ctx *mappingRenderContext) renderDynamic(plan MappingValuePlan, source string) (string, error) {
	if plan.Dynamic == nil {
		return "", fmt.Errorf("dynamic mapping is missing metadata")
	}

	switch plan.Dynamic.Kind {
	case MappingDynamicKindStruct:
		return ctx.renderStructMap(plan, source)
	case MappingDynamicKindValue:
		return ctx.renderDynamicValue(plan, source)
	case MappingDynamicKindListValue:
		return ctx.renderDynamicList(plan, source)
	default:
		return "", fmt.Errorf("unsupported dynamic mapping")
	}
}

func (ctx *mappingRenderContext) renderStructMap(plan MappingValuePlan, source string) (string, error) {
	if isStructpbStructPointer(plan.Source) && isTegoStruct(plan.Target) {
		targetType, err := generateType(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		tmp := ctx.tempName("map")
		ctx.line("var " + tmp + " " + targetType)
		ctx.line("if " + source + " != nil {")
		ctx.line(tmp + " = " + source + ".AsMap()")
		ctx.line("}")
		return tmp, nil
	}

	if isTegoStruct(plan.Source) && isStructpbStructPointer(plan.Target) {
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
		ctx.line("var err error")
		ctx.line(tmp + ", err = " + structpbNewStruct(ctx.g) + "(" + source + ")")
		ctx.line("if err != nil {")
		ctx.line("return " + ctx.errorReturn)
		ctx.line("}")
		ctx.line("}")
		return tmp, nil
	}

	return "", fmt.Errorf("unsupported struct map mapping")
}

func (ctx *mappingRenderContext) renderDynamicValue(plan MappingValuePlan, source string) (string, error) {
	if isStructpbValuePointer(plan.Source) && isTegoValue(plan.Target) {
		targetType, err := generateType(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		tmp := ctx.tempName("value")
		ctx.line("var " + tmp + " " + targetType)
		ctx.line("if " + source + " != nil {")
		ctx.line(tmp + " = " + source + ".AsInterface()")
		ctx.line("}")
		return tmp, nil
	}

	if isTegoValue(plan.Source) && isStructpbValuePointer(plan.Target) {
		if !ctx.canError {
			return "", fmt.Errorf("dynamic value to proto mapping requires an erroring mapper")
		}
		tmp := ctx.tempName("value")
		ctx.line(tmp + ", err := " + structpbNewValue(ctx.g) + "(" + source + ")")
		ctx.line("if err != nil {")
		ctx.line("return " + ctx.errorReturn)
		ctx.line("}")
		return tmp, nil
	}

	return "", fmt.Errorf("unsupported dynamic value mapping")
}

func (ctx *mappingRenderContext) renderDynamicList(plan MappingValuePlan, source string) (string, error) {
	if isStructpbListValuePointer(plan.Source) && isTegoListValue(plan.Target) {
		targetType, err := generateType(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		tmp := ctx.tempName("list")
		ctx.line("var " + tmp + " " + targetType)
		ctx.line("if " + source + " != nil {")
		ctx.line(tmp + " = " + source + ".AsSlice()")
		ctx.line("}")
		return tmp, nil
	}

	if isTegoListValue(plan.Source) && isStructpbListValuePointer(plan.Target) {
		if !ctx.canError {
			return "", fmt.Errorf("dynamic list to proto mapping requires an erroring mapper")
		}
		targetType, err := generateType(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		tmp := ctx.tempName("list")
		ctx.line("var " + tmp + " " + targetType)
		ctx.line("if " + source + " != nil {")
		ctx.line("var err error")
		ctx.line(tmp + ", err = " + structpbNewList(ctx.g) + "(" + source + ")")
		ctx.line("if err != nil {")
		ctx.line("return " + ctx.errorReturn)
		ctx.line("}")
		ctx.line("}")
		return tmp, nil
	}

	return "", fmt.Errorf("unsupported dynamic list mapping")
}

func (ctx *mappingRenderContext) renderEmptyStruct(plan MappingValuePlan) (string, error) {
	if isEmptypbEmptyPointer(plan.Source) && plan.Target.Kind == TypeKindEmptyStruct {
		return "struct{}{}", nil
	}
	if plan.Source.Kind == TypeKindEmptyStruct && isEmptypbEmptyPointer(plan.Target) {
		empty, err := newValueExpr(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		return empty, nil
	}
	return "", fmt.Errorf("unsupported empty struct mapping")
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
	key := ctx.tempPartName("key")
	value := ctx.tempPartName("value")
	ctx.line(tmp + " := make(" + targetType + ", len(" + source + "))")
	ctx.line("for " + key + ", " + value + " := range " + source + " {")
	mappedKey, err := ctx.renderValue(keyPlan, key)
	if err != nil {
		return "", err
	}
	mappedValue, err := ctx.renderValue(valuePlan, value)
	if err != nil {
		return "", err
	}
	ctx.line(tmp + "[" + mappedKey + "] = " + mappedValue)
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
		entry := ctx.tempPartName("entry")
		ctx.line(tmp + " := make(" + targetType + ")")
		ctx.line("if " + source + " != nil {")
		ctx.line("for _, " + entry + " := range " + source + "." + plan.Access.Field.Getter + "() {")
		mappedKey, err := ctx.renderValue(*plan.Key, entry+"."+plan.Access.Key.Getter+"()")
		if err != nil {
			return "", err
		}
		mappedValue, err := ctx.renderValue(*plan.Value, entry+"."+plan.Access.Value.Getter+"()")
		if err != nil {
			return "", err
		}
		ctx.line(tmp + "[" + mappedKey + "] = " + mappedValue)
		ctx.line("}")
		ctx.line("}")
		return tmp, nil
	}

	entryType, err := generateType(ctx.g, plan.Access.ProtoElemType)
	if err != nil {
		return "", err
	}
	entries := ctx.tempPartName("entries")
	key := ctx.tempPartName("key")
	value := ctx.tempPartName("value")
	ctx.line(entries + " := make([]" + entryType + ", 0, len(" + source + "))")
	ctx.line("for " + key + ", " + value + " := range " + source + " {")
	entry := ctx.tempPartName("entry")
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

func structpbNewValue(g *protogen.GeneratedFile) string {
	return g.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: structpbImportPath,
		GoName:       "NewValue",
	})
}

func structpbNewList(g *protogen.GeneratedFile) string {
	return g.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: structpbImportPath,
		GoName:       "NewList",
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

type tempNameAllocator struct {
	used map[string]bool
	next map[string]int
}

func newTempNameAllocator(reserved ...string) *tempNameAllocator {
	allocator := &tempNameAllocator{
		used: make(map[string]bool, len(reserved)),
		next: make(map[string]int),
	}
	for _, name := range reserved {
		allocator.used[name] = true
	}
	return allocator
}

func (allocator *tempNameAllocator) name(base string) string {
	if base == "" {
		base = "tmp"
	}
	if !allocator.used[base] {
		allocator.used[base] = true
		return base
	}

	next := allocator.next[base]
	if next == 0 {
		next = 2
	}
	for {
		name := base + strconv.Itoa(next)
		next++
		if allocator.used[name] {
			continue
		}
		allocator.next[base] = next
		allocator.used[name] = true
		return name
	}
}

func tempIdentifierBase(name string) string {
	parts := casing.Split(name)
	if len(parts) == 0 {
		return "tmp"
	}

	var base strings.Builder
	first := goInitialism(strings.ToLower(parts[0]))
	base.WriteString(strings.ToLower(first))
	for _, part := range parts[1:] {
		base.WriteString(goName(part))
	}

	result := base.String()
	if result == "" {
		return "tmp"
	}
	if goKeyword[result] {
		return result + "Value"
	}
	return result
}

var goKeyword = map[string]bool{
	"break":       true,
	"default":     true,
	"func":        true,
	"interface":   true,
	"select":      true,
	"case":        true,
	"defer":       true,
	"go":          true,
	"map":         true,
	"struct":      true,
	"chan":        true,
	"else":        true,
	"goto":        true,
	"package":     true,
	"switch":      true,
	"const":       true,
	"fallthrough": true,
	"if":          true,
	"range":       true,
	"type":        true,
	"continue":    true,
	"for":         true,
	"import":      true,
	"return":      true,
	"var":         true,
}
