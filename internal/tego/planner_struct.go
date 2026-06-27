package tego

import (
	"strings"

	gotypes "go/types"

	"github.com/seeruk/tego/internal/types"
	"github.com/seeruk/tego/tegopb"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	errorTypeName = "error"

	durationFullName  = protoreflect.FullName("google.protobuf.Duration")
	timestampFullName = protoreflect.FullName("google.protobuf.Timestamp")
)

var wrapperScalarTypes = map[protoreflect.FullName]ScalarKind{
	"google.protobuf.BoolValue":   ScalarKindBool,
	"google.protobuf.Int32Value":  ScalarKindInt32,
	"google.protobuf.Int64Value":  ScalarKindInt64,
	"google.protobuf.UInt32Value": ScalarKindUint32,
	"google.protobuf.UInt64Value": ScalarKindUint64,
	"google.protobuf.FloatValue":  ScalarKindFloat32,
	"google.protobuf.DoubleValue": ScalarKindFloat64,
	"google.protobuf.StringValue": ScalarKindString,
	"google.protobuf.BytesValue":  ScalarKindBytes,
}

func (p *Planner) planStruct(message *ProtoMessage, si *ShapeIndex) (StructPlan, []Diagnostic, bool) {
	if message.Options.GetOmit() || message.Options.HasGoType() || isIndexedShape(message, si) {
		return StructPlan{}, nil, false
	}

	name := plannedMessageName(message)

	plan := StructPlan{
		ProtoName: message.FullName,
		Name:      name,
		Comment: plannedComment(
			message.Options.GetComment(),
			message.Options.HasComment(),
			messageLeadingComment(message),
			string(message.Name),
			name,
		),
	}

	var diagnostics []Diagnostic
	for _, field := range message.Fields {
		fieldPlan, fieldDiagnostics, ok := p.planField(field, si)
		diagnostics = append(diagnostics, fieldDiagnostics...)
		if ok {
			plan.Fields = append(plan.Fields, fieldPlan)
		}
	}

	return plan, diagnostics, true
}

func (p *Planner) planField(field *ProtoField, si *ShapeIndex) (FieldPlan, []Diagnostic, bool) {
	if field.Options.GetOmit() {
		return FieldPlan{}, nil, false
	}

	if field.Oneof != nil {
		return FieldPlan{}, []Diagnostic{fatalDiagnostic(
			string(field.FullName),
			"oneof field planning is not currently supported",
		)}, false
	}

	fieldType, diagnostics := p.planFieldType(field, si)

	name := plannedFieldName(field)

	plan := FieldPlan{
		ProtoName: field.FullName,
		Name:      name,
		Type:      fieldType,
		Comment: plannedComment(
			field.Options.GetComment(),
			field.Options.HasComment(),
			fieldLeadingComment(field),
			string(field.Name),
			name,
		),
	}

	tags, tagDiagnostics := structTags(field)
	plan.Tags = tags
	diagnostics = append(diagnostics, tagDiagnostics...)

	return plan, diagnostics, true
}

func (p *Planner) planFieldType(field *ProtoField, si *ShapeIndex) (TypePlan, []Diagnostic) {
	plan, diagnostics := p.planFieldBaseType(field, si)

	if field.Options.GetNullable() {
		plan = pointerType(plan)
	}

	if isOmittableField(field) {
		plan = TypePlan{
			Kind: TypeKindOmittable,
			Elem: new(plan),
		}
	}

	return plan, diagnostics
}

func (p *Planner) planFieldBaseType(field *ProtoField, si *ShapeIndex) (TypePlan, []Diagnostic) {
	if field.Options.HasGoType() {
		return p.planCustomGoType(field.Options.GetGoType(), sourcePatternForField(field), string(field.FullName))
	}

	if field.IsMap() {
		key, keyDiagnostics := p.planFieldBaseType(field.MapKey, si)
		value, valueDiagnostics := p.planFieldBaseType(field.MapValue, si)
		return TypePlan{
			Kind:  TypeKindMap,
			Key:   &key,
			Value: &value,
		}, append(keyDiagnostics, valueDiagnostics...)
	}

	if field.Cardinality == protoreflect.Repeated {
		elem, diagnostics := p.planSingularFieldType(field, si)
		return TypePlan{
			Kind: TypeKindSlice,
			Elem: &elem,
		}, diagnostics
	}

	return p.planSingularFieldType(field, si)
}

