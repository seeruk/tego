package tego

import (
	"reflect"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type mappingDirection uint

const (
	mappingDirectionFromProto mappingDirection = iota
	mappingDirectionToProto
)

func (p *Planner) planMapping(message *ProtoMessage, structPlan StructPlan, si *ShapeIndex) MappingPlan {
	protoType := protoMessageType(message)
	structType := TypePlan{
		Kind: TypeKindStruct,
		Ref:  plannedStructRef(message),
	}

	plan := MappingPlan{
		ProtoName: message.FullName,
		Name:      structPlan.Name,
		ProtoRef:  protoMessagePlanRef(message),
		Type:      structType,
		FromProto: MappingFunctionPlan{
			Name:   structPlan.Name + "FromProto",
			Source: protoType,
			Target: structType,
		},
		ToProto: MappingFunctionPlan{
			Name:         structPlan.Name + "ToProto",
			ReceiverName: plannedReceiverName(structPlan.Name),
			Source:       structType,
			Target:       protoType,
		},
	}

	fields := protoFieldsByProtoName(message.Fields)
	oneofs := protoOneofsByProtoName(message.Oneofs)
	for _, fieldPlan := range structPlan.Fields {
		if oneof, ok := oneofs[fieldPlan.ProtoName]; ok {
			fromProto := p.planOneofMappingValue(oneof, protoType, fieldPlan.Type, si, mappingDirectionFromProto)
			toProto := p.planOneofMappingValue(oneof, fieldPlan.Type, protoType, si, mappingDirectionToProto)

			plan.FromProto.CanError = plan.FromProto.CanError || fromProto.CanError
			plan.ToProto.CanError = plan.ToProto.CanError || toProto.CanError
			plan.Fields = append(plan.Fields, FieldMappingPlan{
				ProtoName: fieldPlan.ProtoName,
				Name:      fieldPlan.Name,
				FromProto: fromProto,
				ToProto:   toProto,
			})
			continue
		}

		if field, ok := fields[fieldPlan.ProtoName]; ok {
			source := p.planProtoFieldType(field)
			fromProto := p.planFieldMappingValue(field, source, fieldPlan.Type, si, mappingDirectionFromProto)
			toProto := p.planFieldMappingValue(field, fieldPlan.Type, source, si, mappingDirectionToProto)

			plan.FromProto.CanError = plan.FromProto.CanError || fromProto.CanError
			plan.ToProto.CanError = plan.ToProto.CanError || toProto.CanError
			plan.Fields = append(plan.Fields, FieldMappingPlan{
				ProtoName: field.FullName,
				Name:      fieldPlan.Name,
				Proto:     mappingFieldAccess(field),
				FromProto: fromProto,
				ToProto:   toProto,
			})
		}
	}

	return plan
}

func protoFieldsByProtoName(fields []*ProtoField) map[protoreflect.FullName]*ProtoField {
	out := make(map[protoreflect.FullName]*ProtoField, len(fields))
	for _, field := range fields {
		out[field.FullName] = field
	}
	return out
}

func protoOneofsByProtoName(oneofs []*ProtoOneof) map[protoreflect.FullName]*ProtoOneof {
	out := make(map[protoreflect.FullName]*ProtoOneof, len(oneofs))
	for _, oneof := range oneofs {
		out[oneof.FullName] = oneof
	}
	return out
}

func protoMessageType(message *ProtoMessage) TypePlan {
	return pointerType(TypePlan{
		Kind: TypeKindExternal,
		Ref:  protoMessagePlanRef(message),
	})
}

func (p *Planner) planProtoFieldType(field *ProtoField) TypePlan {
	if field.IsMap() {
		return TypePlan{
			Kind:  TypeKindMap,
			Key:   new(p.planProtoFieldType(field.MapKey)),
			Value: new(p.planProtoFieldType(field.MapValue)),
		}
	}

	if field.Cardinality == protoreflect.Repeated {
		return TypePlan{
			Kind: TypeKindSlice,
			Elem: new(p.planProtoSingularFieldType(field)),
		}
	}

	return p.planProtoSingularFieldType(field)
}

func (p *Planner) planProtoSingularFieldType(field *ProtoField) TypePlan {
	switch field.Kind {
	case protoreflect.BoolKind:
		return scalarType(ScalarKindBool)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return scalarType(ScalarKindInt32)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return integerWidthScalarType(ScalarKindInt64, preserveIntegerWidth(field))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return scalarType(ScalarKindUint32)
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return integerWidthScalarType(ScalarKindUint64, preserveIntegerWidth(field))
	case protoreflect.FloatKind:
		return scalarType(ScalarKindFloat32)
	case protoreflect.DoubleKind:
		return scalarType(ScalarKindFloat64)
	case protoreflect.StringKind:
		return scalarType(ScalarKindString)
	case protoreflect.BytesKind:
		return scalarType(ScalarKindBytes)
	case protoreflect.EnumKind:
		return TypePlan{
			Kind: TypeKindEnum,
			Ref:  protoEnumPlanRef(field.Enum),
		}
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  protoMessagePlanRef(field.Message),
		})
	default:
		return TypePlan{}
	}
}

func (p *Planner) planMessageMappingValue(
	message *ProtoMessage,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) MappingValuePlan {
	if message == nil {
		return MappingValuePlan{Kind: MappingValueKindUnsupported, Source: source, Target: target}
	}
	if wrapped, ok := p.planShapeMessageMappingValue(message, source, target, si, direction); ok {
		return wrapped
	}
	return p.planMappingValue(source, target, direction)
}

