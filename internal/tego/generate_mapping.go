package tego

import (
	"fmt"
	"go/ast"
	"go/parser"
	"strconv"
	"strings"

	"github.com/danielgtaylor/casing"
	"google.golang.org/protobuf/compiler/protogen"
)

const (
	durationpbImportPath  = "google.golang.org/protobuf/types/known/durationpb"
	emptypbImportPath     = "google.golang.org/protobuf/types/known/emptypb"
	structpbImportPath    = "google.golang.org/protobuf/types/known/structpb"
	timestamppbImportPath = "google.golang.org/protobuf/types/known/timestamppb"
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
	builderType, err := builderTypeExpr(g, mapping.ToProto.Target)
	if err != nil {
		return fmt.Errorf("to proto builder target: %w", err)
	}

	ctx := newMappingRenderContext(g, mapping.ToProto.CanError, "nil, err")
	var fields []builderField
	for _, field := range mapping.Fields {
		fieldFields, err := generateToProtoBuilderField(ctx, field)
		if err != nil {
			return fmt.Errorf("to proto field %s: %w", field.ProtoName, err)
		}
		fields = append(fields, fieldFields...)
	}

	ctx.line("target := " + builderType + "{")
	for _, field := range fields {
		ctx.line(field.Name + ": " + field.Value + ",")
	}
	ctx.line("}.Build()")
	if mapping.ToProto.CanError {
		g.P("\treturn target, nil")
	} else {
		g.P("\treturn target")
	}
	g.P("}")
	g.P()
	return nil
}

type builderField struct {
	Name  string
	Value string
}