func (p *Planner) planSingularFieldType(field *ProtoField, si *ShapeIndex) (TypePlan, []Diagnostic) {
	switch field.Kind {
	case protoreflect.BoolKind:
		return scalarType(ScalarKindBool), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return scalarType(ScalarKindInt32), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return scalarType(ScalarKindInt64), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return scalarType(ScalarKindUint32), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return scalarType(ScalarKindUint64), nil
	case protoreflect.FloatKind:
		return scalarType(ScalarKindFloat32), nil
	case protoreflect.DoubleKind:
		return scalarType(ScalarKindFloat64), nil
	case protoreflect.StringKind:
		return scalarType(ScalarKindString), nil
	case protoreflect.BytesKind:
		return scalarType(ScalarKindBytes), nil
	case protoreflect.EnumKind:
		return p.planEnumType(field), nil
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return p.planMessageType(field, si)
	default:
		return TypePlan{}, []Diagnostic{fatalDiagnostic(string(field.FullName), "unsupported field kind %s", field.Kind)}
	}
}

func (p *Planner) planEnumType(field *ProtoField) TypePlan {
	if field.Enum == nil {
		return TypePlan{}
	}

	if field.Enum.File.Generate {
		return TypePlan{
			Kind: TypeKindEnum,
			Ref:  plannedEnumRef(field.Enum),
		}
	}

	return TypePlan{
		Kind: TypeKindExternal,
		Ref:  protoEnumRef(field.Enum),
	}
}

func (p *Planner) planMessageType(field *ProtoField, si *ShapeIndex) (TypePlan, []Diagnostic) {
	message := field.Message
	if message == nil {
		return TypePlan{}, []Diagnostic{fatalDiagnostic(string(field.FullName), "missing message descriptor")}
	}

	if message.Options.HasGoType() {
		return p.planCustomGoType(message.Options.GetGoType(), sourcePatternForMessage(message), string(field.FullName))
	}

	if scalar, ok := wrapperScalarTypes[message.FullName]; ok {
		plan := scalarType(scalar)
		return pointerType(plan), nil
	}

	switch message.FullName {
	case timestampFullName:
		return externalType("time", "Time"), nil
	case durationFullName:
		return externalType("time", "Duration"), nil
	}

	if _, ok := si.Nullables[message.FullName]; ok {
		inner, diagnostics := p.planNullableShape(message, si)
		return pointerType(inner), diagnostics
	}
	if _, ok := si.Slices[message.FullName]; ok {
		return p.planSliceShape(message, si)
	}
	if _, ok := si.Maps[message.FullName]; ok {
		return p.planMapShape(message, si)
	}

	if message.Parent != nil {
		return TypePlan{}, []Diagnostic{fatalDiagnostic(
			string(field.FullName),
			"nested message planning is not supported yet for %s",
			message.FullName,
		)}
	}

	if message.File.Generate {
		return TypePlan{
			Kind: TypeKindStruct,
			Ref:  plannedStructRef(message),
		}, nil
	}

	return TypePlan{
		Kind: TypeKindExternal,
		Ref:  protoMessageRef(message),
	}, nil
}

func (p *Planner) planNullableShape(message *ProtoMessage, si *ShapeIndex) (TypePlan, []Diagnostic) {
	for _, field := range message.Fields {
		if field.Enum != nil && field.Enum.FullName == "google.protobuf.NullValue" {
			continue
		}
		if field.Name == "valid" && field.Kind == protoreflect.BoolKind {
			continue
		}
		return p.planFieldBaseType(field, si)
	}

	return TypePlan{}, []Diagnostic{fatalDiagnostic(string(message.FullName), "nullable shape has no value field")}
}

func (p *Planner) planSliceShape(message *ProtoMessage, si *ShapeIndex) (TypePlan, []Diagnostic) {
	if len(message.Fields) != 1 {
		return TypePlan{}, []Diagnostic{fatalDiagnostic(string(message.FullName), "slice shape has no repeated field")}
	}
	return p.planFieldBaseType(message.Fields[0], si)
}

func (p *Planner) planMapShape(message *ProtoMessage, si *ShapeIndex) (TypePlan, []Diagnostic) {
	if len(message.Messages) != 1 {
		return TypePlan{}, []Diagnostic{fatalDiagnostic(string(message.FullName), "map shape has no nested Map message")}
	}

	key, value, ok := mapFields(message.Messages[0])
	if !ok {
		return TypePlan{}, []Diagnostic{fatalDiagnostic(string(message.FullName), "map shape has invalid Map fields")}
	}

	keyType, keyDiagnostics := p.planFieldBaseType(key, si)
	valueType, valueDiagnostics := p.planFieldBaseType(value, si)

	return TypePlan{
		Kind:  TypeKindMap,
		Key:   &keyType,
		Value: &valueType,
	}, append(keyDiagnostics, valueDiagnostics...)
}