func (p *Planner) planShapeMessageMappingValue(
	message *ProtoMessage,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) (MappingValuePlan, bool) {
	if si == nil {
		return MappingValuePlan{}, false
	}

	if si.Flattens[message.FullName] != nil {
		return p.planFlattenMessageMappingValue(message, source, target, si, direction)
	}
	if si.Nullables[message.FullName] != nil {
		return p.planNullableMessageMappingValue(message, source, target, si, direction)
	}
	if si.Slices[message.FullName] != nil {
		return p.planSliceMessageMappingValue(message, source, target, si, direction)
	}
	if si.Maps[message.FullName] != nil {
		return p.planMapMessageMappingValue(message, source, target, si, direction)
	}

	return MappingValuePlan{}, false
}

func (p *Planner) planFlattenMessageMappingValue(
	message *ProtoMessage,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) (MappingValuePlan, bool) {
	shapeField, ok := flattenShapeField(message)
	if !ok {
		return MappingValuePlan{}, false
	}

	var elemSource, elemTarget TypePlan
	if direction == mappingDirectionFromProto {
		elemSource = p.planProtoFieldType(shapeField)
		elemTarget = target
	} else {
		elemSource = source
		elemTarget = p.planProtoFieldType(shapeField)
	}

	elem := p.planFieldMappingValue(shapeField, elemSource, elemTarget, si, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindFlatten,
		Source:   source,
		Target:   target,
		CanError: elem.CanError,
		Access: MappingAccessPlan{
			Field: mappingFieldAccess(shapeField),
		},
		Elem: &elem,
	}, true
}

func (p *Planner) planNullableMessageMappingValue(
	message *ProtoMessage,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) (MappingValuePlan, bool) {
	inner := nullableShapeValueField(message)
	if inner == nil {
		return MappingValuePlan{}, false
	}

	innerType := p.planProtoFieldType(inner)
	var elemSource, elemTarget TypePlan
	if direction == mappingDirectionFromProto {
		elemSource = innerType
		elemTarget = pointerMappingElem(target, direction)
	} else {
		elemSource = pointerMappingElem(source, direction)
		elemTarget = innerType
	}

	elem := p.planFieldCoreMappingValue(inner, elemSource, elemTarget, si, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindNullable,
		Source:   source,
		Target:   target,
		CanError: elem.CanError,
		Access:   nullableShapeAccess(message, inner),
		Elem:     &elem,
	}, true
}

func (p *Planner) planSliceMessageMappingValue(
	message *ProtoMessage,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) (MappingValuePlan, bool) {
	if len(message.Fields) != 1 {
		return MappingValuePlan{}, false
	}

	shapeField := message.Fields[0]
	var elemSource, elemTarget TypePlan
	if direction == mappingDirectionFromProto {
		elemSource = p.planProtoSingularFieldType(shapeField)
		if target.Elem == nil {
			return MappingValuePlan{}, false
		}
		elemTarget = *target.Elem
	} else {
		if source.Elem == nil {
			return MappingValuePlan{}, false
		}
		elemSource = *source.Elem
		elemTarget = p.planProtoSingularFieldType(shapeField)
	}

	elem := p.planSingularFieldMappingValue(shapeField, elemSource, elemTarget, si, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindSlice,
		Source:   source,
		Target:   target,
		CanError: elem.CanError,
		Access: MappingAccessPlan{
			Field:         mappingFieldAccess(shapeField),
			ProtoType:     protoMessageType(message),
			ProtoElemType: elemSource,
		},
		Elem: &elem,
	}, true
}

func (p *Planner) planMapMessageMappingValue(
	message *ProtoMessage,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) (MappingValuePlan, bool) {
	if len(message.Fields) != 1 || len(message.Messages) != 1 {
		return MappingValuePlan{}, false
	}

	keyField, valueField, ok := mapFields(message.Messages[0])
	if !ok {
		return MappingValuePlan{}, false
	}

	var keySource, keyTarget, valueSource, valueTarget TypePlan
	if direction == mappingDirectionFromProto {
		if target.Key == nil || target.Value == nil {
			return MappingValuePlan{}, false
		}
		keySource = p.planProtoFieldType(keyField)
		keyTarget = *target.Key
		valueSource = p.planProtoFieldType(valueField)
		valueTarget = *target.Value
	} else {
		if source.Key == nil || source.Value == nil {
			return MappingValuePlan{}, false
		}
		keySource = *source.Key
		keyTarget = p.planProtoFieldType(keyField)
		valueSource = *source.Value
		valueTarget = p.planProtoFieldType(valueField)
	}

	key := p.planSingularFieldMappingValue(keyField, keySource, keyTarget, si, direction)
	value := p.planSingularFieldMappingValue(valueField, valueSource, valueTarget, si, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindMap,
		Source:   source,
		Target:   target,
		CanError: key.CanError || value.CanError,
		Access: MappingAccessPlan{
			Field:         mappingFieldAccess(message.Fields[0]),
			Key:           mappingFieldAccess(keyField),
			Value:         mappingFieldAccess(valueField),
			ProtoType:     protoMessageType(message),
			ProtoElemType: p.planProtoSingularFieldType(message.Fields[0]),
		},
		Key:   &key,
		Value: &value,
	}, true
}

func (p *Planner) planFieldMappingValue(
	field *ProtoField,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) MappingValuePlan {
	if wrapped, ok := p.planOmittableMappingValue(field, source, target, si, direction); ok {
		return wrapped
	}
	if wrapped, ok := p.planExplicitNullableMappingValue(field, source, target, si, direction); ok {
		return wrapped
	}

	return p.planFieldCoreMappingValue(field, source, target, si, direction)
}

func (p *Planner) planFieldCoreMappingValue(
	field *ProtoField,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) MappingValuePlan {
	if field.Options.HasGoType() {
		return p.planMappingValue(source, target, direction)
	}
	if field.IsMap() {
		return p.planMapFieldMappingValue(field, source, target, si, direction)
	}
	if field.Cardinality == protoreflect.Repeated {
		return p.planRepeatedFieldMappingValue(field, source, target, si, direction)
	}

	return p.planSingularFieldMappingValue(field, source, target, si, direction)
}