type renderedValue struct {
	Expr        string
	Addressable bool
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

func builderTypeExpr(g *protogen.GeneratedFile, plan TypePlan) (string, error) {
	if plan.Kind != TypeKindPointer || plan.Elem == nil {
		return "", fmt.Errorf("builder target must be a pointer type")
	}
	target, err := generateType(g, *plan.Elem)
	if err != nil {
		return "", err
	}
	return target + "_builder", nil
}

func builderFieldType(g *protogen.GeneratedFile, plan TypePlan) (string, error) {
	typ, err := generateType(g, plan)
	if err != nil {
		return "", err
	}
	if builderFieldNeedsPointer(plan) {
		return "*" + typ, nil
	}
	return typ, nil
}

func builderFieldValue(plan TypePlan, value renderedValue) string {
	if builderFieldNeedsPointer(plan) {
		return builderPointerValue(value)
	}
	return value.Expr
}

func builderPointerValue(value renderedValue) string {
	if value.Addressable {
		if dereferenced, ok := strings.CutPrefix(value.Expr, "*"); ok {
			return dereferenced
		}
		return "&" + value.Expr
	}
	return "new(" + value.Expr + ")"
}

func builderFieldNeedsPointer(plan TypePlan) bool {
	switch plan.Kind {
	case TypeKindEnum:
		return true
	case TypeKindScalar:
		return plan.Scalar != ScalarKindBytes
	default:
		return false
	}
}

func renderedAddressableValue(expr string) renderedValue {
	return renderedValue{
		Expr:        expr,
		Addressable: isAddressableExpr(expr),
	}
}

func renderedNonAddressableValue(expr string) renderedValue {
	return renderedValue{Expr: expr}
}

// Tego targets Go 1.26+, where range variables are per-iteration values, so
// generated identifiers like membersKey and aliasesItem can be safely addressed.
func isAddressableExpr(expr string) bool {
	parsed, err := parser.ParseExpr(expr)
	if err != nil {
		return false
	}
	return isAddressableASTExpr(parsed)
}

func isAddressableASTExpr(expr ast.Expr) bool {
	switch expr := expr.(type) {
	case *ast.Ident:
		if expr.Name == "true" || expr.Name == "false" || expr.Name == "nil" || expr.Name == "iota" {
			return false
		}
		return true
	case *ast.ParenExpr:
		return isAddressableASTExpr(expr.X)
	case *ast.SelectorExpr:
		return isAddressableASTExpr(expr.X)
	case *ast.StarExpr:
		return true
	default:
		return false
	}
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

func generateToProtoBuilderField(ctx *mappingRenderContext, field FieldMappingPlan) ([]builderField, error) {
	var fields []builderField
	err := ctx.withTempNameHint(field.Name, func() error {
		if field.ToProto.Kind == MappingValueKindOneof {
			var err error
			fields, err = generateToProtoOneofBuilderFields(ctx, field)
			return err
		}
		if field.ToProto.Kind == MappingValueKindOmittable {
			var err error
			fields, err = generateToProtoOmittableBuilderField(ctx, field)
			return err
		}
		if field.ToProto.Kind == MappingValueKindNullable && !isProtoPointer(field.ToProto.Target) {
			var err error
			fields, err = generateToProtoNullablePresenceBuilderField(ctx, field)
			return err
		}

		expr, err := ctx.renderBuilderValue(field.ToProto, "source."+field.Name)
		if err != nil {
			return err
		}
		value := builderFieldValue(field.ToProto.Target, expr)
		fields = []builderField{{Name: field.Proto.Name, Value: value}}
		return nil
	})
	return fields, err
}

func generateToProtoNullablePresenceBuilderField(ctx *mappingRenderContext, field FieldMappingPlan) ([]builderField, error) {
	if field.ToProto.Elem == nil {
		return nil, fmt.Errorf("nullable mapping is missing an element")
	}

	fieldType, err := builderFieldType(ctx.g, field.ToProto.Target)
	if err != nil {
		return nil, err
	}
	fieldValue := ctx.tempName(field.Proto.Name)
	ctx.line("var " + fieldValue + " " + fieldType)
	ctx.line("if source." + field.Name + " != nil {")
	childSource := mappingChildSource("source."+field.Name, field.ToProto.Source, field.ToProto.Elem.Source)
	expr, err := ctx.renderBuilderValue(*field.ToProto.Elem, childSource)
	if err != nil {
		return nil, err
	}
	value := builderFieldValue(field.ToProto.Target, expr)
	ctx.line(fieldValue + " = " + value)
	ctx.line("}")
	return []builderField{{Name: field.Proto.Name, Value: fieldValue}}, nil
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

func generateToProtoOneofBuilderFields(ctx *mappingRenderContext, field FieldMappingPlan) ([]builderField, error) {
	oneof := field.ToProto.Oneof
	if oneof == nil {
		return nil, fmt.Errorf("oneof mapping is missing metadata")
	}
	if !ctx.canError {
		return nil, fmt.Errorf("oneof to proto mapping requires an erroring mapper")
	}

	fields := make([]builderField, 0, len(oneof.Variants))
	var fieldVars []builderField
	for _, variant := range oneof.Variants {
		fieldType, err := builderFieldType(ctx.g, variant.Value.Target)
		if err != nil {
			return nil, fmt.Errorf("variant %s: %w", variant.ProtoName, err)
		}
		fieldVar := ctx.tempName(variant.Proto.Name)
		ctx.line("var " + fieldVar + " " + fieldType)
		fieldVars = append(fieldVars, builderField{Name: variant.Proto.Name, Value: fieldVar})
	}

	value := ctx.tempName("value")
	ctx.line("switch " + value + " := source." + field.Name + ".(type) {")
	ctx.line("case nil:")
	for index, variant := range oneof.Variants {
		if err := generateToProtoOneofBuilderVariant(ctx, variant.Name, value, variant, fieldVars[index].Value); err != nil {
			return nil, err
		}
		if err := generateToProtoOneofBuilderVariant(ctx, "*"+variant.Name, value, variant, fieldVars[index].Value); err != nil {
			return nil, err
		}
	}
	ctx.line("default:")
	ctx.line("return nil, " + errorsNew(ctx.g) + "(" + fmt.Sprintf("%q", "unsupported oneof implementation") + ")")
	ctx.line("}")
	fields = append(fields, fieldVars...)
	return fields, nil
}

func generateToProtoOneofBuilderVariant(
	ctx *mappingRenderContext,
	caseType string,
	value string,
	variant MappingOneofVariantPlan,
	target string,
) error {
	ctx.line("case " + caseType + ":")
	if strings.HasPrefix(caseType, "*") {
		ctx.line("if " + value + " != nil {")
	}
	expr, err := ctx.renderBuilderValueWithTempNameHint(variant.FieldName, variant.Value, value+"."+variant.FieldName)
	if err != nil {
		return fmt.Errorf("variant %s: %w", variant.ProtoName, err)
	}
	builderValue := builderFieldValue(variant.Value.Target, expr)
	ctx.line(target + " = " + builderValue)
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

	some := generateNamedType(ctx.g, GoTypeRef{ImportPath: omittableImportPath, Name: "Of"})
	none := generateNamedType(ctx.g, GoTypeRef{ImportPath: omittableImportPath, Name: "Empty"})

	presence := "source." + field.Proto.Has + "()"
	if field.Proto.Has == "" {
		presence = "source." + field.Proto.Getter + "() != nil"
	}
	ctx.line("if " + presence + " {")
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

func generateToProtoOmittableBuilderField(ctx *mappingRenderContext, field FieldMappingPlan) ([]builderField, error) {
	if field.ToProto.Elem == nil {
		return nil, fmt.Errorf("omittable mapping is missing an element")
	}

	fieldType, err := builderFieldType(ctx.g, field.ToProto.Target)
	if err != nil {
		return nil, err
	}
	fieldValue := ctx.tempName(field.Proto.Name)
	ctx.line("var " + fieldValue + " " + fieldType)
	ctx.line("if source." + field.Name + ".IsPresent() {")
	expr, err := ctx.renderBuilderValue(*field.ToProto.Elem, "source."+field.Name+".Get()")
	if err != nil {
		return nil, err
	}
	value := builderFieldValue(field.ToProto.Target, expr)
	ctx.line(fieldValue + " = " + value)
	ctx.line("}")
	return []builderField{{Name: field.Proto.Name, Value: fieldValue}}, nil
}

type mappingRenderContext struct {
	g           *protogen.GeneratedFile
	canError    bool
	errorReturn string
	errorLines  []string
	tempNames   *tempNameAllocator
	tempHint    string
}

func newMappingRenderContext(
	g *protogen.GeneratedFile,
	canError bool,
	errorReturn string,
	reserved ...string,
) *mappingRenderContext {
	reservedNames := append([]string{"source", "target", "err"}, reserved...)
	return &mappingRenderContext{
		g:           g,
		canError:    canError,
		errorReturn: errorReturn,
		tempNames:   newTempNameAllocator(reservedNames...),
	}
}

func newMappingRenderContextWithErrorLines(
	g *protogen.GeneratedFile,
	canError bool,
	errorLines []string,
	reserved ...string,
) *mappingRenderContext {
	ctx := newMappingRenderContext(g, canError, "", reserved...)
	ctx.errorLines = errorLines
	return ctx
}

func (ctx *mappingRenderContext) line(line string) {
	// protogen formats generated Go files when building the plugin response, so this
	// renderer only needs to preserve statement ordering and line breaks.
	ctx.g.P(line)
}

func (ctx *mappingRenderContext) emitErrorReturn() {
	if len(ctx.errorLines) > 0 {
		for _, line := range ctx.errorLines {
			ctx.line(line)
		}
		return
	}
	ctx.line("return " + ctx.errorReturn)
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

func (ctx *mappingRenderContext) renderValueWithTempNameHintSuffix(
	name string,
	plan MappingValuePlan,
	source string,
) (string, error) {
	hint := name
	if ctx.tempHint != "" {
		hint = ctx.tempHint + goName(name)
	}
	return ctx.renderValueWithTempNameHint(hint, plan, source)
}

func (ctx *mappingRenderContext) renderBuilderValueWithTempNameHint(
	name string,
	plan MappingValuePlan,
	source string,
) (renderedValue, error) {
	var value renderedValue
	err := ctx.withTempNameHint(name, func() error {
		var err error
		value, err = ctx.renderBuilderValue(plan, source)
		return err
	})
	return value, err
}

func (ctx *mappingRenderContext) renderBuilderValueWithTempNameHintSuffix(
	name string,
	plan MappingValuePlan,
	source string,
) (renderedValue, error) {
	hint := name
	if ctx.tempHint != "" {
		hint = ctx.tempHint + goName(name)
	}
	return ctx.renderBuilderValueWithTempNameHint(hint, plan, source)
}

func (ctx *mappingRenderContext) collectionItemName(source string) string {
	sourceBase, ok := tempIdentifierBaseFromIdentifier(source)
	if ok && (ctx.tempHint == sourceBase+"Tego" || ctx.tempHint == sourceBase+"Proto") {
		return ctx.tempNames.name(sourceBase + "Item")
	}
	return ctx.tempPartName("item")
}

func (ctx *mappingRenderContext) renderBuilderValue(plan MappingValuePlan, source string) (renderedValue, error) {
	switch plan.Kind {
	case MappingValueKindDirect:
		return renderedAddressableValue(source), nil
	case MappingValueKindScalarCast:
		target, err := scalarCastTargetType(ctx.g, plan)
		if err != nil {
			return renderedValue{}, err
		}
		return renderedNonAddressableValue(target + "(" + source + ")"), nil
	case MappingValueKindEnum:
		target, err := generateType(ctx.g, plan.Target)
		if err != nil {
			return renderedValue{}, err
		}
		return renderedNonAddressableValue(target + "(" + source + ")"), nil
	case MappingValueKindStruct:
		if plan.Struct == nil {
			return renderedValue{}, fmt.Errorf("struct mapping is missing a ref")
		}
		name := plan.Struct.Name
		if plan.Struct.Ref.Name != "" {
			name = generateSymbol(ctx.g, plan.Struct.Ref)
		}
		return ctx.renderBuilderCall(name, source, plan.CanError)
	case MappingValueKindCustom:
		if plan.Custom == nil {
			return renderedValue{}, fmt.Errorf("custom mapping is missing conversion refs")
		}
		return ctx.renderBuilderCustom(plan, source)
	case MappingValueKindOmittable:
		return renderedValue{}, fmt.Errorf("omittable mapping must be rendered at field level")
	default:
		expr, err := ctx.renderValue(plan, source)
		if err != nil {
			return renderedValue{}, err
		}
		return renderedAddressableValue(expr), nil
	}
}

func (ctx *mappingRenderContext) renderValue(plan MappingValuePlan, source string) (string, error) {
	switch plan.Kind {
	case MappingValueKindDirect:
		return source, nil
	case MappingValueKindScalarCast:
		target, err := scalarCastTargetType(ctx.g, plan)
		if err != nil {
			return "", err
		}
		return target + "(" + source + ")", nil
	case MappingValueKindEnum:
		target, err := generateType(ctx.g, plan.Target)
		if err != nil {
			return "", err
		}
		return target + "(" + source + ")", nil
	case MappingValueKindStruct:
		if plan.Struct == nil {
			return "", fmt.Errorf("struct mapping is missing a ref")
		}
		name := plan.Struct.Name
		if plan.Struct.Ref.Name != "" {
			name = generateSymbol(ctx.g, plan.Struct.Ref)
		}
		return ctx.renderCall(name, source, plan.CanError)
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
	case MappingValueKindWellKnown:
		return ctx.renderWellKnown(plan, source)
	case MappingValueKindEmptyStruct:
		return ctx.renderEmptyStruct(plan)
	case MappingValueKindFlatten:
		return ctx.renderFlatten(plan, source)
	case MappingValueKindOmittable:
		return "", fmt.Errorf("omittable mapping must be rendered at field level")
	default:
		return "", fmt.Errorf("unsupported mapping node")
	}
}

func scalarCastTargetType(g *protogen.GeneratedFile, plan MappingValuePlan) (string, error) {
	if plan.Cast != nil && plan.Cast.ProtoTarget {
		switch plan.Target.Scalar {
		case ScalarKindInt64, ScalarKindFixedInt64:
			return "int64", nil
		case ScalarKindUint64, ScalarKindFixedUint64:
			return "uint64", nil
		}
	}
	return generateType(g, plan.Target)
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

func (ctx *mappingRenderContext) renderBuilderCustom(plan MappingValuePlan, source string) (renderedValue, error) {
	ref := plan.Custom.FromProto
	if top, ok := topCustomType(plan.Source); ok && top.ToProto.Name != "" {
		ref = plan.Custom.ToProto
	}

	if ref.Receiver != "" {
		return ctx.renderBuilderCall(source+"."+ref.Name, "", plan.CanError)
	}
	name := generateSymbol(ctx.g, ref)
	return ctx.renderBuilderCall(name, source, plan.CanError)
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
	ctx.emitErrorReturn()
	ctx.line("}")
	return tmp, nil
}

func (ctx *mappingRenderContext) renderBuilderCall(name string, source string, canError bool) (renderedValue, error) {
	call := name + "(" + source + ")"
	if source == "" {
		call = name + "()"
	}
	if !canError {
		return renderedNonAddressableValue(call), nil
	}

	tmp := ctx.tempName("mapped")
	ctx.line(tmp + ", err := " + call)
	ctx.line("if err != nil {")
	ctx.emitErrorReturn()
	ctx.line("}")
	return renderedAddressableValue(tmp), nil
}

func (ctx *mappingRenderContext) renderNullable(plan MappingValuePlan, source string) (string, error) {
	if plan.Elem == nil {
		return "", fmt.Errorf("nullable mapping is missing an element")
	}
	if isProtoPointer(plan.Source) || isNullableFromConcreteProto(plan) {
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
		condition := source + " != nil"
		if plan.Source.Kind != TypeKindPointer {
			var ok bool
			condition, ok = nullablePresenceCondition(source, plan.Access.Field)
			if !ok {
				return "", fmt.Errorf("nullable scalar mapping is missing presence access")
			}
		}
		ctx.line("if " + condition + " {")
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
		var builder string
		if plan.Access.NullableForm == MappingNullableFormOneof || plan.Access.NullableForm == MappingNullableFormValue {
			var err error
			builder, err = builderTypeExpr(ctx.g, plan.Target)
			if err != nil {
				return "", err
			}
		}
		ctx.line("var " + tmp + " " + targetType)
		ctx.line("if " + source + " != nil {")
		childSource := mappingChildSource(source, plan.Source, plan.Elem.Source)
		child, err := ctx.renderBuilderValue(*plan.Elem, childSource)
		if err != nil {
			return "", err
		}
		switch plan.Access.NullableForm {
		case MappingNullableFormOneof:
			ctx.line(tmp + " = " + builder + "{")
			ctx.line(plan.Access.Oneof.Value.Name + ": " + builderFieldValue(plan.Elem.Target, child) + ",")
			ctx.line("}.Build()")
		case MappingNullableFormValue:
			ctx.line(tmp + " = " + builder + "{")
			ctx.line(plan.Access.Value.Name + ": " + builderFieldValue(plan.Elem.Target, child) + ",")
			ctx.line(plan.Access.Valid.Name + ": " + builderFieldValue(scalarType(ScalarKindBool), renderedNonAddressableValue("true")) + ",")
			ctx.line("}.Build()")
		default:
			ctx.line(tmp + " = " + child.Expr)
		}
		switch plan.Access.NullableForm {
		case MappingNullableFormOneof:
			ctx.line("} else {")
			ctx.line(tmp + " = " + builder + "{")
			nullType := TypePlan{Kind: TypeKindEnum, Ref: plan.Access.Oneof.NullRef}
			ctx.line(plan.Access.Oneof.Null.Name + ": " + builderFieldValue(nullType, renderedNonAddressableValue(structpbNullValue(ctx.g))) + ",")
			ctx.line("}.Build()")
			ctx.line("}")
		case MappingNullableFormValue:
			ctx.line("} else {")
			ctx.line(tmp + " = " + builder + "{")
			ctx.line(plan.Access.Valid.Name + ": " + builderFieldValue(scalarType(ScalarKindBool), renderedNonAddressableValue("false")) + ",")
			ctx.line("}.Build()")
			ctx.line("}")
		default:
			ctx.line("}")
		}
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
	index := ctx.tempPartName("index")
	item := ctx.collectionItemName(source)
	ctx.line(tmp + " := make(" + targetType + ", len(" + source + "))")
	if mappingConsumesSource(elem) {
		ctx.line("for " + index + ", " + item + " := range " + source + " {")
	} else {
		item = "_"
		ctx.line("for " + index + " := range " + source + " {")
	}
	mapped, err := ctx.renderValueWithTempNameHint(mappedCollectionPartName(item, elem), elem, item)
	if err != nil {
		return "", err
	}
	ctx.line(tmp + "[" + index + "] = " + mapped)
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
	builder, err := builderTypeExpr(ctx.g, plan.Target)
	if err != nil {
		return "", err
	}
	ctx.line(tmp + " := " + builder + "{")
	ctx.line(plan.Access.Field.Name + ": " + builderFieldValue(TypePlan{Kind: TypeKindSlice, Elem: &plan.Elem.Target}, renderedAddressableValue(inner)) + ",")
	ctx.line("}.Build()")
	return tmp, nil
}

func (ctx *mappingRenderContext) renderFlatten(plan MappingValuePlan, source string) (string, error) {
	if plan.Elem == nil {
		return "", fmt.Errorf("flatten mapping is missing an element")
	}
	if isProtoPointer(plan.Source) {
		return ctx.renderFlattenFromProto(plan, source)
	}
	return ctx.renderFlattenToProto(plan, source)
}

func (ctx *mappingRenderContext) renderFlattenFromProto(plan MappingValuePlan, source string) (string, error) {
	targetType, err := generateType(ctx.g, plan.Target)
	if err != nil {
		return "", err
	}

	tmp := ctx.tempName("value")
	ctx.line("var " + tmp + " " + targetType)
	ctx.line("if " + source + " != nil {")
	child, err := ctx.renderValueWithTempNameHintSuffix(plan.Access.Field.Name, *plan.Elem, source+"."+plan.Access.Field.Getter+"()")
	if err != nil {
		return "", err
	}
	ctx.line(tmp + " = " + child)
	ctx.line("}")
	return tmp, nil
}

func (ctx *mappingRenderContext) renderFlattenToProto(plan MappingValuePlan, source string) (string, error) {
	child, err := ctx.renderBuilderValueWithTempNameHintSuffix(plan.Access.Field.Name, *plan.Elem, source)
	if err != nil {
		return "", err
	}
	builder, err := builderTypeExpr(ctx.g, plan.Target)
	if err != nil {
		return "", err
	}
	tmp := ctx.tempName("value")
	ctx.line(tmp + " := " + builder + "{")
	ctx.line(plan.Access.Field.Name + ": " + builderFieldValue(plan.Elem.Target, child) + ",")
	ctx.line("}.Build()")
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
		mapped := ctx.tempName("mapped")
		ctx.line(mapped + ", err := " + structpbNewStruct(ctx.g) + "(" + source + ")")
		ctx.line("if err != nil {")
		ctx.emitErrorReturn()
		ctx.line("}")
		ctx.line(tmp + " = " + mapped)
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
		ctx.emitErrorReturn()
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
		mapped := ctx.tempName("mapped")
		ctx.line(mapped + ", err := " + structpbNewList(ctx.g) + "(" + source + ")")
		ctx.line("if err != nil {")
		ctx.emitErrorReturn()
		ctx.line("}")
		ctx.line(tmp + " = " + mapped)
		ctx.line("}")
		return tmp, nil
	}

	return "", fmt.Errorf("unsupported dynamic list mapping")
}

func (ctx *mappingRenderContext) renderWellKnown(plan MappingValuePlan, source string) (string, error) {
	if plan.WellKnown == nil {
		return "", fmt.Errorf("well-known mapping is missing metadata")
	}

	switch plan.WellKnown.Kind {
	case MappingWellKnownKindTimestamp:
		return ctx.renderTimestamp(plan, source)
	case MappingWellKnownKindDuration:
		return ctx.renderDuration(plan, source)
	default:
		return "", fmt.Errorf("unsupported well-known mapping")
	}
}

func (ctx *mappingRenderContext) renderTimestamp(plan MappingValuePlan, source string) (string, error) {
	if isTimestamppbTimestampPointer(plan.Source) && isTimeTime(plan.Target) {
		return ctx.renderWellKnownFromProto(plan.Target, source, "AsTime")
	}

	if isTimeTime(plan.Source) && isTimestamppbTimestampPointer(plan.Target) {
		return timestampNew(ctx.g) + "(" + source + ")", nil
	}

	return "", fmt.Errorf("unsupported timestamp mapping")
}

func (ctx *mappingRenderContext) renderDuration(plan MappingValuePlan, source string) (string, error) {
	if isDurationpbDurationPointer(plan.Source) && isTimeDuration(plan.Target) {
		return ctx.renderWellKnownFromProto(plan.Target, source, "AsDuration")
	}

	if isTimeDuration(plan.Source) && isDurationpbDurationPointer(plan.Target) {
		return durationNew(ctx.g) + "(" + source + ")", nil
	}

	return "", fmt.Errorf("unsupported duration mapping")
}

func (ctx *mappingRenderContext) renderWellKnownFromProto(plan TypePlan, source string, method string) (string, error) {
	targetType, err := generateType(ctx.g, plan)
	if err != nil {
		return "", err
	}
	tmp := ctx.tempName("wellKnown")
	ctx.line("var " + tmp + " " + targetType)
	ctx.line("if " + source + " != nil {")
	ctx.line(tmp + " = " + source + "." + method + "()")
	ctx.line("}")
	return tmp, nil
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

func mappedCollectionPartName(source string, plan MappingValuePlan) string {
	suffix := "Tego"
	if mappingTargetsProto(plan) {
		suffix = "Proto"
	}
	return source + suffix
}

func mappingTargetsProto(plan MappingValuePlan) bool {
	if plan.Kind == MappingValueKindCustom {
		if top, ok := topCustomType(plan.Source); ok && top.ToProto.Name != "" {
			return true
		}
		return false
	}
	return typeTargetsProto(plan.Target)
}

func typeTargetsProto(plan TypePlan) bool {
	if isProtoPointer(plan) {
		return true
	}
	switch plan.Kind {
	case TypeKindSlice:
		return plan.Elem != nil && typeTargetsProto(*plan.Elem)
	case TypeKindMap:
		return plan.Value != nil && typeTargetsProto(*plan.Value)
	default:
		return false
	}
}

func mappingConsumesSource(plan MappingValuePlan) bool {
	return plan.Kind != MappingValueKindEmptyStruct
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
	if mappingConsumesSource(valuePlan) {
		ctx.line("for " + key + ", " + value + " := range " + source + " {")
	} else {
		value = "_"
		ctx.line("for " + key + " := range " + source + " {")
	}
	mappedKey, err := ctx.renderValueWithTempNameHint(mappedCollectionPartName(key, keyPlan), keyPlan, key)
	if err != nil {
		return "", err
	}
	mappedValue, err := ctx.renderValueWithTempNameHint(mappedCollectionPartName(value, valuePlan), valuePlan, value)
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
		entries := ctx.tempPartName("entries")
		entry := ctx.tempPartName("entry")
		ctx.line("var " + tmp + " " + targetType)
		ctx.line("if " + source + " != nil {")
		ctx.line(entries + " := " + source + "." + plan.Access.Field.Getter + "()")
		ctx.line(tmp + " = make(" + targetType + ", len(" + entries + "))")
		ctx.line("for _, " + entry + " := range " + entries + " {")
		mappedKey, err := ctx.renderValueWithTempNameHintSuffix("Key", *plan.Key, entry+"."+plan.Access.Key.Getter+"()")
		if err != nil {
			return "", err
		}
		mappedValue, err := ctx.renderValueWithTempNameHintSuffix("Value", *plan.Value, entry+"."+plan.Access.Value.Getter+"()")
		if err != nil {
			return "", err
		}
		ctx.line(tmp + "[" + mappedKey + "] = " + mappedValue)
		ctx.line("}")
		ctx.line("} else {")
		ctx.line(tmp + " = make(" + targetType + ")")
		ctx.line("}")
		return tmp, nil
	}

	entryType, err := generateType(ctx.g, plan.Access.ProtoElemType)
	if err != nil {
		return "", err
	}
	entryBuilder, err := builderTypeExpr(ctx.g, plan.Access.ProtoElemType)
	if err != nil {
		return "", err
	}
	entries := ctx.tempPartName("entries")
	key := ctx.tempPartName("key")
	value := ctx.tempPartName("value")
	ctx.line(entries + " := make([]" + entryType + ", 0, len(" + source + "))")
	ctx.line("for " + key + ", " + value + " := range " + source + " {")
	mappedKey, err := ctx.renderBuilderValueWithTempNameHint(mappedCollectionPartName(key, *plan.Key), *plan.Key, key)
	if err != nil {
		return "", err
	}
	mappedValue, err := ctx.renderBuilderValueWithTempNameHint(mappedCollectionPartName(value, *plan.Value), *plan.Value, value)
	if err != nil {
		return "", err
	}
	entry := ctx.tempPartName("entry")
	ctx.line(entry + " := " + entryBuilder + "{")
	ctx.line(plan.Access.Key.Name + ": " + builderFieldValue(plan.Key.Target, mappedKey) + ",")
	ctx.line(plan.Access.Value.Name + ": " + builderFieldValue(plan.Value.Target, mappedValue) + ",")
	ctx.line("}.Build()")
	ctx.line(entries + " = append(" + entries + ", " + entry + ")")
	ctx.line("}")

	tmp := ctx.tempName("mapping")
	wrapper, err := builderTypeExpr(ctx.g, plan.Target)
	if err != nil {
		return "", err
	}
	ctx.line(tmp + " := " + wrapper + "{")
	ctx.line(plan.Access.Field.Name + ": " + builderFieldValue(TypePlan{Kind: TypeKindSlice, Elem: &plan.Access.ProtoElemType}, renderedAddressableValue(entries)) + ",")
	ctx.line("}.Build()")
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

func timestampNew(g *protogen.GeneratedFile) string {
	return g.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: timestamppbImportPath,
		GoName:       "New",
	})
}

func durationNew(g *protogen.GeneratedFile) string {
	return g.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: durationpbImportPath,
		GoName:       "New",
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

func isNullableFromConcreteProto(plan MappingValuePlan) bool {
	return plan.Source.Kind != TypeKindPointer && plan.Target.Kind == TypeKindPointer && !isProtoPointer(plan.Target)
}

func nullablePresenceCondition(source string, access MappingFieldAccessPlan) (string, bool) {
	if access.Getter == "" || access.Has == "" {
		return "", false
	}

	receiver, ok := strings.CutSuffix(source, "."+access.Getter+"()")
	if !ok {
		return "", false
	}
	return receiver + "." + access.Has + "()", true
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

func tempIdentifierBaseFromIdentifier(name string) (string, bool) {
	if !isASCIIIdentifier(name) {
		return "", false
	}
	return tempIdentifierBase(name), true
}

func isASCIIIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
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