func (p *Planner) planCustomGoType(goType *tegopb.GoType, source goTypePattern, diagnosticPath string) (TypePlan, []Diagnostic) {
	var diagnostics []Diagnostic

	if goType.GetRef() == "" {
		diagnostics = append(diagnostics, fatalDiagnostic(diagnosticPath, "go_type ref is required"))
	}
	if goType.GetFromProto() == "" {
		diagnostics = append(diagnostics, fatalDiagnostic(diagnosticPath, "go_type from_proto is required"))
	}
	if goType.GetToProto() == "" {
		diagnostics = append(diagnostics, fatalDiagnostic(diagnosticPath, "go_type to_proto is required"))
	}
	if len(diagnostics) > 0 {
		return TypePlan{}, diagnostics
	}

	customType, err := p.typeLoader.Type(goType.GetRef())
	if err != nil {
		return TypePlan{}, []Diagnostic{fatalDiagnostic(diagnosticPath, "couldn't resolve go_type ref: %s", err)}
	}

	customRef := GoTypeRef{
		ImportPath: customType.ImportPath,
		Name:       customType.Name,
	}
	customPattern := goTypePattern{named: &customRef}
	if goType.GetAsPointer() {
		customBase := customPattern
		customPattern = goTypePattern{pointer: &customBase}
	}

	fromProto, err := p.typeLoader.Function(goType.GetFromProto())
	if err != nil {
		diagnostics = append(diagnostics, fatalDiagnostic(diagnosticPath, "couldn't resolve go_type from_proto: %s", err))
	} else {
		diagnostics = append(diagnostics, validateFromProtoSignature(diagnosticPath, fromProto.Signature, source, customPattern)...)
	}

	toProtoRef, toProtoDiagnostics := p.resolveAndValidateToProto(diagnosticPath, goType.GetToProto(), source, customPattern)
	diagnostics = append(diagnostics, toProtoDiagnostics...)

	customPlan := CustomGoTypePlan{
		Ref: customRef,
		FromProto: GoSymbolRef{
			ImportPath: fromProtoImportPath(fromProto),
			Name:       fromProtoName(fromProto),
		},
		ToProto: toProtoRef,
	}
	plan := TypePlan{
		Kind:   TypeKindCustom,
		Ref:    customRef,
		Custom: customPlan,
	}
	if goType.GetAsPointer() {
		plan = pointerType(plan)
	}

	return plan, diagnostics
}

func (p *Planner) resolveAndValidateToProto(
	diagnosticPath string,
	ref string,
	source goTypePattern,
	custom goTypePattern,
) (GoSymbolRef, []Diagnostic) {
	if fn, err := p.typeLoader.Function(ref); err == nil {
		diagnostics := validateToProtoFunctionSignature(diagnosticPath, fn.Signature, custom, source)
		return GoSymbolRef{ImportPath: fn.ImportPath, Name: fn.Name}, diagnostics
	}

	method, err := p.typeLoader.Method(ref)
	if err != nil {
		return GoSymbolRef{}, []Diagnostic{fatalDiagnostic(diagnosticPath, "couldn't resolve go_type to_proto: %s", err)}
	}

	diagnostics := validateToProtoMethodSignature(diagnosticPath, method.Signature, custom, source)
	return GoSymbolRef{
		ImportPath: method.ImportPath,
		Receiver:   method.Receiver,
		Name:       method.Name,
	}, diagnostics
}

func validateFromProtoSignature(path string, sig *gotypes.Signature, source, custom goTypePattern) []Diagnostic {
	var diagnostics []Diagnostic
	if sig.Params().Len() != 1 {
		diagnostics = append(diagnostics, fatalDiagnostic(path, "go_type from_proto must accept exactly one parameter"))
	} else if !source.matches(sig.Params().At(0).Type()) {
		diagnostics = append(diagnostics, fatalDiagnostic(path, "go_type from_proto parameter has wrong type"))
	}

	validateResults := validateConversionResults(path, "from_proto", sig.Results(), custom)
	diagnostics = append(diagnostics, validateResults...)
	return diagnostics
}