func (p *Planner) planRepeatedFieldMappingValue(
	field *ProtoField,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) MappingValuePlan {
	if source.Kind != TypeKindSlice || target.Kind != TypeKindSlice || source.Elem == nil || target.Elem == nil {
		return MappingValuePlan{Kind: MappingValueKindUnsupported, Source: source, Target: target}
	}

	elem := p.planSingularFieldMappingValue(field, *source.Elem, *target.Elem, si, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindSlice,
		Source:   source,
		Target:   target,
		CanError: elem.CanError,
		Elem:     &elem,
	}
}

func (p *Planner) planMapFieldMappingValue(
	field *ProtoField,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) MappingValuePlan {
	if source.Kind != TypeKindMap || target.Kind != TypeKindMap ||
		source.Key == nil || source.Value == nil || target.Key == nil || target.Value == nil ||
		field.MapKey == nil || field.MapValue == nil {
		return MappingValuePlan{Kind: MappingValueKindUnsupported, Source: source, Target: target}
	}

	key := p.planSingularFieldMappingValue(field.MapKey, *source.Key, *target.Key, si, direction)
	value := p.planSingularFieldMappingValue(field.MapValue, *source.Value, *target.Value, si, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindMap,
		Source:   source,
		Target:   target,
		CanError: key.CanError || value.CanError,
		Key:      &key,
		Value:    &value,
	}
}

func (p *Planner) planSingularFieldMappingValue(
	field *ProtoField,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) MappingValuePlan {
	if wrapped, ok := p.planImplicitNullableMappingValue(field, source, target, si, direction); ok {
		return wrapped
	}
	if wrapped, ok := p.planFlattenShapeMappingValue(field, source, target, si, direction); ok {
		return wrapped
	}
	if wrapped, ok := p.planSliceShapeMappingValue(field, source, target, si, direction); ok {
		return wrapped
	}
	if wrapped, ok := p.planMapShapeMappingValue(field, source, target, si, direction); ok {
		return wrapped
	}
	if field.Kind == protoreflect.MessageKind && field.Message != nil {
		return p.planMessageMappingValue(field.Message, source, target, si, direction)
	}

	return p.planMappingValue(source, target, direction)
}

func (p *Planner) planOneofMappingValue(
	oneof *ProtoOneof,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) MappingValuePlan {
	mapping := MappingOneofPlan{
		Which: mappingOneofWhich(oneof),
	}

	// ToProto is always marked as erroring, because new interface implementations may have been
	// introduced which produce invalid results in Protobuf, and we'd never know until runtime.
	canError := direction == mappingDirectionToProto

	for _, field := range oneof.Fields {
		if field.Options.GetOmit() {
			continue
		}

		variantType, _ := p.planOneofVariantType(field, si)
		protoType := p.planProtoFieldType(field)

		var value MappingValuePlan
		switch direction {
		case mappingDirectionFromProto:
			value = p.planFieldMappingValue(field, protoType, variantType, si, direction)
		case mappingDirectionToProto:
			value = p.planFieldMappingValue(field, variantType, protoType, si, direction)
		}

		canError = canError || value.CanError
		mapping.Variants = append(mapping.Variants, MappingOneofVariantPlan{
			ProtoName: field.FullName,
			Name:      plannedOneofVariantName(field),
			FieldName: plannedFieldName(field),
			Proto:     mappingFieldAccess(field),
			Case:      mappingCaseRef(field),
			Value:     value,
		})
	}

	return MappingValuePlan{
		Kind:     MappingValueKindOneof,
		Source:   source,
		Target:   target,
		CanError: canError,
		Oneof:    &mapping,
	}
}

func (p *Planner) planOmittableMappingValue(
	field *ProtoField,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) (MappingValuePlan, bool) {
	var elemSource, elemTarget TypePlan
	switch {
	case direction == mappingDirectionFromProto && target.Kind == TypeKindOmittable && target.Elem != nil:
		elemSource = source
		elemTarget = *target.Elem
	case direction == mappingDirectionToProto && source.Kind == TypeKindOmittable && source.Elem != nil:
		elemSource = *source.Elem
		elemTarget = target
	default:
		return MappingValuePlan{}, false
	}

	elem := p.planFieldMappingValue(field, elemSource, elemTarget, si, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindOmittable,
		Source:   source,
		Target:   target,
		CanError: elem.CanError,
		Access: MappingAccessPlan{
			Field: mappingFieldAccess(field),
		},
		Elem: &elem,
	}, true
}

func (p *Planner) planExplicitNullableMappingValue(
	field *ProtoField,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) (MappingValuePlan, bool) {
	if !field.Options.GetNullable() {
		return MappingValuePlan{}, false
	}

	elemSource, elemTarget := nullableMappingElemTypes(source, target, direction)
	elemField := field
	access := MappingAccessPlan{
		Field:        mappingFieldAccess(field),
		NullableForm: MappingNullableFormPointer,
		ProtoType:    source,
	}
	if field.Kind == protoreflect.MessageKind && isNullableShapeMessage(field.Message, si) {
		inner := nullableShapeValueField(field.Message)
		if inner != nil {
			innerType := p.planProtoFieldType(inner)
			if direction == mappingDirectionFromProto {
				elemSource = innerType
			} else {
				elemTarget = innerType
			}
			access = nullableShapeAccess(field.Message, inner)
			elemField = inner
		}
	}

	elem := p.planFieldCoreMappingValue(elemField, elemSource, elemTarget, si, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindNullable,
		Source:   source,
		Target:   target,
		CanError: elem.CanError,
		Access:   access,
		Elem:     &elem,
	}, true
}

