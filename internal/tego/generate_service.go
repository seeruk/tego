package tego

import (
	"fmt"
	"strings"
)

const (
	connectImportPath = "connectrpc.com/connect"
	contextImportPath = "context"
	errorsImportPath  = "errors"
	fmtImportPath     = "fmt"
	grpcImportPath    = "google.golang.org/grpc"
	httpImportPath    = "net/http"
	ioImportPath      = "io"
	iterImportPath    = "iter"
)

func generateService(g *generatedFile, service ServicePlan, rpc RPCOptions) error {
	generateComment(g, "", service.Comment)
	g.P("type ", service.Name, " interface {")
	for _, method := range service.Methods {
		signature, err := generateServiceMethodSignature(g, method)
		if err != nil {
			return fmt.Errorf("service %s method %s: %w", service.ProtoName, method.ProtoName, err)
		}
		generateComment(g, "\t", method.Comment)
		g.P("\t", signature)
	}
	g.P("}")
	g.P()

	if err := generateUnimplementedService(g, service); err != nil {
		return err
	}

	if err := generateServiceHooks(g, service); err != nil {
		return err
	}

	if rpc.GRPC {
		if err := generateGRPCService(g, service); err != nil {
			return err
		}
	}
	if rpc.Connect {
		if err := generateConnectService(g, service); err != nil {
			return err
		}
	}

	return nil
}

func generateServiceMethodSignature(g *generatedFile, method ServiceMethodPlan) (string, error) {
	request, response, err := generateServiceMethodTypes(g, method)
	if err != nil {
		return "", err
	}

	contextType := generateContextType(g)
	switch method.StreamType {
	case ServiceStreamTypeUnary:
		arguments := "ctx " + contextType + ", request " + request
		if method.InlineRequest != nil {
			params, err := generateServiceInlineFieldParameters(g, method.InlineRequest.Fields)
			if err != nil {
				return "", err
			}
			arguments = "ctx " + contextType + ", " + params
		}
		if serviceResponseIsEmpty(method) {
			return fmt.Sprintf("%s(%s) error", method.Name, arguments), nil
		}
		results := response + ", error"
		if method.InlineResponse != nil {
			inlineResults, err := generateServiceInlineFieldTypes(g, method.InlineResponse.Fields)
			if err != nil {
				return "", err
			}
			results = inlineResults + ", error"
		}
		return fmt.Sprintf("%s(%s) (%s)", method.Name, arguments, results), nil
	case ServiceStreamTypeServerStreaming:
		arguments := "ctx " + contextType + ", request " + request
		if method.InlineRequest != nil {
			params, err := generateServiceInlineFieldParameters(g, method.InlineRequest.Fields)
			if err != nil {
				return "", err
			}
			arguments = "ctx " + contextType + ", " + params
		}
		return fmt.Sprintf(
			"%s(%s) (%s, error)",
			method.Name,
			arguments,
			generateSeq2Type(g, response),
		), nil
	case ServiceStreamTypeClientStreaming:
		requests := generateSeq2Type(g, request)
		if serviceResponseIsEmpty(method) {
			return fmt.Sprintf("%s(ctx %s, requests %s) error", method.Name, contextType, requests), nil
		}
		results := response + ", error"
		if method.InlineResponse != nil {
			inlineResults, err := generateServiceInlineFieldTypes(g, method.InlineResponse.Fields)
			if err != nil {
				return "", err
			}
			results = inlineResults + ", error"
		}
		return fmt.Sprintf("%s(ctx %s, requests %s) (%s)", method.Name, contextType, requests, results), nil
	case ServiceStreamTypeBidiStreaming:
		return fmt.Sprintf(
			"%s(ctx %s, requests %s) (%s, error)",
			method.Name,
			contextType,
			generateSeq2Type(g, request),
			generateSeq2Type(g, response),
		), nil
	default:
		return "", fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
}

func generateServiceMethodTypes(g *generatedFile, method ServiceMethodPlan) (string, string, error) {
	request, err := generateType(g, method.Request.Type)
	if err != nil {
		return "", "", fmt.Errorf("request type: %w", err)
	}

	response, err := generateType(g, method.Response.Type)
	if err != nil {
		return "", "", fmt.Errorf("response type: %w", err)
	}

	return request, response, nil
}

func generateContextType(g *generatedFile) string {
	return generateNamedType(g, GoTypeRef{ImportPath: contextImportPath, Name: "Context"})
}

func generateContextWithCancel(g *generatedFile) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: contextImportPath, Name: "WithCancel"})
}

func generateSeq2Type(g *generatedFile, value string) string {
	return generateNamedType(g, GoTypeRef{ImportPath: iterImportPath, Name: "Seq2"}) + "[" + value + ", error]"
}

func serviceResponseIsEmpty(method ServiceMethodPlan) bool {
	return method.Response.Type.Kind == TypeKindEmptyStruct
}

func generateServiceRequestInlineHelper(g *generatedFile, helper ServiceInlineHelperPlan) error {
	messageType, err := generateType(g, helper.Type)
	if err != nil {
		return fmt.Errorf("inline helper type: %w", err)
	}
	contextType := generateContextType(g)
	fieldTypes, err := generateServiceInlineFieldTypes(g, helper.Fields)
	if err != nil {
		return err
	}

	g.P("func ", helper.ToInlineName, "(ctx ", contextType, ", request ", messageType, ") (", contextType, ", ", fieldTypes, ") {")
	g.P("\treturn ctx, ", serviceInlineStructFieldValues("request", helper.Fields))
	g.P("}")
	g.P()

	fieldParameters, err := generateServiceInlineFieldParameters(g, helper.Fields)
	if err != nil {
		return err
	}
	g.P("func ", helper.FromInlineName, "(ctx ", contextType, ", ", fieldParameters, ") (", contextType, ", ", messageType, ") {")
	g.P("\treturn ctx, ", messageType, "{", serviceInlineStructLiteralFields(helper.Fields), "}")
	g.P("}")
	g.P()
	return nil
}

func generateServiceResponseInlineHelper(g *generatedFile, helper ServiceInlineHelperPlan) error {
	messageType, err := generateType(g, helper.Type)
	if err != nil {
		return fmt.Errorf("inline helper type: %w", err)
	}
	fieldTypes, err := generateServiceInlineFieldTypes(g, helper.Fields)
	if err != nil {
		return err
	}

	g.P("func ", helper.ToInlineName, "(response ", messageType, ", err error) (", fieldTypes, ", error) {")
	if err := generateServiceInlineErrorReturn(g, helper.Fields); err != nil {
		return err
	}
	g.P("\treturn ", serviceInlineStructFieldValues("response", helper.Fields), ", nil")
	g.P("}")
	g.P()

	fieldParameters, err := generateServiceInlineFieldParameters(g, helper.Fields)
	if err != nil {
		return err
	}
	g.P("func ", helper.FromInlineName, "(", fieldParameters, ", err error) (", messageType, ", error) {")
	g.P("\tif err != nil {")
	g.P("\t\tvar zero ", messageType)
	g.P("\t\treturn zero, err")
	g.P("\t}")
	g.P("\treturn ", messageType, "{", serviceInlineStructLiteralFields(helper.Fields), "}, nil")
	g.P("}")
	g.P()
	return nil
}

func generateServiceInlineFieldParameters(g *generatedFile, fields []ServiceInlineFieldPlan) (string, error) {
	parameters := make([]string, 0, len(fields))
	for _, field := range fields {
		typ, err := generateType(g, field.Type)
		if err != nil {
			return "", fmt.Errorf("inline field %s: %w", field.FieldName, err)
		}
		parameters = append(parameters, field.Name+" "+typ)
	}
	return strings.Join(parameters, ", "), nil
}

func generateServiceInlineFieldTypes(g *generatedFile, fields []ServiceInlineFieldPlan) (string, error) {
	types := make([]string, 0, len(fields))
	for _, field := range fields {
		typ, err := generateType(g, field.Type)
		if err != nil {
			return "", fmt.Errorf("inline field %s: %w", field.FieldName, err)
		}
		types = append(types, typ)
	}
	return strings.Join(types, ", "), nil
}

func serviceInlineStructFieldValues(source string, fields []ServiceInlineFieldPlan) string {
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		values = append(values, source+"."+field.FieldName)
	}
	return strings.Join(values, ", ")
}

func serviceInlineStructLiteralFields(fields []ServiceInlineFieldPlan) string {
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		values = append(values, field.FieldName+": "+field.Name)
	}
	return strings.Join(values, ", ")
}

func generateServiceInlineErrorReturn(g *generatedFile, fields []ServiceInlineFieldPlan) error {
	g.P("\tif err != nil {")
	zeros := make([]string, 0, len(fields))
	names := newTempNameAllocator("err")
	for _, field := range fields {
		zero := names.name("zero" + goName(field.FieldName))
		typ, err := generateType(g, field.Type)
		if err != nil {
			return fmt.Errorf("inline field %s: %w", field.FieldName, err)
		}
		g.P("\t\tvar ", zero, " ", typ)
		zeros = append(zeros, zero)
	}
	g.P("\t\treturn ", strings.Join(append(zeros, "err"), ", "))
	g.P("\t}")
	return nil
}