func validateToProtoFunctionSignature(path string, sig *gotypes.Signature, custom, source goTypePattern) []Diagnostic {
	var diagnostics []Diagnostic
	if sig.Params().Len() != 1 {
		diagnostics = append(diagnostics, fatalDiagnostic(path, "go_type to_proto function must accept exactly one parameter"))
	} else if !custom.matches(sig.Params().At(0).Type()) {
		diagnostics = append(diagnostics, fatalDiagnostic(path, "go_type to_proto parameter has wrong type"))
	}

	validateResults := validateConversionResults(path, "to_proto", sig.Results(), source)
	diagnostics = append(diagnostics, validateResults...)
	return diagnostics
}

func validateToProtoMethodSignature(path string, sig *gotypes.Signature, custom, source goTypePattern) []Diagnostic {
	var diagnostics []Diagnostic
	if sig.Recv() == nil {
		diagnostics = append(diagnostics, fatalDiagnostic(path, "go_type to_proto method has no receiver"))
	} else if !custom.matches(sig.Recv().Type()) {
		diagnostics = append(diagnostics, fatalDiagnostic(path, "go_type to_proto receiver has wrong type"))
	}
	if sig.Params().Len() > 0 {
		diagnostics = append(diagnostics, fatalDiagnostic(path, "go_type to_proto method must not accept parameters"))
	}

	validateResults := validateConversionResults(path, "to_proto", sig.Results(), source)
	diagnostics = append(diagnostics, validateResults...)
	return diagnostics
}

func validateConversionResults(path, name string, results *gotypes.Tuple, expected goTypePattern) []Diagnostic {
	if results.Len() != 1 && results.Len() != 2 {
		return []Diagnostic{fatalDiagnostic(path, "go_type %s must return value or value and error", name)}
	}
	if !expected.matches(results.At(0).Type()) {
		return []Diagnostic{fatalDiagnostic(path, "go_type %s result has wrong type", name)}
	}
	if results.Len() == 2 && !isErrorType(results.At(1).Type()) {
		return []Diagnostic{fatalDiagnostic(path, "go_type %s second result must be error", name)}
	}
	return nil
}

func messageLeadingComment(message *ProtoMessage) protogen.Comments {
	if message.Desc == nil {
		return ""
	}
	return message.Desc.Comments.Leading
}

func fieldLeadingComment(field *ProtoField) protogen.Comments {
	if field.Desc == nil {
		return ""
	}
	return field.Desc.Comments.Leading
}

func structTags(field *ProtoField) ([]StructTagPlan, []Diagnostic) {
	var tags []StructTagPlan
	var hasJSONTag bool

	for _, tag := range field.Options.GetTags() {
		if tag.GetKey() == "json" {
			hasJSONTag = true
		}
		tags = append(tags, StructTagPlan{
			Key:   tag.GetKey(),
			Value: tag.GetValue(),
		})
	}

	if field.Options.HasJsonTag() {
		if hasJSONTag {
			return tags, []Diagnostic{fatalDiagnostic(string(field.FullName), "json_tag conflicts with explicit json struct tag")}
		}
		tags = append(tags, StructTagPlan{
			Key:   "json",
			Value: jsonStructTagValue(field.Options.GetJsonTag()),
		})
	}

	return tags, nil
}

func jsonStructTagValue(tag *tegopb.GoJsonStructTag) string {
	parts := []string{tag.GetValue()}
	if tag.GetOmitempty() {
		parts = append(parts, "omitempty")
	}
	if tag.GetOmitzero() {
		parts = append(parts, "omitzero")
	}
	return strings.Join(parts, ",")
}

func isOmittableField(field *ProtoField) bool {
	if field.IsRequired() {
		return false
	}
	if field.Options.HasOmittable() {
		return field.Options.GetOmittable()
	}
	return field.Parent != nil && field.Parent.Options.GetFieldsOmittable()
}

func isIndexedShape(message *ProtoMessage, si *ShapeIndex) bool {
	if message == nil || si == nil {
		return false
	}
	if _, ok := si.Nullables[message.FullName]; ok {
		return true
	}
	if _, ok := si.Slices[message.FullName]; ok {
		return true
	}
	if _, ok := si.Maps[message.FullName]; ok {
		return true
	}
	return false
}

func pointerType(plan TypePlan) TypePlan {
	if plan.Kind == TypeKindPointer {
		return plan
	}
	return TypePlan{
		Kind: TypeKindPointer,
		Elem: &plan,
	}
}

func scalarType(kind ScalarKind) TypePlan {
	return TypePlan{
		Kind:   TypeKindScalar,
		Scalar: kind,
	}
}