func (p *Planner) planImplicitNullableMappingValue(
	field *ProtoField,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) (MappingValuePlan, bool) {
	if !isImplicitNullableMappingField(field, si) {
		return MappingValuePlan{}, false
	}

	elemSource, elemTarget := nullableMappingElemTypes(source, target, direction)
	access := MappingAccessPlan{
		Field:        mappingFieldAccess(field),
		NullableForm: MappingNullableFormPointer,
		ProtoType:    source,
	}

	var elem MappingValuePlan
	if field.Kind == protoreflect.MessageKind && isNullableShapeMessage(field.Message, si) {
		inner := nullableShapeValueField(field.Message)
		if inner == nil {
			return MappingValuePlan{}, false
		}
		innerType := p.planProtoFieldType(inner)
		if direction == mappingDirectionFromProto {
			elemSource = innerType
		} else {
			elemTarget = innerType
		}
		access = nullableShapeAccess(field.Message, inner)
		elem = p.planFieldCoreMappingValue(inner, elemSource, elemTarget, si, direction)
	} else {
		elem = p.planMappingValue(elemSource, elemTarget, direction)
	}

	return MappingValuePlan{
		Kind:     MappingValueKindNullable,
		Source:   source,
		Target:   target,
		CanError: elem.CanError,
		Access:   access,
		Elem:     &elem,
	}, true
}

func (p *Planner) planFlattenShapeMappingValue(
	field *ProtoField,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) (MappingValuePlan, bool) {
	if field.Kind != protoreflect.MessageKind || field.Message == nil || si == nil || si.Flattens[field.Message.FullName] == nil {
		return MappingValuePlan{}, false
	}
	shapeField, ok := flattenShapeField(field.Message)
	if !ok {
		return MappingValuePlan{}, false
	}

	var elemSource, elemTarget TypePlan
	if direction == mappingDirectionFromProto {
		elemSource = p.planProtoFieldType(shapeField)
		elemTarget = target
	} else {
		elemSource = source
		elemTarget = p.planProtoFieldType(shapeField)
	}

	elem := p.planFieldMappingValue(shapeField, elemSource, elemTarget, si, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindFlatten,
		Source:   source,
		Target:   target,
		CanError: elem.CanError,
		Access: MappingAccessPlan{
			Field: mappingFieldAccess(shapeField),
		},
		Elem: &elem,
	}, true
}

func (p *Planner) planSliceShapeMappingValue(
	field *ProtoField,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) (MappingValuePlan, bool) {
	if field.Kind != protoreflect.MessageKind || field.Message == nil || si == nil || si.Slices[field.Message.FullName] == nil {
		return MappingValuePlan{}, false
	}

	shapeField := field.Message.Fields[0]
	var elemSource, elemTarget TypePlan
	if direction == mappingDirectionFromProto {
		elemSource = p.planProtoSingularFieldType(shapeField)
		if target.Elem == nil {
			return MappingValuePlan{}, false
		}
		elemTarget = *target.Elem
	} else {
		if source.Elem == nil {
			return MappingValuePlan{}, false
		}
		elemSource = *source.Elem
		elemTarget = p.planProtoSingularFieldType(shapeField)
	}

	elem := p.planSingularFieldMappingValue(shapeField, elemSource, elemTarget, si, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindSlice,
		Source:   source,
		Target:   target,
		CanError: elem.CanError,
		Access: MappingAccessPlan{
			Field:         mappingFieldAccess(shapeField),
			ProtoType:     source,
			ProtoElemType: elemSource,
		},
		Elem: &elem,
	}, true
}

func (p *Planner) planMapShapeMappingValue(
	field *ProtoField,
	source TypePlan,
	target TypePlan,
	si *ShapeIndex,
	direction mappingDirection,
) (MappingValuePlan, bool) {
	if field.Kind != protoreflect.MessageKind || field.Message == nil || si == nil || si.Maps[field.Message.FullName] == nil {
		return MappingValuePlan{}, false
	}

	keyField, valueField, ok := mapFields(field.Message.Messages[0])
	if !ok {
		return MappingValuePlan{}, false
	}

	var keySource, keyTarget, valueSource, valueTarget TypePlan
	if direction == mappingDirectionFromProto {
		if target.Key == nil || target.Value == nil {
			return MappingValuePlan{}, false
		}
		keySource = p.planProtoFieldType(keyField)
		keyTarget = *target.Key
		valueSource = p.planProtoFieldType(valueField)
		valueTarget = *target.Value
	} else {
		if source.Key == nil || source.Value == nil {
			return MappingValuePlan{}, false
		}
		keySource = *source.Key
		keyTarget = p.planProtoFieldType(keyField)
		valueSource = *source.Value
		valueTarget = p.planProtoFieldType(valueField)
	}

	key := p.planSingularFieldMappingValue(keyField, keySource, keyTarget, si, direction)
	value := p.planSingularFieldMappingValue(valueField, valueSource, valueTarget, si, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindMap,
		Source:   source,
		Target:   target,
		CanError: key.CanError || value.CanError,
		Access: MappingAccessPlan{
			Field:         mappingFieldAccess(field.Message.Fields[0]),
			Key:           mappingFieldAccess(keyField),
			Value:         mappingFieldAccess(valueField),
			ProtoType:     source,
			ProtoElemType: p.planProtoSingularFieldType(field.Message.Fields[0]),
		},
		Key:   &key,
		Value: &value,
	}, true
}

