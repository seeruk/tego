package tego

import (
	"fmt"
	"path"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (p *Planner) planService(
	service *ProtoService,
	si *ShapeIndex,
	structsByProtoName map[protoreflect.FullName]StructPlan,
) (ServicePlan, []Diagnostic) {
	name := plannedServiceName(service)
	comment := ""
	hasComment := false
	if service.Options != nil {
		comment = service.Options.GetComment()
		hasComment = service.Options.HasComment()
	}
	plan := ServicePlan{
		ProtoName:             service.FullName,
		ProtoRef:              protoServicePlanRef(service),
		ConnectRef:            connectServicePlanRef(service, p.rpc.ConnectPackageSuffix),
		Name:                  name,
		UnimplementedName:     plannedServiceUnimplementedName(name),
		GRPCServerName:        plannedServiceGRPCServerName(name),
		GRPCAdapterName:       plannedServiceGRPCAdapterName(name),
		GRPCClientName:        plannedServiceGRPCClientName(name),
		GRPCRegisterName:      plannedServiceGRPCRegisterName(name),
		GRPCNewServerName:     plannedServiceGRPCNewServerName(name),
		GRPCNewClientName:     plannedServiceGRPCNewClientName(name),
		ConnectHandlerName:    plannedServiceConnectHandlerName(name),
		ConnectAdapterName:    plannedServiceConnectAdapterName(name),
		ConnectClientName:     plannedServiceConnectClientName(name),
		ConnectNewHandlerName: plannedServiceConnectNewHandlerName(name),
		ConnectNewClientName:  plannedServiceConnectNewClientName(name),
		Comment: plannedComment(
			comment,
			hasComment,
			serviceLeadingComment(service),
			string(service.Name),
			name,
		),
	}

	var diagnostics []Diagnostic
	for _, method := range service.Methods {
		methodPlan, methodDiagnostics := p.planServiceMethod(service, method, si, structsByProtoName)
		diagnostics = append(diagnostics, methodDiagnostics...)
		plan.Methods = append(plan.Methods, methodPlan)
	}

	return plan, diagnostics
}

func (p *Planner) planServiceMethod(
	service *ProtoService,
	method *ProtoMethod,
	si *ShapeIndex,
	structsByProtoName map[protoreflect.FullName]StructPlan,
) (ServiceMethodPlan, []Diagnostic) {
	name := plannedMethodName(method)
	comment := ""
	hasComment := false
	if method.Options != nil {
		comment = method.Options.GetComment()
		hasComment = method.Options.HasComment()
	}
	plan := ServiceMethodPlan{
		ProtoName:   method.FullName,
		ProtoGoName: method.GoName,
		Name:        name,
		Procedure:   serviceMethodProcedure(method),
		StreamType:  serviceMethodStreamType(method),
		Comment: plannedComment(
			comment,
			hasComment,
			methodLeadingComment(method),
			string(method.Name),
			name,
		),
	}

	request, requestDiagnostics := p.planServiceMessage(method.Input, si, string(method.FullName))
	response, responseDiagnostics := p.planServiceMessage(method.Output, si, string(method.FullName))
	plan.Request = request
	plan.Response = response

	diagnostics := append(requestDiagnostics, responseDiagnostics...)

	inlining := planServiceInlining(service, method)
	if inlining.Request.Enabled {
		inline, inlineDiagnostics := p.planServiceInlineRequest(
			method.Input,
			request,
			inlining.Request.Explicit,
			plan.StreamType,
			structsByProtoName,
			string(method.FullName),
		)
		diagnostics = append(diagnostics, inlineDiagnostics...)
		if inline != nil {
			plan.InlineRequest = inline
		}
	}

	if inlining.Response.Enabled {
		inline, inlineDiagnostics := p.planServiceInlineResponse(
			method.Output,
			response,
			inlining.Response.Explicit,
			plan.StreamType,
			structsByProtoName,
			string(method.FullName),
		)
		diagnostics = append(diagnostics, inlineDiagnostics...)
		if inline != nil {
			plan.InlineResponse = inline
		}
	}

	return plan, diagnostics
}

// serviceInliningPlan describes a method's resolved inline options after merging the
// service-level option with any method-level overrides.
type serviceInliningPlan struct {
	Request  serviceInliningDecision
	Response serviceInliningDecision
}

// serviceInliningDecision is the effective inline decision for one side of a method.
// Explicit distinguishes user-requested inlining from service-default inlining, because explicit
// invalid inlining is fatal while automatic inlining silently skips sides that cannot be inlined.
type serviceInliningDecision struct {
	Enabled  bool
	Explicit bool
}

// planServiceInlining resolves service defaults and method-level overrides into the
// request/response inline decisions used by the planner. Method inline applies to both sides first;
// side-specific method options then override just their side.
func planServiceInlining(service *ProtoService, method *ProtoMethod) serviceInliningPlan {
	inlineByDefault := serviceInlineByDefault(service)

	plan := serviceInliningPlan{
		Request:  serviceInliningDecision{Enabled: inlineByDefault},
		Response: serviceInliningDecision{Enabled: inlineByDefault},
	}

	options := method.Options
	if options == nil {
		return plan
	}
	if options.HasInline() {
		inline := options.GetInline()
		plan.Request = serviceInliningDecision{Enabled: inline, Explicit: true}
		plan.Response = serviceInliningDecision{Enabled: inline, Explicit: true}
	}
	if options.HasInlineRequest() {
		plan.Request = serviceInliningDecision{Enabled: options.GetInlineRequest(), Explicit: true}
	}
	if options.HasInlineResponse() {
		plan.Response = serviceInliningDecision{Enabled: options.GetInlineResponse(), Explicit: true}
	}

	return plan
}

func serviceInlineByDefault(service *ProtoService) bool {
	if service == nil || service.Options == nil {
		return true
	}
	return service.Options.GetInlineByDefault()
}

func (p *Planner) planServiceInlineRequest(
	message *ProtoMessage,
	serviceMessage ServiceMessagePlan,
	explicit bool,
	streamType ServiceStreamType,
	structsByProtoName map[protoreflect.FullName]StructPlan,
	diagnosticPath string,
) (*ServiceInlineHelperPlan, []Diagnostic) {
	return p.planServiceInlineSide(
		message,
		serviceMessage,
		"request",
		serviceInlineRequestSupported(streamType),
		explicit,
		streamType,
		structsByProtoName,
		diagnosticPath,
	)
}

func (p *Planner) planServiceInlineResponse(
	message *ProtoMessage,
	serviceMessage ServiceMessagePlan,
	explicit bool,
	streamType ServiceStreamType,
	structsByProtoName map[protoreflect.FullName]StructPlan,
	diagnosticPath string,
) (*ServiceInlineHelperPlan, []Diagnostic) {
	return p.planServiceInlineSide(
		message,
		serviceMessage,
		"response",
		serviceInlineResponseSupported(streamType),
		explicit,
		streamType,
		structsByProtoName,
		diagnosticPath,
	)
}

// planServiceInlineSide plans a helper for one inlineable side. Default inlining is best-effort, so
// unsupported stream sides or non-inlineable message shapes are skipped unless the method explicitly
// requested that side.
func (p *Planner) planServiceInlineSide(
	message *ProtoMessage,
	serviceMessage ServiceMessagePlan,
	side string,
	supportedForStream bool,
	explicit bool,
	streamType ServiceStreamType,
	structsByProtoName map[protoreflect.FullName]StructPlan,
	diagnosticPath string,
) (*ServiceInlineHelperPlan, []Diagnostic) {
	if !supportedForStream {
		if explicit {
			return nil, []Diagnostic{fatalDiagnostic(
				diagnosticPath,
				"facade inline %s is not supported on %s methods",
				side,
				serviceStreamTypeName(streamType),
			)}
		}
		return nil, nil
	}

	inline, diagnostics := p.planServiceInlineHelper(
		message,
		serviceMessage,
		structsByProtoName,
		diagnosticPath,
	)
	if len(diagnostics) > 0 {
		if explicit {
			return nil, diagnostics
		}
		return nil, nil
	}
	return &inline, nil
}

func serviceInlineRequestSupported(streamType ServiceStreamType) bool {
	return streamType == ServiceStreamTypeUnary || streamType == ServiceStreamTypeServerStreaming
}

func serviceInlineResponseSupported(streamType ServiceStreamType) bool {
	return streamType == ServiceStreamTypeUnary || streamType == ServiceStreamTypeClientStreaming
}

func serviceStreamTypeName(streamType ServiceStreamType) string {
	switch streamType {
	case ServiceStreamTypeUnary:
		return "unary"
	case ServiceStreamTypeClientStreaming:
		return "client-streaming"
	case ServiceStreamTypeServerStreaming:
		return "server-streaming"
	case ServiceStreamTypeBidiStreaming:
		return "bidi-streaming"
	default:
		return fmt.Sprintf("unknown stream type %d", streamType)
	}
}

func (p *Planner) planServiceInlineHelper(
	message *ProtoMessage,
	serviceMessage ServiceMessagePlan,
	structsByProtoName map[protoreflect.FullName]StructPlan,
	diagnosticPath string,
) (ServiceInlineHelperPlan, []Diagnostic) {
	if serviceMessage.Type.Kind != TypeKindStruct {
		return ServiceInlineHelperPlan{}, []Diagnostic{fatalDiagnostic(
			diagnosticPath,
			"facade inline requires an ordinary generated struct-shaped message",
		)}
	}

	structure, ok := structsByProtoName[message.FullName]
	if !ok {
		return ServiceInlineHelperPlan{}, []Diagnostic{fatalDiagnostic(
			diagnosticPath,
			"facade inline requires an ordinary generated struct-shaped message",
		)}
	}
	if len(structure.Fields) == 0 {
		return ServiceInlineHelperPlan{}, []Diagnostic{fatalDiagnostic(
			diagnosticPath,
			"facade inline requires a message with at least one generated field",
		)}
	}

	names := newTempNameAllocator("ctx", "err")
	fields := make([]ServiceInlineFieldPlan, 0, len(structure.Fields))
	for _, field := range structure.Fields {
		fields = append(fields, ServiceInlineFieldPlan{
			Name:      names.name(tempIdentifierBase(field.Name)),
			FieldName: field.Name,
			Type:      field.Type,
		})
	}

	return ServiceInlineHelperPlan{
		ProtoName:      message.FullName,
		Type:           serviceMessage.Type,
		ToInlineName:   plannedServiceInlineToName(structure.Name),
		FromInlineName: plannedServiceInlineFromName(structure.Name),
		Fields:         fields,
	}, nil
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