func externalType(importPath, name string) TypePlan {
	return TypePlan{
		Kind: TypeKindExternal,
		Ref: GoTypeRef{
			ImportPath: importPath,
			Name:       name,
		},
	}
}

func plannedEnumRef(enum *ProtoEnum) GoTypeRef {
	ref := protoEnumRef(enum)
	ref.Name = plannedEnumName(enum)
	return ref
}

func plannedStructRef(message *ProtoMessage) GoTypeRef {
	ref := protoMessageRef(message)
	ref.Name = plannedMessageName(message)
	return ref
}

func protoEnumRef(enum *ProtoEnum) GoTypeRef {
	return GoTypeRef{
		ImportPath: string(enum.Desc.GoIdent.GoImportPath),
		Name:       enum.Desc.GoIdent.GoName,
	}
}

func protoMessageRef(message *ProtoMessage) GoTypeRef {
	return GoTypeRef{
		ImportPath: string(message.Desc.GoIdent.GoImportPath),
		Name:       message.Desc.GoIdent.GoName,
	}
}

func fromProtoImportPath(function *types.Function) string {
	if function == nil {
		return ""
	}
	return function.ImportPath
}

func fromProtoName(function *types.Function) string {
	if function == nil {
		return ""
	}
	return function.Name
}

type goTypePattern struct {
	basic   gotypes.BasicKind
	named   *GoTypeRef
	pointer *goTypePattern
	slice   *goTypePattern
	mapKey  *goTypePattern
	mapVal  *goTypePattern
}

func (p goTypePattern) matches(typ gotypes.Type) bool {
	switch {
	case p.named != nil:
		named, ok := typ.(*gotypes.Named)
		if !ok {
			return false
		}
		obj := named.Obj()
		return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == p.named.ImportPath && obj.Name() == p.named.Name
	case p.pointer != nil:
		pointer, ok := typ.(*gotypes.Pointer)
		return ok && p.pointer.matches(pointer.Elem())
	case p.slice != nil:
		slice, ok := typ.(*gotypes.Slice)
		return ok && p.slice.matches(slice.Elem())
	case p.mapKey != nil && p.mapVal != nil:
		mapType, ok := typ.(*gotypes.Map)
		return ok && p.mapKey.matches(mapType.Key()) && p.mapVal.matches(mapType.Elem())
	default:
		basic, ok := typ.(*gotypes.Basic)
		return ok && basic.Kind() == p.basic
	}
}

func sourcePatternForField(field *ProtoField) goTypePattern {
	if field.IsMap() {
		return goTypePattern{
			mapKey: new(sourcePatternForField(field.MapKey)),
			mapVal: new(sourcePatternForField(field.MapValue)),
		}
	}
	if field.Cardinality == protoreflect.Repeated {
		return goTypePattern{slice: new(sourcePatternForSingularField(field))}
	}
	return sourcePatternForSingularField(field)
}

func sourcePatternForSingularField(field *ProtoField) goTypePattern {
	switch field.Kind {
	case protoreflect.BoolKind:
		return basicPattern(gotypes.Bool)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return basicPattern(gotypes.Int32)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return basicPattern(gotypes.Int64)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return basicPattern(gotypes.Uint32)
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return basicPattern(gotypes.Uint64)
	case protoreflect.FloatKind:
		return basicPattern(gotypes.Float32)
	case protoreflect.DoubleKind:
		return basicPattern(gotypes.Float64)
	case protoreflect.StringKind:
		return basicPattern(gotypes.String)
	case protoreflect.BytesKind:
		return goTypePattern{slice: new(basicPattern(gotypes.Byte))}
	case protoreflect.EnumKind:
		return goTypePattern{named: &GoTypeRef{
			ImportPath: string(field.Enum.Desc.GoIdent.GoImportPath),
			Name:       field.Enum.Desc.GoIdent.GoName,
		}}
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return sourcePatternForMessage(field.Message)
	default:
		return goTypePattern{}
	}
}

func sourcePatternForMessage(message *ProtoMessage) goTypePattern {
	ref := GoTypeRef{
		ImportPath: string(message.Desc.GoIdent.GoImportPath),
		Name:       message.Desc.GoIdent.GoName,
	}
	named := goTypePattern{named: &ref}
	return goTypePattern{pointer: &named}
}

func basicPattern(kind gotypes.BasicKind) goTypePattern {
	return goTypePattern{basic: kind}
}

func isErrorType(typ gotypes.Type) bool {
	errorType := gotypes.Universe.Lookup(errorTypeName).Type()
	return gotypes.Identical(typ, errorType)
}