func (p *Planner) planMappingValue(source TypePlan, target TypePlan, direction mappingDirection) MappingValuePlan {
	if custom, canError, ok := mappingCustomConversion(source, target, direction); ok {
		return MappingValuePlan{
			Kind:     MappingValueKindCustom,
			Source:   source,
			Target:   target,
			CanError: canError,
			Custom:   &custom,
		}
	}

	if source.Kind == TypeKindSlice && target.Kind == TypeKindSlice && source.Elem != nil && target.Elem != nil {
		elem := p.planMappingValue(*source.Elem, *target.Elem, direction)
		return MappingValuePlan{
			Kind:     MappingValueKindSlice,
			Source:   source,
			Target:   target,
			CanError: elem.CanError,
			Elem:     &elem,
		}
	}

	if source.Kind == TypeKindMap && target.Kind == TypeKindMap &&
		source.Key != nil && source.Value != nil && target.Key != nil && target.Value != nil {
		key := p.planMappingValue(*source.Key, *target.Key, direction)
		value := p.planMappingValue(*source.Value, *target.Value, direction)
		return MappingValuePlan{
			Kind:     MappingValueKindMap,
			Source:   source,
			Target:   target,
			CanError: key.CanError || value.CanError,
			Key:      &key,
			Value:    &value,
		}
	}

	if ref, ok := mappingStructRef(source, target, direction); ok {
		return MappingValuePlan{
			Kind:   MappingValueKindStruct,
			Source: source,
			Target: target,
			Struct: &ref,
		}
	}

	if structMap, ok := structpbStructMapMapping(source, target, direction); ok {
		return structMap
	}

	if emptyStruct, ok := emptypbEmptyStructMapping(source, target, direction); ok {
		return emptyStruct
	}

	if dynamicValue, ok := structpbValueMapping(source, target, direction); ok {
		return dynamicValue
	}

	if dynamicList, ok := structpbListValueMapping(source, target, direction); ok {
		return dynamicList
	}

	if wellKnown, ok := wellKnownTypeMapping(source, target, direction); ok {
		return wellKnown
	}

	if source.Kind == TypeKindEnum && target.Kind == TypeKindEnum {
		enum := MappingEnumPlan{Source: source, Target: target}
		return MappingValuePlan{
			Kind:   MappingValueKindEnum,
			Source: source,
			Target: target,
			Enum:   &enum,
		}
	}

	if source.Kind == TypeKindScalar && target.Kind == TypeKindScalar && needsScalarCast(source, target) {
		cast := MappingCastPlan{
			Source:      source,
			Target:      target,
			ProtoTarget: direction == mappingDirectionToProto,
		}
		return MappingValuePlan{
			Kind:   MappingValueKindScalarCast,
			Source: source,
			Target: target,
			Cast:   &cast,
		}
	}

	if reflect.DeepEqual(source, target) {
		return MappingValuePlan{
			Kind:   MappingValueKindDirect,
			Source: source,
			Target: target,
		}
	}

	if source.Kind == TypeKindPointer || target.Kind == TypeKindPointer {
		elemSource := pointerMappingElem(source, direction)
		elemTarget := pointerMappingElem(target, direction)
		if reflect.DeepEqual(elemSource, source) && reflect.DeepEqual(elemTarget, target) {
			return MappingValuePlan{
				Kind:   MappingValueKindUnsupported,
				Source: source,
				Target: target,
			}
		}
		elem := p.planMappingValue(elemSource, elemTarget, direction)
		return MappingValuePlan{
			Kind:     MappingValueKindNullable,
			Source:   source,
			Target:   target,
			CanError: elem.CanError,
			Elem:     &elem,
		}
	}

	return MappingValuePlan{
		Kind:   MappingValueKindUnsupported,
		Source: source,
		Target: target,
	}
}

func wellKnownTypeMapping(source TypePlan, target TypePlan, direction mappingDirection) (MappingValuePlan, bool) {
	switch {
	case isTimestampMapping(source, target, direction):
		return wellKnownMapping(source, target, MappingWellKnownKindTimestamp), true
	case isDurationMapping(source, target, direction):
		return wellKnownMapping(source, target, MappingWellKnownKindDuration), true
	default:
		return MappingValuePlan{}, false
	}
}

func wellKnownMapping(source TypePlan, target TypePlan, kind MappingWellKnownKind) MappingValuePlan {
	return MappingValuePlan{
		Kind:      MappingValueKindWellKnown,
		Source:    source,
		Target:    target,
		WellKnown: &MappingWellKnownPlan{Kind: kind},
	}
}

func isTimestampMapping(source TypePlan, target TypePlan, direction mappingDirection) bool {
	switch direction {
	case mappingDirectionFromProto:
		return isTimestamppbTimestampPointer(source) && isTimeTime(target)
	case mappingDirectionToProto:
		return isTimeTime(source) && isTimestamppbTimestampPointer(target)
	default:
		return false
	}
}

func isDurationMapping(source TypePlan, target TypePlan, direction mappingDirection) bool {
	switch direction {
	case mappingDirectionFromProto:
		return isDurationpbDurationPointer(source) && isTimeDuration(target)
	case mappingDirectionToProto:
		return isTimeDuration(source) && isDurationpbDurationPointer(target)
	default:
		return false
	}
}

func structpbStructMapMapping(source TypePlan, target TypePlan, direction mappingDirection) (MappingValuePlan, bool) {
	switch direction {
	case mappingDirectionFromProto:
		if !isStructpbStructPointer(source) || !isTegoStruct(target) {
			return MappingValuePlan{}, false
		}
		return MappingValuePlan{
			Kind:    MappingValueKindDynamic,
			Source:  source,
			Target:  target,
			Dynamic: &MappingDynamicPlan{Kind: MappingDynamicKindStruct},
		}, true
	case mappingDirectionToProto:
		if !isTegoStruct(source) || !isStructpbStructPointer(target) {
			return MappingValuePlan{}, false
		}
		return MappingValuePlan{
			Kind:     MappingValueKindDynamic,
			Source:   source,
			Target:   target,
			CanError: true,
			Dynamic:  &MappingDynamicPlan{Kind: MappingDynamicKindStruct},
		}, true
	default:
		return MappingValuePlan{}, false
	}
}

