package tego

import (
	"fmt"
	"path"

	"google.golang.org/protobuf/compiler/protogen"
)

func (p *Planner) planService(service *ProtoService, si *ShapeIndex) (ServicePlan, []Diagnostic) {
	name := plannedServiceName(service)
	plan := ServicePlan{
		ProtoName:             service.FullName,
		ProtoRef:              protoServicePlanRef(service),
		ConnectRef:            connectServicePlanRef(service, p.rpc.ConnectPackageSuffix),
		Name:                  name,
		ClientName:            plannedServiceClientName(name),
		GRPCServerName:        plannedServiceGRPCServerName(name),
		GRPCClientName:        plannedServiceGRPCClientName(name),
		GRPCRegisterName:      plannedServiceGRPCRegisterName(name),
		GRPCNewClientName:     plannedServiceGRPCNewClientName(name),
		ConnectHandlerName:    plannedServiceConnectHandlerName(name),
		ConnectClientName:     plannedServiceConnectClientName(name),
		ConnectNewHandlerName: plannedServiceConnectNewHandlerName(name),
		ConnectNewClientName:  plannedServiceConnectNewClientName(name),
		Comment: plannedComment(
			"",
			false,
			serviceLeadingComment(service),
			string(service.Name),
			name,
		),
	}

	var diagnostics []Diagnostic
	for _, method := range service.Methods {
		methodPlan, methodDiagnostics := p.planServiceMethod(method, si)
		diagnostics = append(diagnostics, methodDiagnostics...)
		plan.Methods = append(plan.Methods, methodPlan)
	}

	return plan, diagnostics
}

func (p *Planner) planServiceMethod(method *ProtoMethod, si *ShapeIndex) (ServiceMethodPlan, []Diagnostic) {
	name := plannedMethodName(method)
	plan := ServiceMethodPlan{
		ProtoName:   method.FullName,
		ProtoGoName: method.GoName,
		Name:        name,
		Procedure:   serviceMethodProcedure(method),
		StreamType:  serviceMethodStreamType(method),
		Comment: plannedComment(
			"",
			false,
			methodLeadingComment(method),
			string(method.Name),
			name,
		),
	}

	request, requestDiagnostics := p.planServiceMessage(method.Input, si, string(method.FullName))
	response, responseDiagnostics := p.planServiceMessage(method.Output, si, string(method.FullName))
	plan.Request = request
	plan.Response = response

	return plan, append(requestDiagnostics, responseDiagnostics...)
}

func (p *Planner) planServiceMessage(
	message *ProtoMessage,
	si *ShapeIndex,
	diagnosticPath string,
) (ServiceMessagePlan, []Diagnostic) {
	if message == nil {
		return ServiceMessagePlan{}, []Diagnostic{fatalDiagnostic(diagnosticPath, "missing message descriptor")}
	}

	protoType := protoMessageType(message)
	nativeType, diagnostics := p.planMessageValueType(message, si, diagnosticPath)
	fromProto := p.planMessageMappingValue(message, protoType, nativeType, si, mappingDirectionFromProto)
	toProto := p.planMessageMappingValue(message, nativeType, protoType, si, mappingDirectionToProto)

	return ServiceMessagePlan{
		ProtoName: message.FullName,
		ProtoType: protoType,
		Type:      nativeType,
		FromProto: fromProto,
		ToProto:   toProto,
	}, diagnostics
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
	if wrapped, ok := p.planDirectShapeMappingValue(message, source, target, si, direction); ok {
		return wrapped
	}
	return p.planMappingValue(source, target, direction)
}

func (p *Planner) planDirectShapeMappingValue(
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
		return p.planDirectFlattenShapeMappingValue(message, source, target, si, direction)
	}
	if si.Nullables[message.FullName] != nil {
		return p.planDirectNullableShapeMappingValue(message, source, target, direction)
	}
	if si.Slices[message.FullName] != nil {
		return p.planDirectSliceShapeMappingValue(message, source, target, direction)
	}
	if si.Maps[message.FullName] != nil {
		return p.planDirectMapShapeMappingValue(message, source, target, direction)
	}

	return MappingValuePlan{}, false
}

func (p *Planner) planDirectFlattenShapeMappingValue(
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

func (p *Planner) planDirectNullableShapeMappingValue(
	message *ProtoMessage,
	source TypePlan,
	target TypePlan,
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

	elem := p.planMappingValue(elemSource, elemTarget, direction)

	return MappingValuePlan{
		Kind:     MappingValueKindNullable,
		Source:   source,
		Target:   target,
		CanError: elem.CanError,
		Access:   nullableShapeAccess(message, inner),
		Elem:     &elem,
	}, true
}

func (p *Planner) planDirectSliceShapeMappingValue(
	message *ProtoMessage,
	source TypePlan,
	target TypePlan,
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

	elem := p.planMappingValue(elemSource, elemTarget, direction)

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

func (p *Planner) planDirectMapShapeMappingValue(
	message *ProtoMessage,
	source TypePlan,
	target TypePlan,
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

	key := p.planMappingValue(keySource, keyTarget, direction)
	value := p.planMappingValue(valueSource, valueTarget, direction)

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

func protoMessageType(message *ProtoMessage) TypePlan {
	return pointerType(TypePlan{
		Kind: TypeKindExternal,
		Ref:  protoMessagePlanRef(message),
	})
}

func protoServicePlanRef(service *ProtoService) GoTypeRef {
	if service == nil {
		return GoTypeRef{}
	}
	if service.File != nil && service.File.Desc != nil {
		return GoTypeRef{
			ImportPath: string(service.File.Desc.GoImportPath),
			Name:       service.GoName,
		}
	}
	return GoTypeRef{Name: service.GoName}
}

func connectServicePlanRef(service *ProtoService, suffix string) GoTypeRef {
	ref := protoServicePlanRef(service)
	if ref.ImportPath == "" || suffix == "" {
		return ref
	}

	if service.File == nil || service.File.Desc == nil {
		return ref
	}

	return GoTypeRef{
		ImportPath: path.Join(ref.ImportPath, string(service.File.Desc.GoPackageName)+suffix),
		Name:       ref.Name,
	}
}

func serviceMethodProcedure(method *ProtoMethod) string {
	if method == nil || method.Parent == nil {
		return ""
	}
	return fmt.Sprintf("/%s/%s", method.Parent.FullName, method.Name)
}

func serviceMethodStreamType(method *ProtoMethod) ServiceStreamType {
	switch {
	case method.ClientStreaming && method.ServerStreaming:
		return ServiceStreamTypeBidiStreaming
	case method.ClientStreaming:
		return ServiceStreamTypeClientStreaming
	case method.ServerStreaming:
		return ServiceStreamTypeServerStreaming
	default:
		return ServiceStreamTypeUnary
	}
}

func serviceLeadingComment(service *ProtoService) protogen.Comments {
	if service.Desc == nil {
		return ""
	}
	return service.Desc.Comments.Leading
}

func methodLeadingComment(method *ProtoMethod) protogen.Comments {
	if method.Desc == nil {
		return ""
	}
	return method.Desc.Comments.Leading
}