func generateUnimplementedService(g *generatedFile, service ServicePlan) error {
	g.P("type ", service.UnimplementedName, " struct{}")
	g.P()

	for _, method := range service.Methods {
		signature, err := generateServiceMethodSignature(g, method)
		if err != nil {
			return err
		}

		g.P("func (", service.UnimplementedName, ") ", signature, " {")
		if err := generateUnimplementedServiceMethodBody(g, service, method); err != nil {
			return err
		}
		g.P("}")
		g.P()
	}

	g.P("func ", service.UnimplementedErrorName(), "(method string) error {")
	g.P("\treturn ", generateFmtSymbol(g, "Errorf"), "(", fmt.Sprintf("%q", service.Name+".%s: %w"), ", method, ", generateTegoSymbol(g, "ErrUnimplemented"), ")")
	g.P("}")
	g.P()
	return nil
}

func generateUnimplementedServiceMethodBody(
	g *generatedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	errExpr := service.UnimplementedErrorName() + "(" + fmt.Sprintf("%q", method.Name) + ")"
	switch method.StreamType {
	case ServiceStreamTypeUnary, ServiceStreamTypeClientStreaming:
		if serviceResponseIsEmpty(method) {
			g.P("\treturn ", errExpr)
			return nil
		}
		if method.InlineResponse != nil {
			return generateServiceInlineZeroReturn(g, method.InlineResponse.Fields, errExpr)
		}
		responseType, err := generateType(g, method.Response.Type)
		if err != nil {
			return fmt.Errorf("response type: %w", err)
		}
		g.P("\tvar zero ", responseType)
		g.P("\treturn zero, ", errExpr)
		return nil
	case ServiceStreamTypeServerStreaming, ServiceStreamTypeBidiStreaming:
		g.P("\treturn nil, ", errExpr)
		return nil
	default:
		return fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
}

func (service ServicePlan) UnimplementedErrorName() string {
	return "unimplemented" + service.Name + "Error"
}

func generateServiceInlineZeroReturn(
	g *generatedFile,
	fields []ServiceInlineFieldPlan,
	errExpr string,
) error {
	zeros := make([]string, 0, len(fields))
	names := newTempNameAllocator("err")
	for _, field := range fields {
		zero := names.name("zero" + goName(field.FieldName))
		typ, err := generateType(g, field.Type)
		if err != nil {
			return fmt.Errorf("inline field %s: %w", field.FieldName, err)
		}
		g.P("\tvar ", zero, " ", typ)
		zeros = append(zeros, zero)
	}
	g.P("\treturn ", strings.Join(append(zeros, errExpr), ", "))
	return nil
}

func generateServiceHooks(g *generatedFile, service ServicePlan) error {
	hooksType := serviceHooksTypeName(service)
	g.P("type ", hooksType, " struct {")
	for _, method := range service.Methods {
		g.P("\t", hookFieldName("Pre", method, "Request"), " []", hookTypeName(service, "Pre", method, "Request"))
		g.P("\t", hookFieldName("Post", method, "Request"), " []", hookTypeName(service, "Post", method, "Request"))
		g.P("\t", hookFieldName("Pre", method, "Response"), " []", hookTypeName(service, "Pre", method, "Response"))
		g.P("\t", hookFieldName("Post", method, "Response"), " []", hookTypeName(service, "Post", method, "Response"))
	}
	g.P("}")
	g.P()

	for _, method := range service.Methods {
		requestProto, err := generateType(g, method.Request.ProtoType)
		if err != nil {
			return fmt.Errorf("hook request proto type: %w", err)
		}
		request, err := generateType(g, method.Request.Type)
		if err != nil {
			return fmt.Errorf("hook request type: %w", err)
		}
		response, err := generateType(g, method.Response.Type)
		if err != nil {
			return fmt.Errorf("hook response type: %w", err)
		}
		responseProto, err := generateType(g, method.Response.ProtoType)
		if err != nil {
			return fmt.Errorf("hook response proto type: %w", err)
		}

		contextType := generateContextType(g)
		rpcInfo := generateTegoSymbol(g, "RPCInfo")
		g.P("type ", hookTypeName(service, "Pre", method, "Request"), " func(", contextType, ", ", rpcInfo, ", ", requestProto, ") (", contextType, ", ", requestProto, ", error)")
		g.P()
		g.P("type ", hookTypeName(service, "Post", method, "Request"), " func(", contextType, ", ", rpcInfo, ", ", request, ") (", contextType, ", ", request, ", error)")
		g.P()
		g.P("type ", hookTypeName(service, "Pre", method, "Response"), " func(", contextType, ", ", rpcInfo, ", ", response, ") (", response, ", error)")
		g.P()
		g.P("type ", hookTypeName(service, "Post", method, "Response"), " func(", contextType, ", ", rpcInfo, ", ", responseProto, ") (", responseProto, ", error)")
		g.P()
	}

	for _, method := range service.Methods {
		for _, slot := range hookSlots() {
			field := hookFieldName(slot.when, method, slot.side)
			hookType := hookTypeName(service, slot.when, method, slot.side)
			g.P("func (h *", hooksType, ") Add", field, "Hook(hooks ...", hookType, ") *", hooksType, " {")
			g.P("\th.", field, " = append(h.", field, ", hooks...)")
			g.P("\treturn h")
			g.P("}")
			g.P()
			g.P("func (h *", hooksType, ") Set", field, "Hooks(hooks ...", hookType, ") *", hooksType, " {")
			g.P("\th.", field, " = hooks")
			g.P("\treturn h")
			g.P("}")
			g.P()
		}
	}

	g.P("func ", mergeHooksName(service), "(hooks ...", hooksType, ") ", hooksType, " {")
	g.P("\tvar merged ", hooksType)
	g.P("\tfor _, hooks := range hooks {")
	for _, method := range service.Methods {
		for _, slot := range hookSlots() {
			field := hookFieldName(slot.when, method, slot.side)
			g.P("\t\tmerged.", field, " = append(merged.", field, ", hooks.", field, "...)")
		}
	}
	g.P("\t}")
	g.P("\treturn merged")
	g.P("}")
	g.P()
	return nil
}

func serviceHooksTypeName(service ServicePlan) string {
	return service.Name + "Hooks"
}

func mergeHooksName(service ServicePlan) string {
	return "merge" + serviceHooksTypeName(service)
}

func hookFieldName(when string, method ServiceMethodPlan, side string) string {
	return when + method.Name + side + "Mapping"
}

func hookTypeName(service ServicePlan, when string, method ServiceMethodPlan, side string) string {
	return service.Name + hookFieldName(when, method, side) + "Hook"
}

type hookSlot struct {
	when string
	side string
}

func hookSlots() []hookSlot {
	return []hookSlot{
		{when: "Pre", side: "Request"},
		{when: "Post", side: "Request"},
		{when: "Pre", side: "Response"},
		{when: "Post", side: "Response"},
	}
}

func generateGRPCService(g *generatedFile, service ServicePlan) error {
	g.P("func ", service.GRPCRegisterName, "(", "registrar ", generateGRPCType(g, "ServiceRegistrar"), ", service ", service.Name, ", opts ...", generateTegoSymbol(g, "GRPCServerOption"), ") {")
	g.P("\t", generateProtoServiceSymbol(g, service, "Register", "Server"), "(registrar, ", service.GRPCNewServerName, "(service, opts...))")
	g.P("}")
	g.P()

	g.P("func ", service.GRPCRegisterWithAdapterName(), "(", "registrar ", generateGRPCType(g, "ServiceRegistrar"), ", adapter *", service.GRPCAdapterName, ", opts ...", generateTegoSymbol(g, "GRPCServerOption"), ") {")
	g.P("\t", generateProtoServiceSymbol(g, service, "Register", "Server"), "(registrar, ", service.GRPCNewServerWithAdapterName(), "(adapter, opts...))")
	g.P("}")
	g.P()

	g.P("func ", service.GRPCNewServerName, "(service ", service.Name, ", opts ...", generateTegoSymbol(g, "GRPCServerOption"), ") ", generateProtoServiceType(g, service, "", "Server"), " {")
	g.P("\treturn ", service.GRPCNewServerWithAdapterName(), "(New", service.GRPCAdapterName, "(service), opts...)")
	g.P("}")
	g.P()

	g.P("func ", service.GRPCNewServerWithAdapterName(), "(adapter *", service.GRPCAdapterName, ", opts ...", generateTegoSymbol(g, "GRPCServerOption"), ") ", generateProtoServiceType(g, service, "", "Server"), " {")
	g.P("\toptions := ", generateTegoSymbol(g, "NewGRPCServerOptions"), "(opts...)")
	g.P("\tadapter.errorMapper = options.ErrorMapper(adapter.errorMapper)")
	g.P("\treturn &", service.GRPCServerName, "{", service.GRPCAdapterName, ": adapter}")
	g.P("}")
	g.P()

	g.P("type ", service.GRPCServerName, " struct {")
	g.P("\t", generateProtoServiceType(g, service, "Unimplemented", "Server"))
	g.P("\t*", service.GRPCAdapterName)
	g.P("}")
	g.P()

	generateGRPCAdapter(g, service)
	if err := generateAdapterHookRunners(g, service, service.GRPCAdapterName); err != nil {
		return err
	}

	for _, method := range service.Methods {
		if err := generateGRPCServerMethod(g, service, method); err != nil {
			return fmt.Errorf("gRPC server method %s: %w", method.ProtoName, err)
		}
		if err := generateGRPCAdapterMethod(g, service, method); err != nil {
			return fmt.Errorf("gRPC adapter method %s: %w", method.ProtoName, err)
		}
	}

	g.P("func ", service.GRPCNewClientName, "(client ", generateProtoServiceType(g, service, "", "Client"), ", opts ...", generateTegoSymbol(g, "GRPCClientOption"), ") ", service.Name, " {")
	g.P("\toptions := ", generateTegoSymbol(g, "NewGRPCClientOptions"), "(opts...)")
	g.P("\treturn &", service.GRPCClientName, "{client: client, errorMapper: options.ErrorMapper(nil)}")
	g.P("}")
	g.P()

	g.P("type ", service.GRPCClientName, " struct {")
	g.P("\tclient ", generateProtoServiceType(g, service, "", "Client"))
	g.P("\terrorMapper ", generateTegoSymbol(g, "ErrorMapper"))
	g.P("}")
	g.P()

	g.P("func (c *", service.GRPCClientName, ") mapError(err error) error {")
	g.P("\tif err == nil {")
	g.P("\t\treturn nil")
	g.P("\t}")
	g.P("\tif c.errorMapper == nil {")
	g.P("\t\treturn err")
	g.P("\t}")
	g.P("\treturn c.errorMapper(err)")
	g.P("}")
	g.P()

	for _, method := range service.Methods {
		if err := generateGRPCClientMethod(g, service, method); err != nil {
			return fmt.Errorf("gRPC client method %s: %w", method.ProtoName, err)
		}
	}

	return nil
}

func (service ServicePlan) GRPCNewServerWithAdapterName() string {
	return service.GRPCNewServerName + "WithAdapter"
}

func (service ServicePlan) GRPCRegisterWithAdapterName() string {
	return service.GRPCRegisterName + "WithAdapter"
}

func generateGRPCAdapter(g *generatedFile, service ServicePlan) {
	g.P("type ", service.GRPCAdapterName, " struct {")
	g.P("\tservice ", service.Name)
	g.P("\terrorMapper ", generateTegoSymbol(g, "ErrorMapper"))
	g.P("\tserviceHooks ", serviceHooksTypeName(service))
	generateAdapterInterfaceHookFields(g)
	g.P("}")
	g.P()

	g.P("func New", service.GRPCAdapterName, "(service ", service.Name, ", opts ...", generateTegoSymbol(g, "GRPCAdapterOption"), ") *", service.GRPCAdapterName, " {")
	g.P("\toptions := ", generateTegoSymbol(g, "NewGRPCAdapterOptions"), "(opts...)")
	g.P("\treturn &", service.GRPCAdapterName, "{service: service, errorMapper: options.ErrorMapper(", generateTegoSymbol(g, "GRPCError"), ")}")
	g.P("}")
	g.P()

	g.P("func (a *", service.GRPCAdapterName, ") mapError(err error) error {")
	g.P("\tif err == nil {")
	g.P("\t\treturn nil")
	g.P("\t}")
	g.P("\tif a.errorMapper == nil {")
	g.P("\t\treturn ", generateTegoSymbol(g, "GRPCError"), "(err)")
	g.P("\t}")
	g.P("\treturn a.errorMapper(err)")
	g.P("}")
	g.P()

	generateAdapterHookMethods(g, service, service.GRPCAdapterName)
}

func generateAdapterInterfaceHookFields(g *generatedFile) {
	g.P("\tinterfaceHooks ", generateTegoSymbol(g, "InterfaceHooks"))
}

func generateAdapterHookMethods(g *generatedFile, service ServicePlan, adapterName string) {
	hooksType := serviceHooksTypeName(service)
	g.P("func (a *", adapterName, ") AddServiceHooks(hooks ", hooksType, ") *", adapterName, " {")
	g.P("\ta.serviceHooks = ", mergeHooksName(service), "(a.serviceHooks, hooks)")
	g.P("\treturn a")
	g.P("}")
	g.P()

	g.P("func (a *", adapterName, ") SetServiceHooks(hooks ", hooksType, ") *", adapterName, " {")
	g.P("\ta.serviceHooks = ", mergeHooksName(service), "(hooks)")
	g.P("\treturn a")
	g.P("}")
	g.P()

	g.P("func (a *", adapterName, ") AddInterfaceHooks(hooks ", generateTegoSymbol(g, "InterfaceHooks"), ") *", adapterName, " {")
	g.P("\ta.interfaceHooks = ", generateTegoSymbol(g, "MergeInterfaceHooks"), "(a.interfaceHooks, hooks)")
	g.P("\treturn a")
	g.P("}")
	g.P()

	g.P("func (a *", adapterName, ") SetInterfaceHooks(hooks ", generateTegoSymbol(g, "InterfaceHooks"), ") *", adapterName, " {")
	g.P("\ta.interfaceHooks = ", generateTegoSymbol(g, "MergeInterfaceHooks"), "(hooks)")
	g.P("\treturn a")
	g.P("}")
	g.P()
}

func generateAdapterHookRunners(g *generatedFile, service ServicePlan, adapterName string) error {
	for _, method := range service.Methods {
		if err := generateAdapterHookRunner(g, service, adapterName, method, "Pre", "Request", method.Request.ProtoType); err != nil {
			return err
		}
		if err := generateAdapterHookRunner(g, service, adapterName, method, "Post", "Request", method.Request.Type); err != nil {
			return err
		}
		if err := generateAdapterHookRunner(g, service, adapterName, method, "Pre", "Response", method.Response.Type); err != nil {
			return err
		}
		if err := generateAdapterHookRunner(g, service, adapterName, method, "Post", "Response", method.Response.ProtoType); err != nil {
			return err
		}
	}
	return nil
}

func generateAdapterHookRunner(
	g *generatedFile,
	service ServicePlan,
	adapterName string,
	method ServiceMethodPlan,
	when string,
	side string,
	valueType TypePlan,
) error {
	typ, err := generateType(g, valueType)
	if err != nil {
		return fmt.Errorf("hook runner value type: %w", err)
	}
	contextType := generateContextType(g)
	rpcInfo := generateTegoSymbol(g, "RPCInfo")
	name := hookRunnerName(when, method, side)

	if side == "Request" {
		g.P("func (a *", adapterName, ") ", name, "(ctx ", contextType, ", info ", rpcInfo, ", value ", typ, ") (", contextType, ", ", typ, ", error) {")
		g.P("\tfor _, hook := range a.serviceHooks.", hookFieldName(when, method, side), " {")
		g.P("\t\tvar err error")
		g.P("\t\tctx, value, err = hook(ctx, info, value)")
		g.P("\t\tif err != nil {")
		g.P("\t\t\treturn ctx, value, err")
		g.P("\t\t}")
		g.P("\t}")
		g.P("\tvar err error")
		g.P("\tctx, err = ", requestInterfaceHookRunnerSymbol(g, when), "(ctx, info, value, a.interfaceHooks.", interfaceHookFieldName(when, side), ")")
		g.P("\tif err != nil {")
		g.P("\t\treturn ctx, value, err")
		g.P("\t}")
		g.P("\treturn ctx, value, nil")
		g.P("}")
		g.P()
		return nil
	}

	g.P("func (a *", adapterName, ") ", name, "(ctx ", contextType, ", info ", rpcInfo, ", value ", typ, ") (", typ, ", error) {")
	g.P("\tfor _, hook := range a.serviceHooks.", hookFieldName(when, method, side), " {")
	g.P("\t\tvar err error")
	g.P("\t\tvalue, err = hook(ctx, info, value)")
	g.P("\t\tif err != nil {")
	g.P("\t\t\treturn value, err")
	g.P("\t\t}")
	g.P("\t}")
	interfaceHookValue := "value"
	if when == "Pre" {
		interfaceHookValue = "&value"
	}
	g.P("\tif err := ", responseInterfaceHookRunnerSymbol(g, when), "(ctx, info, ", interfaceHookValue, ", a.interfaceHooks.", interfaceHookFieldName(when, side), "); err != nil {")
	g.P("\t\treturn value, err")
	g.P("\t}")
	g.P("\treturn value, nil")
	g.P("}")
	g.P()
	return nil
}

func hookRunnerName(when string, method ServiceMethodPlan, side string) string {
	return "run" + hookFieldName(when, method, side)
}

func interfaceHookFieldName(when string, side string) string {
	return when + side + "Mapping"
}

func requestInterfaceHookRunnerSymbol(g *generatedFile, when string) string {
	if when == "Pre" {
		return generateTegoSymbol(g, "RunPreRequestMappingInterfaceHooks")
	}
	return generateTegoSymbol(g, "RunPostRequestMappingInterfaceHooks")
}

func responseInterfaceHookRunnerSymbol(g *generatedFile, when string) string {
	if when == "Pre" {
		return generateTegoSymbol(g, "RunPreResponseMappingInterfaceHooks")
	}
	return generateTegoSymbol(g, "RunPostResponseMappingInterfaceHooks")
}

func generateGRPCServerMethod(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	nativeMethodName := serviceNativeMethodName(method)
	signature, err := generateGRPCServerMethodSignature(g, nativeMethodName, method)
	if err != nil {
		return err
	}

	arguments, err := generateGRPCServerMethodArguments(method)
	if err != nil {
		return err
	}

	g.P("func (s *", service.GRPCServerName, ") ", signature, " {")
	g.P("\treturn s.Adapt", nativeMethodName, "(", arguments, ")")
	g.P("}")
	g.P()
	return nil
}

func generateGRPCAdapterMethod(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	signature, err := generateGRPCServerMethodSignature(g, "Adapt"+serviceNativeMethodName(method), method)
	if err != nil {
		return err
	}

	g.P("func (a *", service.GRPCAdapterName, ") ", signature, " {")
	switch method.StreamType {
	case ServiceStreamTypeUnary:
		err = generateGRPCAdapterUnaryMethodBody(g, service, method)
	case ServiceStreamTypeServerStreaming:
		err = generateGRPCAdapterServerStreamingMethodBody(g, service, method)
	case ServiceStreamTypeClientStreaming:
		err = generateGRPCAdapterClientStreamingMethodBody(g, service, method)
	case ServiceStreamTypeBidiStreaming:
		err = generateGRPCAdapterBidiStreamingMethodBody(g, service, method)
	default:
		err = fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
	if err != nil {
		return err
	}
	g.P("}")
	g.P()
	return nil
}

func generateGRPCServerMethodSignature(
	g *generatedFile,
	name string,
	method ServiceMethodPlan,
) (string, error) {
	contextType := generateContextType(g)
	requestProto, responseProto, err := generateServiceMethodProtoTypes(g, method)
	if err != nil {
		return "", err
	}

	switch method.StreamType {
	case ServiceStreamTypeUnary:
		return fmt.Sprintf("%s(ctx %s, requestProto %s) (%s, error)", name, contextType, requestProto, responseProto), nil
	case ServiceStreamTypeServerStreaming:
		stream, err := generateGRPCStreamType(g, "ServerStreamingServer", method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(requestProto %s, stream %s) error", name, requestProto, stream), nil
	case ServiceStreamTypeClientStreaming:
		stream, err := generateGRPCStreamType(g, "ClientStreamingServer", method.Request.ProtoType, method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(stream %s) error", name, stream), nil
	case ServiceStreamTypeBidiStreaming:
		stream, err := generateGRPCStreamType(g, "BidiStreamingServer", method.Request.ProtoType, method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(stream %s) error", name, stream), nil
	default:
		return "", fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
}

func generateGRPCServerMethodArguments(method ServiceMethodPlan) (string, error) {
	switch method.StreamType {
	case ServiceStreamTypeUnary:
		return "ctx, requestProto", nil
	case ServiceStreamTypeServerStreaming:
		return "requestProto, stream", nil
	case ServiceStreamTypeClientStreaming, ServiceStreamTypeBidiStreaming:
		return "stream", nil
	default:
		return "", fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
}

func generateGRPCAdapterUnaryMethodBody(
	g *generatedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	ctx.line("info := " + serviceRPCInfoExpr(g, service, method))
	ctx.line("ctx, requestProto, err := a." + hookRunnerName("Pre", method, "Request") + "(ctx, info, requestProto)")
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("ctx, request, err = a." + hookRunnerName("Post", method, "Request") + "(ctx, info, request)")
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")

	call := serviceMethodCall(method, "a.service", "ctx", "request")
	if serviceResponseIsEmpty(method) {
		ctx.line("if err := " + call + "; err != nil {")
		ctx.line("return nil, a.mapError(err)")
		ctx.line("}")
		ctx.line("response, err := a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, struct{}{})")
		ctx.line("if err != nil {")
		ctx.line("return nil, a.mapError(err)")
		ctx.line("}")
		if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
			return fmt.Errorf("response: %w", err)
		}
		ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
		ctx.line("if err != nil {")
		ctx.line("return nil, a.mapError(err)")
		ctx.line("}")
		ctx.line("return responseProto, nil")
		return nil
	}

	if method.InlineResponse != nil {
		ctx.line("response, err := " + method.InlineResponse.FromInlineName + "(" + call + ")")
	} else {
		ctx.line("response, err := " + call)
	}
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")
	ctx.line("response, err = a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, response)")
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")
	ctx.line("return responseProto, nil")
	return nil
}

func generateGRPCAdapterServerStreamingMethodBody(
	g *generatedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	ctx := newMappingRenderContext(g, true, "err")
	ctx.line("ctx := stream.Context()")
	ctx.line("info := " + serviceRPCInfoExpr(g, service, method))
	ctx.line("ctx, requestProto, err := a." + hookRunnerName("Pre", method, "Request") + "(ctx, info, requestProto)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("ctx, request, err = a." + hookRunnerName("Post", method, "Request") + "(ctx, info, request)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	call := serviceMethodCall(method, "a.service", "ctx", "request")
	ctx.line("responses, err := " + call)
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("if responses == nil {")
	ctx.line("return nil")
	ctx.line("}")
	ctx.line("for response, err := range responses {")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("response, err = a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, response)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("if err := stream.Send(responseProto); err != nil {")
	ctx.line("return err")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return nil")
	return nil
}

func generateGRPCAdapterClientStreamingMethodBody(
	g *generatedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	requestType, err := generateType(g, method.Request.Type)
	if err != nil {
		return fmt.Errorf("request type: %w", err)
	}
	ctx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2ReceiveErrorLines(requestType), "requestProto")
	ctx.line("ctx := stream.Context()")
	ctx.line("info := " + serviceRPCInfoExpr(g, service, method))
	ctx.line("var receiveErr error")
	ctx.line("requests := func(yield func(" + requestType + ", error) bool) {")
	ctx.line("for {")
	ctx.line("requestProto, err := stream.Recv()")
	ctx.line("if err == " + generateIOSymbol(g, "EOF") + " {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("if err != nil {")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	ctx.line("ctx, requestProto, err = a." + hookRunnerName("Pre", method, "Request") + "(ctx, info, requestProto)")
	ctx.line("if err != nil {")
	ctx.line("err = a.mapError(err)")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("ctx, request, err = a." + hookRunnerName("Post", method, "Request") + "(ctx, info, request)")
	ctx.line("if err != nil {")
	ctx.line("err = a.mapError(err)")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	ctx.line("if !yield(request, nil) {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("}")

	ctx = newMappingRenderContext(g, true, "err")
	if serviceResponseIsEmpty(method) {
		ctx.line("if err := a.service." + method.Name + "(ctx, requests); err != nil {")
		ctx.line("return a.mapError(err)")
		ctx.line("}")
		ctx.line("if receiveErr != nil {")
		ctx.line("return receiveErr")
		ctx.line("}")
		ctx.line("response, err := a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, struct{}{})")
		ctx.line("if err != nil {")
		ctx.line("return a.mapError(err)")
		ctx.line("}")
		if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
			return fmt.Errorf("response: %w", err)
		}
		ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
		ctx.line("if err != nil {")
		ctx.line("return a.mapError(err)")
		ctx.line("}")
		ctx.line("return stream.SendAndClose(responseProto)")
		return nil
	}

	call := "a.service." + method.Name + "(ctx, requests)"
	if method.InlineResponse != nil {
		ctx.line("response, err := " + method.InlineResponse.FromInlineName + "(" + call + ")")
	} else {
		ctx.line("response, err := " + call)
	}
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("if receiveErr != nil {")
	ctx.line("return receiveErr")
	ctx.line("}")
	ctx.line("response, err = a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, response)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("return stream.SendAndClose(responseProto)")
	return nil
}

func generateGRPCAdapterBidiStreamingMethodBody(
	g *generatedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	requestType, err := generateType(g, method.Request.Type)
	if err != nil {
		return fmt.Errorf("request type: %w", err)
	}
	ctx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2ReceiveErrorLines(requestType), "requestProto")
	ctx.line("ctx := stream.Context()")
	ctx.line("info := " + serviceRPCInfoExpr(g, service, method))
	ctx.line("var receiveErr error")
	ctx.line("requests := func(yield func(" + requestType + ", error) bool) {")
	ctx.line("for {")
	ctx.line("requestProto, err := stream.Recv()")
	ctx.line("if err == " + generateIOSymbol(g, "EOF") + " {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("if err != nil {")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	ctx.line("ctx, requestProto, err = a." + hookRunnerName("Pre", method, "Request") + "(ctx, info, requestProto)")
	ctx.line("if err != nil {")
	ctx.line("err = a.mapError(err)")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("ctx, request, err = a." + hookRunnerName("Post", method, "Request") + "(ctx, info, request)")
	ctx.line("if err != nil {")
	ctx.line("err = a.mapError(err)")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	ctx.line("if !yield(request, nil) {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("}")
	ctx = newMappingRenderContext(g, true, "err")
	ctx.line("responses, err := a.service." + method.Name + "(ctx, requests)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("if responses != nil {")
	ctx.line("for response, err := range responses {")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("response, err = a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, response)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("if err := stream.Send(responseProto); err != nil {")
	ctx.line("return err")
	ctx.line("}")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return receiveErr")
	return nil
}

func generateGRPCClientMethod(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	signature, err := generateServiceMethodSignature(g, method)
	if err != nil {
		return err
	}

	g.P("func (c *", service.GRPCClientName, ") ", signature, " {")
	switch method.StreamType {
	case ServiceStreamTypeUnary:
		err = generateGRPCClientUnaryMethodBody(g, service, method)
	case ServiceStreamTypeServerStreaming:
		err = generateGRPCClientServerStreamingMethodBody(g, service, method)
	case ServiceStreamTypeClientStreaming:
		err = generateGRPCClientClientStreamingMethodBody(g, service, method)
	case ServiceStreamTypeBidiStreaming:
		err = generateGRPCClientBidiStreamingMethodBody(g, service, method)
	default:
		err = fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
	if err != nil {
		return err
	}
	g.P("}")
	g.P()
	return nil
}

func generateGRPCClientUnaryMethodBody(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, serviceClientErrorReturn(g, method))
	generateServiceClientZeroValue(ctx, g, method)
	if method.InlineRequest != nil {
		ctx.line("ctx, request := " + method.InlineRequest.FromInlineName + "(ctx, " + serviceInlineFieldNames(method.InlineRequest.Fields) + ")")
	}
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request", mappingDirectionToProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	responseProto := "responseProto"
	if serviceResponseIsEmpty(method) {
		responseProto = "_"
	}
	ctx.line(responseProto + ", err := c.client." + serviceNativeMethodName(method) + "(ctx, requestProto)")
	ctx.line("if err != nil {")
	ctx.line("return " + serviceClientMappedErrorReturn(g, method))
	ctx.line("}")
	if serviceResponseIsEmpty(method) {
		ctx.line("return nil")
		return nil
	}
	if err := generateServiceMappedAssignment(ctx, "response", method.Response.FromProto, "responseProto", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	if method.InlineResponse != nil {
		ctx.line("return " + method.InlineResponse.ToInlineName + "(response, nil)")
		return nil
	}
	ctx.line("return response, nil")
	_ = service
	return nil
}

func generateGRPCClientServerStreamingMethodBody(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	responseType, err := generateType(g, method.Response.Type)
	if err != nil {
		return fmt.Errorf("response type: %w", err)
	}
	ctx := newMappingRenderContext(g, true, "nil, err")
	if method.InlineRequest != nil {
		ctx.line("ctx, request := " + method.InlineRequest.FromInlineName + "(ctx, " + serviceInlineFieldNames(method.InlineRequest.Fields) + ")")
	}
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request", mappingDirectionToProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("responses := func(yield func(" + responseType + ", error) bool) {")
	ctx.line("ctx, cancel := " + generateContextWithCancel(g) + "(ctx)")
	ctx.line("defer cancel()")
	ctx.line("stream, err := c.client." + serviceNativeMethodName(method) + "(ctx, requestProto)")
	ctx.line("if err != nil {")
	ctx.line("var zero " + responseType)
	ctx.line("yield(zero, c.mapError(err))")
	ctx.line("return")
	ctx.line("}")
	ctx.line("for {")
	ctx.line("responseProto, err := stream.Recv()")
	ctx.line("if err == " + generateIOSymbol(g, "EOF") + " {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("if err != nil {")
	ctx.line("var zero " + responseType)
	ctx.line("yield(zero, c.mapError(err))")
	ctx.line("return")
	ctx.line("}")
	seqCtx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2YieldErrorLines(responseType))
	if err := generateServiceMappedAssignment(seqCtx, "response", method.Response.FromProto, "responseProto", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("if !yield(response, nil) {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return responses, nil")
	_ = service
	return nil
}

func generateGRPCClientClientStreamingMethodBody(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, serviceClientErrorReturn(g, method))
	generateServiceClientZeroValue(ctx, g, method)
	ctx.line("stream, err := c.client." + serviceNativeMethodName(method) + "(ctx)")
	ctx.line("if err != nil {")
	ctx.line("return " + serviceClientMappedErrorReturn(g, method))
	ctx.line("}")
	ctx.line("for request, err := range requests {")
	ctx.line("if err != nil {")
	ctx.line("return " + serviceClientErrorReturn(g, method))
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request", mappingDirectionToProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("if err := stream.Send(requestProto); err != nil {")
	ctx.line("return " + serviceClientMappedErrorReturn(g, method))
	ctx.line("}")
	ctx.line("}")
	ctx.line("responseProto, err := stream.CloseAndRecv()")
	ctx.line("if err != nil {")
	ctx.line("return " + serviceClientMappedErrorReturn(g, method))
	ctx.line("}")
	if serviceResponseIsEmpty(method) {
		ctx.line("return nil")
		return nil
	}
	if err := generateServiceMappedAssignment(ctx, "response", method.Response.FromProto, "responseProto", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	if method.InlineResponse != nil {
		ctx.line("return " + method.InlineResponse.ToInlineName + "(response, nil)")
		return nil
	}
	ctx.line("return response, nil")
	_ = service
	return nil
}

func generateGRPCClientBidiStreamingMethodBody(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	_, responseType, err := generateServiceMethodTypes(g, method)
	if err != nil {
		return err
	}
	ctx := newMappingRenderContext(g, true, "nil, err")
	ctx.line("responses := func(yield func(" + responseType + ", error) bool) {")
	ctx.line("ctx, cancel := " + generateContextWithCancel(g) + "(ctx)")
	ctx.line("defer cancel()")
	ctx.line("stream, err := c.client." + serviceNativeMethodName(method) + "(ctx)")
	ctx.line("if err != nil {")
	ctx.line("var zero " + responseType)
	ctx.line("yield(zero, c.mapError(err))")
	ctx.line("return")
	ctx.line("}")
	ctx.line("sendErr := make(chan error, 1)")
	ctx.line("go func() {")
	ctx.line("for request, err := range requests {")
	ctx.line("if err != nil {")
	ctx.line("sendErr <- err")
	ctx.line("return")
	ctx.line("}")
	sendCtx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2SendErrorLines())
	if err := generateServiceMappedAssignment(sendCtx, "requestProto", method.Request.ToProto, "request", mappingDirectionToProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("if err := stream.Send(requestProto); err != nil {")
	ctx.line("sendErr <- c.mapError(err)")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("sendErr <- c.mapError(stream.CloseSend())")
	ctx.line("}()")
	ctx.line("for {")
	ctx.line("responseProto, err := stream.Recv()")
	ctx.line("if err == " + generateIOSymbol(g, "EOF") + " {")
	ctx.line("if err := <-sendErr; err != nil {")
	ctx.line("var zero " + responseType)
	ctx.line("yield(zero, err)")
	ctx.line("}")
	ctx.line("return")
	ctx.line("}")
	ctx.line("if err != nil {")
	ctx.line("var zero " + responseType)
	ctx.line("yield(zero, c.mapError(err))")
	ctx.line("return")
	ctx.line("}")
	recvCtx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2YieldErrorLines(responseType))
	if err := generateServiceMappedAssignment(recvCtx, "response", method.Response.FromProto, "responseProto", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("if !yield(response, nil) {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return responses, nil")
	_ = service
	return nil
}

func generateConnectService(g *generatedFile, service ServicePlan) error {
	g.P("func ", service.ConnectNewHandlerName, "(service ", service.Name, ", opts ...", generateTegoSymbol(g, "ConnectHandlerOption"), ") (string, ", generateHTTPType(g, "Handler"), ") {")
	g.P("\treturn ", service.ConnectNewHandlerWithAdapterName(), "(New", service.ConnectAdapterName, "(service), opts...)")
	g.P("}")
	g.P()

	g.P("func ", service.ConnectNewHandlerWithAdapterName(), "(adapter *", service.ConnectAdapterName, ", opts ...", generateTegoSymbol(g, "ConnectHandlerOption"), ") (string, ", generateHTTPType(g, "Handler"), ") {")
	g.P("\toptions := ", generateTegoSymbol(g, "NewConnectHandlerOptions"), "(opts...)")
	g.P("\tadapter.errorMapper = options.ErrorMapper(adapter.errorMapper)")
	g.P("\treturn ", generateConnectServiceSymbol(g, service, "New", "Handler"), "(&", service.ConnectHandlerName, "{", service.ConnectAdapterName, ": adapter}, options.ConnectHandlerOptions()...)")
	g.P("}")
	g.P()

	g.P("type ", service.ConnectHandlerName, " struct {")
	g.P("\t*", service.ConnectAdapterName)
	g.P("}")
	g.P()

	generateConnectAdapter(g, service)
	if err := generateAdapterHookRunners(g, service, service.ConnectAdapterName); err != nil {
		return err
	}

	for _, method := range service.Methods {
		if err := generateConnectHandlerMethod(g, service, method); err != nil {
			return fmt.Errorf("connect handler method %s: %w", method.ProtoName, err)
		}
		if err := generateConnectAdapterMethod(g, service, method); err != nil {
			return fmt.Errorf("connect adapter method %s: %w", method.ProtoName, err)
		}
	}

	g.P("func ", service.ConnectNewClientName, "(client ", generateConnectServiceType(g, service, "", "Client"), ", opts ...", generateTegoSymbol(g, "ConnectClientOption"), ") ", service.Name, " {")
	g.P("\toptions := ", generateTegoSymbol(g, "NewConnectClientOptions"), "(opts...)")
	g.P("\treturn &", service.ConnectClientName, "{client: client, errorMapper: options.ErrorMapper(nil)}")
	g.P("}")
	g.P()

	g.P("type ", service.ConnectClientName, " struct {")
	g.P("\tclient ", generateConnectServiceType(g, service, "", "Client"))
	g.P("\terrorMapper ", generateTegoSymbol(g, "ErrorMapper"))
	g.P("}")
	g.P()

	g.P("func (c *", service.ConnectClientName, ") mapError(err error) error {")
	g.P("\tif err == nil {")
	g.P("\t\treturn nil")
	g.P("\t}")
	g.P("\tif c.errorMapper == nil {")
	g.P("\t\treturn err")
	g.P("\t}")
	g.P("\treturn c.errorMapper(err)")
	g.P("}")
	g.P()

	for _, method := range service.Methods {
		if err := generateConnectClientMethod(g, service, method); err != nil {
			return fmt.Errorf("connect client method %s: %w", method.ProtoName, err)
		}
	}

	return nil
}

func (service ServicePlan) ConnectNewHandlerWithAdapterName() string {
	return service.ConnectNewHandlerName + "WithAdapter"
}

func generateConnectAdapter(g *generatedFile, service ServicePlan) {
	g.P("type ", service.ConnectAdapterName, " struct {")
	g.P("\tservice ", service.Name)
	g.P("\terrorMapper ", generateTegoSymbol(g, "ErrorMapper"))
	g.P("\tserviceHooks ", serviceHooksTypeName(service))
	generateAdapterInterfaceHookFields(g)
	g.P("}")
	g.P()

	g.P("func New", service.ConnectAdapterName, "(service ", service.Name, ", opts ...", generateTegoSymbol(g, "ConnectAdapterOption"), ") *", service.ConnectAdapterName, " {")
	g.P("\toptions := ", generateTegoSymbol(g, "NewConnectAdapterOptions"), "(opts...)")
	g.P("\treturn &", service.ConnectAdapterName, "{service: service, errorMapper: options.ErrorMapper(", generateTegoSymbol(g, "ConnectError"), ")}")
	g.P("}")
	g.P()

	g.P("func (a *", service.ConnectAdapterName, ") mapError(err error) error {")
	g.P("\tif err == nil {")
	g.P("\t\treturn nil")
	g.P("\t}")
	g.P("\tif a.errorMapper == nil {")
	g.P("\t\treturn ", generateTegoSymbol(g, "ConnectError"), "(err)")
	g.P("\t}")
	g.P("\treturn a.errorMapper(err)")
	g.P("}")
	g.P()

	generateAdapterHookMethods(g, service, service.ConnectAdapterName)
}

func generateConnectHandlerMethod(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	nativeMethodName := serviceNativeMethodName(method)
	signature, err := generateConnectHandlerMethodSignature(g, nativeMethodName, method)
	if err != nil {
		return err
	}
	arguments, err := generateConnectHandlerMethodArguments(method)
	if err != nil {
		return err
	}

	g.P("func (s *", service.ConnectHandlerName, ") ", signature, " {")
	g.P("\treturn s.Adapt", nativeMethodName, "(", arguments, ")")
	g.P("}")
	g.P()
	return nil
}

func generateConnectAdapterMethod(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	signature, err := generateConnectHandlerMethodSignature(g, "Adapt"+serviceNativeMethodName(method), method)
	if err != nil {
		return err
	}

	g.P("func (a *", service.ConnectAdapterName, ") ", signature, " {")
	switch method.StreamType {
	case ServiceStreamTypeUnary:
		err = generateConnectAdapterUnaryMethodBody(g, service, method)
	case ServiceStreamTypeServerStreaming:
		err = generateConnectAdapterServerStreamingMethodBody(g, service, method)
	case ServiceStreamTypeClientStreaming:
		err = generateConnectAdapterClientStreamingMethodBody(g, service, method)
	case ServiceStreamTypeBidiStreaming:
		err = generateConnectAdapterBidiStreamingMethodBody(g, service, method)
	default:
		err = fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
	if err != nil {
		return err
	}
	g.P("}")
	g.P()
	return nil
}

func generateConnectHandlerMethodSignature(g *generatedFile, name string, method ServiceMethodPlan) (string, error) {
	contextType := generateContextType(g)

	switch method.StreamType {
	case ServiceStreamTypeUnary:
		request, err := generateConnectMessageType(g, "Request", method.Request.ProtoType)
		if err != nil {
			return "", err
		}
		response, err := generateConnectMessageType(g, "Response", method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(ctx %s, requestProto %s) (%s, error)", name, contextType, request, response), nil
	case ServiceStreamTypeServerStreaming:
		request, err := generateConnectMessageType(g, "Request", method.Request.ProtoType)
		if err != nil {
			return "", err
		}
		stream, err := generateConnectMessageType(g, "ServerStream", method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(ctx %s, requestProto %s, stream %s) error", name, contextType, request, stream), nil
	case ServiceStreamTypeClientStreaming:
		stream, err := generateConnectMessageType(g, "ClientStream", method.Request.ProtoType)
		if err != nil {
			return "", err
		}
		response, err := generateConnectMessageType(g, "Response", method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(ctx %s, stream %s) (%s, error)", name, contextType, stream, response), nil
	case ServiceStreamTypeBidiStreaming:
		stream, err := generateConnectMessageType(g, "BidiStream", method.Request.ProtoType, method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(ctx %s, stream %s) error", name, contextType, stream), nil
	default:
		return "", fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
}

func generateConnectHandlerMethodArguments(method ServiceMethodPlan) (string, error) {
	switch method.StreamType {
	case ServiceStreamTypeUnary:
		return "ctx, requestProto", nil
	case ServiceStreamTypeServerStreaming:
		return "ctx, requestProto, stream", nil
	case ServiceStreamTypeClientStreaming, ServiceStreamTypeBidiStreaming:
		return "ctx, stream", nil
	default:
		return "", fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
}

func generateConnectAdapterUnaryMethodBody(
	g *generatedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	ctx.line("info := " + serviceRPCInfoExpr(g, service, method))
	ctx.line("requestProtoMsg := requestProto.Msg")
	ctx.line("ctx, requestProtoMsg, err := a." + hookRunnerName("Pre", method, "Request") + "(ctx, info, requestProtoMsg)")
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProtoMsg", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("ctx, request, err = a." + hookRunnerName("Post", method, "Request") + "(ctx, info, request)")
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")

	call := serviceMethodCall(method, "a.service", "ctx", "request")
	if serviceResponseIsEmpty(method) {
		ctx.line("if err := " + call + "; err != nil {")
		ctx.line("return nil, a.mapError(err)")
		ctx.line("}")
		ctx.line("response, err := a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, struct{}{})")
		ctx.line("if err != nil {")
		ctx.line("return nil, a.mapError(err)")
		ctx.line("}")
		if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
			return fmt.Errorf("response: %w", err)
		}
		ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
		ctx.line("if err != nil {")
		ctx.line("return nil, a.mapError(err)")
		ctx.line("}")
		ctx.line("return " + generateConnectSymbol(g, "NewResponse") + "(responseProto), nil")
		return nil
	}

	if method.InlineResponse != nil {
		ctx.line("response, err := " + method.InlineResponse.FromInlineName + "(" + call + ")")
	} else {
		ctx.line("response, err := " + call)
	}
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")
	ctx.line("response, err = a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, response)")
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")
	ctx.line("return " + generateConnectSymbol(g, "NewResponse") + "(responseProto), nil")
	return nil
}

func generateConnectAdapterServerStreamingMethodBody(
	g *generatedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	ctx := newMappingRenderContext(g, true, "err")
	ctx.line("info := " + serviceRPCInfoExpr(g, service, method))
	ctx.line("requestProtoMsg := requestProto.Msg")
	ctx.line("ctx, requestProtoMsg, err := a." + hookRunnerName("Pre", method, "Request") + "(ctx, info, requestProtoMsg)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProtoMsg", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("ctx, request, err = a." + hookRunnerName("Post", method, "Request") + "(ctx, info, request)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	call := serviceMethodCall(method, "a.service", "ctx", "request")
	ctx.line("responses, err := " + call)
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("if responses == nil {")
	ctx.line("return nil")
	ctx.line("}")
	ctx.line("for response, err := range responses {")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("response, err = a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, response)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("if err := stream.Send(responseProto); err != nil {")
	ctx.line("return err")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return nil")
	return nil
}

func generateConnectAdapterClientStreamingMethodBody(
	g *generatedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	requestType, err := generateType(g, method.Request.Type)
	if err != nil {
		return fmt.Errorf("request type: %w", err)
	}
	ctx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2ReceiveErrorLines(requestType), "requestProto")
	ctx.line("info := " + serviceRPCInfoExpr(g, service, method))
	ctx.line("var receiveErr error")
	ctx.line("requests := func(yield func(" + requestType + ", error) bool) {")
	ctx.line("for stream.Receive() {")
	ctx.line("requestProto := stream.Msg()")
	ctx.line("ctx, requestProto, err := a." + hookRunnerName("Pre", method, "Request") + "(ctx, info, requestProto)")
	ctx.line("if err != nil {")
	ctx.line("err = a.mapError(err)")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("ctx, request, err = a." + hookRunnerName("Post", method, "Request") + "(ctx, info, request)")
	ctx.line("if err != nil {")
	ctx.line("err = a.mapError(err)")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	ctx.line("if !yield(request, nil) {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("if err := stream.Err(); err != nil {")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("}")
	ctx.line("}")

	ctx = newMappingRenderContext(g, true, "nil, err")
	if serviceResponseIsEmpty(method) {
		ctx.line("if err := a.service." + method.Name + "(ctx, requests); err != nil {")
		ctx.line("return nil, a.mapError(err)")
		ctx.line("}")
		ctx.line("if receiveErr != nil {")
		ctx.line("return nil, receiveErr")
		ctx.line("}")
		ctx.line("response, err := a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, struct{}{})")
		ctx.line("if err != nil {")
		ctx.line("return nil, a.mapError(err)")
		ctx.line("}")
		if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
			return fmt.Errorf("response: %w", err)
		}
		ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
		ctx.line("if err != nil {")
		ctx.line("return nil, a.mapError(err)")
		ctx.line("}")
		ctx.line("return " + generateConnectSymbol(g, "NewResponse") + "(responseProto), nil")
		return nil
	}

	call := "a.service." + method.Name + "(ctx, requests)"
	if method.InlineResponse != nil {
		ctx.line("response, err := " + method.InlineResponse.FromInlineName + "(" + call + ")")
	} else {
		ctx.line("response, err := " + call)
	}
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")
	ctx.line("if receiveErr != nil {")
	ctx.line("return nil, receiveErr")
	ctx.line("}")
	ctx.line("response, err = a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, response)")
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
	ctx.line("if err != nil {")
	ctx.line("return nil, a.mapError(err)")
	ctx.line("}")
	ctx.line("return " + generateConnectSymbol(g, "NewResponse") + "(responseProto), nil")
	return nil
}

func generateConnectAdapterBidiStreamingMethodBody(
	g *generatedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	requestType, err := generateType(g, method.Request.Type)
	if err != nil {
		return fmt.Errorf("request type: %w", err)
	}
	ctx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2ReceiveErrorLines(requestType), "requestProto")
	ctx.line("info := " + serviceRPCInfoExpr(g, service, method))
	ctx.line("var receiveErr error")
	ctx.line("requests := func(yield func(" + requestType + ", error) bool) {")
	ctx.line("for {")
	ctx.line("requestProto, err := stream.Receive()")
	ctx.line("if err != nil {")
	ctx.line("if " + generateErrorsSymbol(g, "Is") + "(err, " + generateIOSymbol(g, "EOF") + ") {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	ctx.line("ctx, requestProto, err = a." + hookRunnerName("Pre", method, "Request") + "(ctx, info, requestProto)")
	ctx.line("if err != nil {")
	ctx.line("err = a.mapError(err)")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("ctx, request, err = a." + hookRunnerName("Post", method, "Request") + "(ctx, info, request)")
	ctx.line("if err != nil {")
	ctx.line("err = a.mapError(err)")
	ctx.line("receiveErr = err")
	ctx.line("var zero " + requestType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	ctx.line("if !yield(request, nil) {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("}")
	ctx = newMappingRenderContext(g, true, "err")
	ctx.line("responses, err := a.service." + method.Name + "(ctx, requests)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("if responses != nil {")
	ctx.line("for response, err := range responses {")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("response, err = a." + hookRunnerName("Pre", method, "Response") + "(ctx, info, response)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response", mappingDirectionToProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("responseProto, err = a." + hookRunnerName("Post", method, "Response") + "(ctx, info, responseProto)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("if err := stream.Send(responseProto); err != nil {")
	ctx.line("return err")
	ctx.line("}")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return receiveErr")
	return nil
}

func generateConnectClientMethod(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	signature, err := generateServiceMethodSignature(g, method)
	if err != nil {
		return err
	}

	g.P("func (c *", service.ConnectClientName, ") ", signature, " {")
	switch method.StreamType {
	case ServiceStreamTypeUnary:
		err = generateConnectClientUnaryMethodBody(g, service, method)
	case ServiceStreamTypeServerStreaming:
		err = generateConnectClientServerStreamingMethodBody(g, service, method)
	case ServiceStreamTypeClientStreaming:
		err = generateConnectClientClientStreamingMethodBody(g, service, method)
	case ServiceStreamTypeBidiStreaming:
		err = generateConnectClientBidiStreamingMethodBody(g, service, method)
	default:
		err = fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
	if err != nil {
		return err
	}
	g.P("}")
	g.P()
	return nil
}

func generateConnectClientUnaryMethodBody(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, serviceClientErrorReturn(g, method))
	generateServiceClientZeroValue(ctx, g, method)
	if method.InlineRequest != nil {
		ctx.line("ctx, request := " + method.InlineRequest.FromInlineName + "(ctx, " + serviceInlineFieldNames(method.InlineRequest.Fields) + ")")
	}
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request", mappingDirectionToProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	responseProto := "responseProto"
	if serviceResponseIsEmpty(method) {
		responseProto = "_"
	}
	ctx.line(responseProto + ", err := c.client." + serviceNativeMethodName(method) + "(ctx, " + generateConnectSymbol(g, "NewRequest") + "(requestProto))")
	ctx.line("if err != nil {")
	ctx.line("return " + serviceClientMappedErrorReturn(g, method))
	ctx.line("}")
	if serviceResponseIsEmpty(method) {
		ctx.line("return nil")
		return nil
	}
	if err := generateServiceMappedAssignment(ctx, "response", method.Response.FromProto, "responseProto.Msg", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	if method.InlineResponse != nil {
		ctx.line("return " + method.InlineResponse.ToInlineName + "(response, nil)")
		return nil
	}
	ctx.line("return response, nil")
	_ = service
	return nil
}

func generateConnectClientServerStreamingMethodBody(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	responseType, err := generateType(g, method.Response.Type)
	if err != nil {
		return fmt.Errorf("response type: %w", err)
	}
	ctx := newMappingRenderContext(g, true, "nil, err")
	if method.InlineRequest != nil {
		ctx.line("ctx, request := " + method.InlineRequest.FromInlineName + "(ctx, " + serviceInlineFieldNames(method.InlineRequest.Fields) + ")")
	}
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request", mappingDirectionToProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("responses := func(yield func(" + responseType + ", error) bool) {")
	ctx.line("ctx, cancel := " + generateContextWithCancel(g) + "(ctx)")
	ctx.line("defer cancel()")
	ctx.line("stream, err := c.client." + serviceNativeMethodName(method) + "(ctx, " + generateConnectSymbol(g, "NewRequest") + "(requestProto))")
	ctx.line("if err != nil {")
	ctx.line("var zero " + responseType)
	ctx.line("yield(zero, c.mapError(err))")
	ctx.line("return")
	ctx.line("}")
	ctx.line("defer stream.Close()")
	ctx.line("for stream.Receive() {")
	seqCtx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2YieldErrorLines(responseType))
	if err := generateServiceMappedAssignment(seqCtx, "response", method.Response.FromProto, "stream.Msg()", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("if !yield(response, nil) {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("if err := stream.Err(); err != nil {")
	ctx.line("var zero " + responseType)
	ctx.line("yield(zero, c.mapError(err))")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return responses, nil")
	_ = service
	return nil
}

func generateConnectClientClientStreamingMethodBody(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, serviceClientErrorReturn(g, method))
	generateServiceClientZeroValue(ctx, g, method)
	ctx.line("stream := c.client." + serviceNativeMethodName(method) + "(ctx)")
	ctx.line("for request, err := range requests {")
	ctx.line("if err != nil {")
	ctx.line("return " + serviceClientErrorReturn(g, method))
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request", mappingDirectionToProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("if err := stream.Send(requestProto); err != nil {")
	ctx.line("return " + serviceClientMappedErrorReturn(g, method))
	ctx.line("}")
	ctx.line("}")
	ctx.line("responseProto, err := stream.CloseAndReceive()")
	ctx.line("if err != nil {")
	ctx.line("return " + serviceClientMappedErrorReturn(g, method))
	ctx.line("}")
	if serviceResponseIsEmpty(method) {
		ctx.line("return nil")
		return nil
	}
	if err := generateServiceMappedAssignment(ctx, "response", method.Response.FromProto, "responseProto.Msg", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	if method.InlineResponse != nil {
		ctx.line("return " + method.InlineResponse.ToInlineName + "(response, nil)")
		return nil
	}
	ctx.line("return response, nil")
	_ = service
	return nil
}

func generateConnectClientBidiStreamingMethodBody(g *generatedFile, service ServicePlan, method ServiceMethodPlan) error {
	_, responseType, err := generateServiceMethodTypes(g, method)
	if err != nil {
		return err
	}
	ctx := newMappingRenderContext(g, true, "nil, err")
	ctx.line("responses := func(yield func(" + responseType + ", error) bool) {")
	ctx.line("ctx, cancel := " + generateContextWithCancel(g) + "(ctx)")
	ctx.line("defer cancel()")
	ctx.line("stream := c.client." + serviceNativeMethodName(method) + "(ctx)")
	ctx.line("sendErr := make(chan error, 1)")
	ctx.line("go func() {")
	ctx.line("for request, err := range requests {")
	ctx.line("if err != nil {")
	ctx.line("sendErr <- err")
	ctx.line("return")
	ctx.line("}")
	sendCtx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2SendErrorLines())
	if err := generateServiceMappedAssignment(sendCtx, "requestProto", method.Request.ToProto, "request", mappingDirectionToProto); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("if err := stream.Send(requestProto); err != nil {")
	ctx.line("sendErr <- c.mapError(err)")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("sendErr <- c.mapError(stream.CloseRequest())")
	ctx.line("}()")
	ctx.line("for {")
	ctx.line("responseProto, err := stream.Receive()")
	ctx.line("if err != nil {")
	ctx.line("if " + generateErrorsSymbol(g, "Is") + "(err, " + generateIOSymbol(g, "EOF") + ") {")
	ctx.line("break")
	ctx.line("}")
	ctx.line("var zero " + responseType)
	ctx.line("yield(zero, c.mapError(err))")
	ctx.line("return")
	ctx.line("}")
	recvCtx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2YieldErrorLines(responseType))
	if err := generateServiceMappedAssignment(recvCtx, "response", method.Response.FromProto, "responseProto", mappingDirectionFromProto); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("if !yield(response, nil) {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("if err := <-sendErr; err != nil {")
	ctx.line("var zero " + responseType)
	ctx.line("yield(zero, err)")
	ctx.line("return")
	ctx.line("}")
	ctx.line("if err := stream.CloseResponse(); err != nil {")
	ctx.line("var zero " + responseType)
	ctx.line("yield(zero, c.mapError(err))")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return responses, nil")
	_ = service
	return nil
}

func serviceClientErrorReturn(g *generatedFile, method ServiceMethodPlan) string {
	return serviceClientErrorReturnExpr(method, "err")
}

func serviceClientMappedErrorReturn(g *generatedFile, method ServiceMethodPlan) string {
	return serviceClientErrorReturnExpr(method, "c.mapError(err)")
}

func serviceClientErrorReturnExpr(method ServiceMethodPlan, errExpr string) string {
	if serviceResponseIsEmpty(method) {
		return errExpr
	}
	if method.InlineResponse != nil {
		return method.InlineResponse.ToInlineName + "(zero, " + errExpr + ")"
	}
	switch method.StreamType {
	case ServiceStreamTypeServerStreaming, ServiceStreamTypeBidiStreaming:
		return "nil, " + errExpr
	default:
		return "zero, " + errExpr
	}
}

func serviceMethodCall(method ServiceMethodPlan, receiver, ctxExpr, requestExpr string) string {
	if method.InlineRequest != nil {
		return receiver + "." + method.Name + "(" + method.InlineRequest.ToInlineName + "(" + ctxExpr + ", " + requestExpr + "))"
	}
	return receiver + "." + method.Name + "(" + ctxExpr + ", " + requestExpr + ")"
}

func serviceNativeMethodName(method ServiceMethodPlan) string {
	if method.ProtoGoName != "" {
		return method.ProtoGoName
	}
	return method.Name
}

func serviceRPCInfoExpr(g *generatedFile, service ServicePlan, method ServiceMethodPlan) string {
	return generateTegoSymbol(g, "RPCInfo") + "{" +
		"Service: " + fmt.Sprintf("%q", string(service.ProtoName)) + ", " +
		"Method: " + fmt.Sprintf("%q", serviceNativeMethodName(method)) + ", " +
		"Procedure: " + fmt.Sprintf("%q", method.Procedure) +
		"}"
}

func serviceInlineFieldNames(fields []ServiceInlineFieldPlan) string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return strings.Join(names, ", ")
}

func serviceSeq2YieldErrorLines(valueType string) []string {
	return []string{
		"var zero " + valueType,
		"yield(zero, err)",
		"return",
	}
}

func serviceSeq2ReceiveErrorLines(valueType string) []string {
	return []string{
		"receiveErr = err",
		"var zero " + valueType,
		"yield(zero, err)",
		"return",
	}
}

func serviceSeq2SendErrorLines() []string {
	return []string{
		"sendErr <- err",
		"return",
	}
}

func generateServiceClientZeroValue(ctx *mappingRenderContext, g *generatedFile, method ServiceMethodPlan) {
	if serviceResponseIsEmpty(method) {
		return
	}
	switch method.StreamType {
	case ServiceStreamTypeUnary, ServiceStreamTypeClientStreaming:
		responseType, err := generateType(g, method.Response.Type)
		if err == nil {
			ctx.line("var zero " + responseType)
		}
	}
}

func generateServiceMappedAssignment(
	ctx *mappingRenderContext,
	name string,
	plan MappingValuePlan,
	source string,
	direction mappingDirection,
) error {
	var expr string
	if err := ctx.withDirection(direction, func() error {
		var err error
		expr, err = ctx.renderValueWithTempNameHint(name, plan, source)
		return err
	}); err != nil {
		return err
	}
	if expr == name {
		return nil
	}
	ctx.line(name + " := " + expr)
	return nil
}

func generateServiceMethodProtoTypes(g *generatedFile, method ServiceMethodPlan) (string, string, error) {
	request, err := generateType(g, method.Request.ProtoType)
	if err != nil {
		return "", "", fmt.Errorf("request proto type: %w", err)
	}

	response, err := generateType(g, method.Response.ProtoType)
	if err != nil {
		return "", "", fmt.Errorf("response proto type: %w", err)
	}

	return request, response, nil
}

func generateGRPCStreamType(g *generatedFile, name string, types ...TypePlan) (string, error) {
	args := make([]GoTypeRef, 0, len(types))
	for _, typ := range types {
		arg, err := protoMessageTypeArg(typ)
		if err != nil {
			return "", err
		}
		args = append(args, arg)
	}
	return generateNamedType(g, GoTypeRef{ImportPath: grpcImportPath, Name: name, Args: args}), nil
}

func generateConnectMessageType(g *generatedFile, name string, types ...TypePlan) (string, error) {
	args := make([]GoTypeRef, 0, len(types))
	for _, typ := range types {
		arg, err := protoMessageTypeArg(typ)
		if err != nil {
			return "", err
		}
		args = append(args, arg)
	}
	return "*" + generateNamedType(g, GoTypeRef{ImportPath: connectImportPath, Name: name, Args: args}), nil
}

func protoMessageTypeArg(plan TypePlan) (GoTypeRef, error) {
	if plan.Kind == TypeKindPointer && plan.Elem != nil && plan.Elem.Kind == TypeKindExternal {
		return plan.Elem.Ref, nil
	}
	return GoTypeRef{}, fmt.Errorf("RPC proto type argument must be a proto message pointer")
}

func generateProtoServiceType(g *generatedFile, service ServicePlan, prefix, suffix string) string {
	ref := service.ProtoRef
	ref.Name = prefix + ref.Name + suffix
	return generateNamedType(g, ref)
}

func generateProtoServiceSymbol(g *generatedFile, service ServicePlan, prefix, suffix string) string {
	return generateSymbol(g, GoSymbolRef{
		ImportPath: service.ProtoRef.ImportPath,
		Name:       prefix + service.ProtoRef.Name + suffix,
	})
}

func generateConnectServiceType(g *generatedFile, service ServicePlan, prefix, suffix string) string {
	ref := service.ConnectRef
	ref.Name = prefix + ref.Name + suffix
	return generateNamedType(g, ref)
}

func generateConnectServiceSymbol(g *generatedFile, service ServicePlan, prefix, suffix string) string {
	return generateSymbol(g, GoSymbolRef{
		ImportPath: service.ConnectRef.ImportPath,
		Name:       prefix + service.ConnectRef.Name + suffix,
	})
}

func generateGRPCType(g *generatedFile, name string) string {
	return generateNamedType(g, GoTypeRef{ImportPath: grpcImportPath, Name: name})
}

func generateConnectSymbol(g *generatedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: connectImportPath, Name: name})
}

func generateFmtSymbol(g *generatedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: fmtImportPath, Name: name})
}

func generateErrorsSymbol(g *generatedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: errorsImportPath, Name: name})
}

func generateTegoSymbol(g *generatedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: tegoImportPath, Name: name})
}

func generateHTTPType(g *generatedFile, name string) string {
	return generateNamedType(g, GoTypeRef{ImportPath: httpImportPath, Name: name})
}

func generateIOSymbol(g *generatedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: ioImportPath, Name: name})
}