func emptypbEmptyStructMapping(source TypePlan, target TypePlan, direction mappingDirection) (MappingValuePlan, bool) {
	switch direction {
	case mappingDirectionFromProto:
		if !isEmptypbEmptyPointer(source) || target.Kind != TypeKindEmptyStruct {
			return MappingValuePlan{}, false
		}
		return MappingValuePlan{
			Kind:   MappingValueKindEmptyStruct,
			Source: source,
			Target: target,
		}, true
	case mappingDirectionToProto:
		if source.Kind != TypeKindEmptyStruct || !isEmptypbEmptyPointer(target) {
			return MappingValuePlan{}, false
		}
		return MappingValuePlan{
			Kind:   MappingValueKindEmptyStruct,
			Source: source,
			Target: target,
		}, true
	default:
		return MappingValuePlan{}, false
	}
}

func structpbValueMapping(source TypePlan, target TypePlan, direction mappingDirection) (MappingValuePlan, bool) {
	switch direction {
	case mappingDirectionFromProto:
		if !isStructpbValuePointer(source) || !isTegoValue(target) {
			return MappingValuePlan{}, false
		}
		return MappingValuePlan{
			Kind:    MappingValueKindDynamic,
			Source:  source,
			Target:  target,
			Dynamic: &MappingDynamicPlan{Kind: MappingDynamicKindValue},
		}, true
	case mappingDirectionToProto:
		if !isTegoValue(source) || !isStructpbValuePointer(target) {
			return MappingValuePlan{}, false
		}
		return MappingValuePlan{
			Kind:     MappingValueKindDynamic,
			Source:   source,
			Target:   target,
			CanError: true,
			Dynamic:  &MappingDynamicPlan{Kind: MappingDynamicKindValue},
		}, true
	default:
		return MappingValuePlan{}, false
	}
}

func structpbListValueMapping(source TypePlan, target TypePlan, direction mappingDirection) (MappingValuePlan, bool) {
	switch direction {
	case mappingDirectionFromProto:
		if !isStructpbListValuePointer(source) || !isTegoListValue(target) {
			return MappingValuePlan{}, false
		}
		return MappingValuePlan{
			Kind:    MappingValueKindDynamic,
			Source:  source,
			Target:  target,
			Dynamic: &MappingDynamicPlan{Kind: MappingDynamicKindListValue},
		}, true
	case mappingDirectionToProto:
		if !isTegoListValue(source) || !isStructpbListValuePointer(target) {
			return MappingValuePlan{}, false
		}
		return MappingValuePlan{
			Kind:     MappingValueKindDynamic,
			Source:   source,
			Target:   target,
			CanError: true,
			Dynamic:  &MappingDynamicPlan{Kind: MappingDynamicKindListValue},
		}, true
	default:
		return MappingValuePlan{}, false
	}
}

func mappingCustomConversion(source TypePlan, target TypePlan, direction mappingDirection) (CustomGoTypePlan, bool, bool) {
	switch direction {
	case mappingDirectionFromProto:
		custom, ok := topCustomType(target)
		return custom, custom.FromProtoCanError, ok
	case mappingDirectionToProto:
		custom, ok := topCustomType(source)
		return custom, custom.ToProtoCanError, ok
	default:
		return CustomGoTypePlan{}, false, false
	}
}

func topCustomType(plan TypePlan) (CustomGoTypePlan, bool) {
	if plan.Kind == TypeKindCustom {
		return plan.Custom, true
	}
	if plan.Kind == TypeKindPointer && plan.Elem != nil && plan.Elem.Kind == TypeKindCustom {
		return plan.Elem.Custom, true
	}
	return CustomGoTypePlan{}, false
}

func mappingStructRef(source TypePlan, target TypePlan, direction mappingDirection) (MappingRefPlan, bool) {
	var ref GoSymbolRef
	switch {
	case direction == mappingDirectionFromProto && source.Kind == TypeKindPointer &&
		target.Kind == TypeKindStruct && source.Elem != nil && source.Elem.Kind == TypeKindExternal:
		ref = GoSymbolRef{ImportPath: target.Ref.ImportPath, Name: target.Ref.Name + "FromProto"}
	case direction == mappingDirectionToProto && source.Kind == TypeKindStruct &&
		target.Kind == TypeKindPointer && target.Elem != nil && target.Elem.Kind == TypeKindExternal:
		ref = GoSymbolRef{ImportPath: source.Ref.ImportPath, Name: source.Ref.Name + "ToProto"}
	default:
		return MappingRefPlan{}, false
	}

	return MappingRefPlan{
		Name:   ref.Name,
		Ref:    ref,
		Source: source,
		Target: target,
	}, true
}

func propagateMappingErrors(plan *Plan) {
	for {
		refs := mappingErrorRefs(plan)
		var changed bool
		for fileIndex := range plan.Files {
			for mappingIndex := range plan.Files[fileIndex].Mappings {
				mapping := &plan.Files[fileIndex].Mappings[mappingIndex]
				changed = propagateMappingFunctionErrors(&mapping.FromProto, mapping.Fields, true, refs) || changed
				changed = propagateMappingFunctionErrors(&mapping.ToProto, mapping.Fields, false, refs) || changed
			}
			for serviceIndex := range plan.Files[fileIndex].Services {
				changed = propagateServiceErrors(&plan.Files[fileIndex].Services[serviceIndex], refs) || changed
			}
		}
		if !changed {
			return
		}
	}
}

func mappingErrorRefs(plan *Plan) map[string]bool {
	refs := make(map[string]bool)
	for _, file := range plan.Files {
		for _, mapping := range file.Mappings {
			refs[mapping.FromProto.Name] = mapping.FromProto.CanError
			refs[mapping.ToProto.Name] = mapping.ToProto.CanError
		}
	}
	return refs
}

func propagateServiceErrors(service *ServicePlan, refs map[string]bool) bool {
	var changed bool
	for methodIndex := range service.Methods {
		method := &service.Methods[methodIndex]
		changed = propagateServiceMessageErrors(&method.Request, refs) || changed
		changed = propagateServiceMessageErrors(&method.Response, refs) || changed
	}
	return changed
}

func propagateServiceMessageErrors(message *ServiceMessagePlan, refs map[string]bool) bool {
	changed := propagateMappingValueErrors(&message.FromProto, refs)
	changed = propagateMappingValueErrors(&message.ToProto, refs) || changed
	return changed
}

func propagateMappingFunctionErrors(function *MappingFunctionPlan, fields []FieldMappingPlan, fromProto bool, refs map[string]bool) bool {
	var changed bool
	for index := range fields {
		var value *MappingValuePlan
		if fromProto {
			value = &fields[index].FromProto
		} else {
			value = &fields[index].ToProto
		}
		changed = propagateMappingValueErrors(value, refs) || changed
		if value.CanError && !function.CanError {
			function.CanError = true
			changed = true
		}
	}
	return changed
}

func propagateMappingValueErrors(value *MappingValuePlan, refs map[string]bool) bool {
	var changed bool

	if value.Struct != nil && refs[value.Struct.Name] && !value.CanError {
		value.CanError = true
		changed = true
	}

	if value.Elem != nil {
		changed = propagateMappingValueErrors(value.Elem, refs) || changed
		if value.Elem.CanError && !value.CanError {
			value.CanError = true
			changed = true
		}
	}

	if value.Key != nil {
		changed = propagateMappingValueErrors(value.Key, refs) || changed
		if value.Key.CanError && !value.CanError {
			value.CanError = true
			changed = true
		}
	}

	if value.Value != nil {
		changed = propagateMappingValueErrors(value.Value, refs) || changed
		if value.Value.CanError && !value.CanError {
			value.CanError = true
			changed = true
		}
	}

	if value.Oneof != nil {
		for index := range value.Oneof.Variants {
			variant := &value.Oneof.Variants[index]
			changed = propagateMappingValueErrors(&variant.Value, refs) || changed
			if variant.Value.CanError && !value.CanError {
				value.CanError = true
				changed = true
			}
		}
	}

	return changed
}

func needsScalarCast(source TypePlan, target TypePlan) bool {
	if source.Scalar != target.Scalar {
		return true
	}
	return source.Scalar == ScalarKindInt64 || source.Scalar == ScalarKindUint64
}

func isStructpbStructPointer(plan TypePlan) bool {
	if plan.Kind != TypeKindPointer || plan.Elem == nil || plan.Elem.Kind != TypeKindExternal {
		return false
	}
	return plan.Elem.Ref.ImportPath == structpbImportPath && plan.Elem.Ref.Name == "Struct"
}

func isStructpbValuePointer(plan TypePlan) bool {
	if plan.Kind != TypeKindPointer || plan.Elem == nil || plan.Elem.Kind != TypeKindExternal {
		return false
	}
	return plan.Elem.Ref.ImportPath == structpbImportPath && plan.Elem.Ref.Name == "Value"
}

func isStructpbListValuePointer(plan TypePlan) bool {
	if plan.Kind != TypeKindPointer || plan.Elem == nil || plan.Elem.Kind != TypeKindExternal {
		return false
	}
	return plan.Elem.Ref.ImportPath == structpbImportPath && plan.Elem.Ref.Name == "ListValue"
}

func isTimestamppbTimestampPointer(plan TypePlan) bool {
	if plan.Kind != TypeKindPointer || plan.Elem == nil || plan.Elem.Kind != TypeKindExternal {
		return false
	}
	return plan.Elem.Ref.ImportPath == timestamppbImportPath && plan.Elem.Ref.Name == "Timestamp"
}

func isDurationpbDurationPointer(plan TypePlan) bool {
	if plan.Kind != TypeKindPointer || plan.Elem == nil || plan.Elem.Kind != TypeKindExternal {
		return false
	}
	return plan.Elem.Ref.ImportPath == durationpbImportPath && plan.Elem.Ref.Name == "Duration"
}

func isEmptypbEmptyPointer(plan TypePlan) bool {
	if plan.Kind != TypeKindPointer || plan.Elem == nil || plan.Elem.Kind != TypeKindExternal {
		return false
	}
	return plan.Elem.Ref.ImportPath == emptypbImportPath && plan.Elem.Ref.Name == "Empty"
}

func isTegoStruct(plan TypePlan) bool {
	return plan.Kind == TypeKindExternal && plan.Ref.ImportPath == tegoImportPath && plan.Ref.Name == "Struct"
}

func isTegoValue(plan TypePlan) bool {
	return plan.Kind == TypeKindExternal && plan.Ref.ImportPath == tegoImportPath && plan.Ref.Name == "Value"
}

func isTegoListValue(plan TypePlan) bool {
	return plan.Kind == TypeKindExternal && plan.Ref.ImportPath == tegoImportPath && plan.Ref.Name == "ListValue"
}

func isTimeTime(plan TypePlan) bool {
	return plan.Kind == TypeKindExternal && plan.Ref.ImportPath == "time" && plan.Ref.Name == "Time"
}

func isTimeDuration(plan TypePlan) bool {
	return plan.Kind == TypeKindExternal && plan.Ref.ImportPath == "time" && plan.Ref.Name == "Duration"
}

func nullableMappingElemTypes(source TypePlan, target TypePlan, direction mappingDirection) (TypePlan, TypePlan) {
	switch direction {
	case mappingDirectionFromProto:
		// FromProto fills a nullable target from a concrete proto value, so unwrap the target only.
		if target.Kind == TypeKindPointer && target.Elem != nil {
			return source, *target.Elem
		}
	case mappingDirectionToProto:
		// ToProto reads from a nullable source while keeping the proto target shape intact.
		if source.Kind == TypeKindPointer && source.Elem != nil {
			return *source.Elem, target
		}
	}
	return pointerMappingElem(source, direction), pointerMappingElem(target, direction)
}

func pointerMappingElem(plan TypePlan, direction mappingDirection) TypePlan {
	if plan.Kind != TypeKindPointer || plan.Elem == nil {
		return plan
	}
	if direction == mappingDirectionToProto && plan.Elem.Kind == TypeKindExternal {
		// A proto message pointer is the value ToProto must construct, not a wrapper to unwrap.
		return plan
	}
	return *plan.Elem
}

func isImplicitNullableMappingField(field *ProtoField, si *ShapeIndex) bool {
	if field.Kind != protoreflect.MessageKind || field.Message == nil {
		return false
	}
	if isNullableShapeMessage(field.Message, si) {
		return true
	}
	return false
}

func isNullableShapeMessage(message *ProtoMessage, si *ShapeIndex) bool {
	return message != nil && si != nil && si.Nullables[message.FullName] != nil
}

func nullableShapeAccess(message *ProtoMessage, inner *ProtoField) MappingAccessPlan {
	access := MappingAccessPlan{
		Field:        mappingFieldAccess(inner),
		NullableForm: MappingNullableFormValue,
		ProtoType: pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  protoMessagePlanRef(message),
		}),
		ProtoElemType: TypePlan{},
	}

	if isNullableOneofShape(message) && inner.Oneof != nil {
		access.NullableForm = MappingNullableFormOneof
		access.Oneof = MappingOneofAccessPlan{
			Name:     inner.Oneof.GoName,
			Which:    mappingOneofWhich(inner.Oneof),
			Value:    mappingFieldAccess(inner),
			ValueRef: mappingCaseRef(inner),
		}
		for _, field := range inner.Oneof.Fields {
			if field.Enum != nil && field.Enum.FullName == "google.protobuf.NullValue" {
				access.Oneof.Null = mappingFieldAccess(field)
				access.Oneof.NullRef = mappingCaseRef(field)
				break
			}
		}
		return access
	}

	for _, field := range message.Fields {
		switch field.Name {
		case "value":
			access.Value = mappingFieldAccess(field)
		case "valid":
			access.Valid = mappingFieldAccess(field)
		}
	}

	return access
}

func nullableShapeValueField(message *ProtoMessage) *ProtoField {
	for _, field := range message.Fields {
		if field.Name == "value" {
			return field
		}
		if field.Enum == nil || field.Enum.FullName != "google.protobuf.NullValue" {
			return field
		}
	}
	return nil
}

func protoEnumPlanRef(enum *ProtoEnum) GoTypeRef {
	if enum == nil {
		return GoTypeRef{}
	}
	if enum.Desc != nil {
		return protoEnumRef(enum)
	}
	return GoTypeRef{Name: enum.GoName}
}

func protoMessagePlanRef(message *ProtoMessage) GoTypeRef {
	if message == nil {
		return GoTypeRef{}
	}
	if message.Desc != nil {
		return protoMessageRef(message)
	}
	if ref, ok := wellKnownProtoMessagePlanRef(message.FullName); ok {
		return ref
	}
	return GoTypeRef{Name: message.GoName}
}

func wellKnownProtoMessagePlanRef(name protoreflect.FullName) (GoTypeRef, bool) {
	switch name {
	case timestampFullName:
		return GoTypeRef{ImportPath: timestamppbImportPath, Name: "Timestamp"}, true
	case durationFullName:
		return GoTypeRef{ImportPath: durationpbImportPath, Name: "Duration"}, true
	case emptyFullName:
		return GoTypeRef{ImportPath: emptypbImportPath, Name: "Empty"}, true
	case structFullName:
		return GoTypeRef{ImportPath: structpbImportPath, Name: "Struct"}, true
	case valueFullName:
		return GoTypeRef{ImportPath: structpbImportPath, Name: "Value"}, true
	case listValueFullName:
		return GoTypeRef{ImportPath: structpbImportPath, Name: "ListValue"}, true
	default:
		return GoTypeRef{}, false
	}
}

func protoFieldPlanName(field *ProtoField) string {
	if field.GoName != "" {
		return field.GoName
	}
	return plannedFieldName(field)
}

func mappingFieldAccess(field *ProtoField) MappingFieldAccessPlan {
	name := protoFieldPlanName(field)
	access := MappingFieldAccessPlan{
		Name:   name,
		Getter: "Get" + name,
		Setter: "Set" + name,
		Has:    "Has" + name,
		Clear:  "Clear" + name,
	}
	if field != nil && field.Desc != nil {
		if getter, _ := field.Desc.MethodName("Get"); getter != "" {
			access.Getter = getter
		}
		if setter, _ := field.Desc.MethodName("Set"); setter != "" {
			access.Setter = setter
		}
		if !field.HasPresence() {
			access.Has = ""
			access.Clear = ""
		} else {
			if has, _ := field.Desc.MethodName("Has"); has != "" {
				access.Has = has
			} else {
				access.Has = ""
			}
			if clear, _ := field.Desc.MethodName("Clear"); clear != "" {
				access.Clear = clear
			} else {
				access.Clear = ""
			}
		}
	}
	return access
}

func mappingOneofWhich(oneof *ProtoOneof) string {
	if oneof == nil {
		return ""
	}
	if oneof.Desc != nil {
		return oneof.Desc.MethodName("Which")
	}
	return "Which" + oneof.GoName
}

func mappingCaseRef(field *ProtoField) GoTypeRef {
	if field == nil {
		return GoTypeRef{}
	}
	if field.Desc != nil {
		return GoTypeRef{
			ImportPath: string(field.Desc.GoIdent.GoImportPath),
			Name:       field.Desc.GoIdent.GoName + "_case",
		}
	}
	if field.Parent != nil {
		return GoTypeRef{Name: field.Parent.GoName + "_" + protoFieldPlanName(field) + "_case"}
	}
	return GoTypeRef{Name: protoFieldPlanName(field) + "_case"}
}
