package tego

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
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

func generateService(g *protogen.GeneratedFile, service ServicePlan, rpc RPCOptions) error {
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

func generateServiceMethodSignature(g *protogen.GeneratedFile, method ServiceMethodPlan) (string, error) {
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

func generateServiceMethodTypes(g *protogen.GeneratedFile, method ServiceMethodPlan) (string, string, error) {
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

func generateContextType(g *protogen.GeneratedFile) string {
	return generateNamedType(g, GoTypeRef{ImportPath: contextImportPath, Name: "Context"})
}

func generateSeq2Type(g *protogen.GeneratedFile, value string) string {
	return generateNamedType(g, GoTypeRef{ImportPath: iterImportPath, Name: "Seq2"}) + "[" + value + ", error]"
}

func serviceResponseIsEmpty(method ServiceMethodPlan) bool {
	return method.Response.Type.Kind == TypeKindEmptyStruct
}

func generateServiceRequestInlineHelper(g *protogen.GeneratedFile, helper ServiceInlineHelperPlan) error {
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

func generateServiceResponseInlineHelper(g *protogen.GeneratedFile, helper ServiceInlineHelperPlan) error {
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

func generateServiceInlineFieldParameters(g *protogen.GeneratedFile, fields []ServiceInlineFieldPlan) (string, error) {
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

func generateServiceInlineFieldTypes(g *protogen.GeneratedFile, fields []ServiceInlineFieldPlan) (string, error) {
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

func generateServiceInlineErrorReturn(g *protogen.GeneratedFile, fields []ServiceInlineFieldPlan) error {
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

func generateUnimplementedService(g *protogen.GeneratedFile, service ServicePlan) error {
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
	g *protogen.GeneratedFile,
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
	g *protogen.GeneratedFile,
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

func generateGRPCService(g *protogen.GeneratedFile, service ServicePlan) error {
	g.P("func ", service.GRPCRegisterName, "(", "registrar ", generateGRPCType(g, "ServiceRegistrar"), ", service ", service.Name, ", opts ...", generateTegoSymbol(g, "GRPCServerOption"), ") {")
	g.P("\t", generateProtoServiceSymbol(g, service, "Register", "Server"), "(registrar, ", service.GRPCNewServerName, "(service, opts...))")
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

func generateGRPCAdapter(g *protogen.GeneratedFile, service ServicePlan) {
	g.P("type ", service.GRPCAdapterName, " struct {")
	g.P("\tservice ", service.Name)
	g.P("\terrorMapper ", generateTegoSymbol(g, "ErrorMapper"))
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
}

func generateGRPCServerMethod(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
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

func generateGRPCAdapterMethod(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
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
	g *protogen.GeneratedFile,
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
	g *protogen.GeneratedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto"); err != nil {
		return fmt.Errorf("request: %w", err)
	}

	call := serviceMethodCall(method, "a.service", "ctx", "request")
	if serviceResponseIsEmpty(method) {
		ctx.line("if err := " + call + "; err != nil {")
		ctx.line("return nil, a.mapError(err)")
		ctx.line("}")
		responseExpr, err := serviceEmptyStructConverterReturnExpr(g, method.Response.ToProto)
		if err != nil {
			return fmt.Errorf("response: %w", err)
		}
		ctx.line("return " + responseExpr + ", nil")
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
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("return responseProto, nil")
	return nil
}

func generateGRPCAdapterServerStreamingMethodBody(
	g *protogen.GeneratedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	ctx := newMappingRenderContext(g, true, "err")
	ctx.line("ctx := stream.Context()")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
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
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("if err := stream.Send(responseProto); err != nil {")
	ctx.line("return err")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return nil")
	return nil
}

func generateGRPCAdapterClientStreamingMethodBody(
	g *protogen.GeneratedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	requestType, err := generateType(g, method.Request.Type)
	if err != nil {
		return fmt.Errorf("request type: %w", err)
	}
	ctx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2ReceiveErrorLines(requestType), "requestProto")
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
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("if !yield(request, nil) {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("}")

	ctx = newMappingRenderContext(g, true, "err")
	if serviceResponseIsEmpty(method) {
		ctx.line("if err := a.service." + method.Name + "(stream.Context(), requests); err != nil {")
		ctx.line("return a.mapError(err)")
		ctx.line("}")
		ctx.line("if receiveErr != nil {")
		ctx.line("return receiveErr")
		ctx.line("}")
		responseExpr, err := serviceEmptyStructConverterReturnExpr(g, method.Response.ToProto)
		if err != nil {
			return fmt.Errorf("response: %w", err)
		}
		ctx.line("return stream.SendAndClose(" + responseExpr + ")")
		return nil
	}

	call := "a.service." + method.Name + "(stream.Context(), requests)"
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
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("return stream.SendAndClose(responseProto)")
	return nil
}

func generateGRPCAdapterBidiStreamingMethodBody(
	g *protogen.GeneratedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	requestType, err := generateType(g, method.Request.Type)
	if err != nil {
		return fmt.Errorf("request type: %w", err)
	}
	ctx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2ReceiveErrorLines(requestType), "requestProto")
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
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("if !yield(request, nil) {")
	ctx.line("return")
	ctx.line("}")
	ctx.line("}")
	ctx.line("}")
	ctx = newMappingRenderContext(g, true, "err")
	ctx.line("responses, err := a.service." + method.Name + "(stream.Context(), requests)")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	ctx.line("if responses != nil {")
	ctx.line("for response, err := range responses {")
	ctx.line("if err != nil {")
	ctx.line("return a.mapError(err)")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("if err := stream.Send(responseProto); err != nil {")
	ctx.line("return err")
	ctx.line("}")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return receiveErr")
	return nil
}

func generateGRPCClientMethod(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
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

func generateGRPCClientUnaryMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, serviceClientErrorReturn(g, method))
	generateServiceClientZeroValue(ctx, g, method)
	if method.InlineRequest != nil {
		ctx.line("ctx, request := " + method.InlineRequest.FromInlineName + "(ctx, " + serviceInlineFieldNames(method.InlineRequest.Fields) + ")")
	}
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request"); err != nil {
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
	if err := generateServiceMappedAssignment(ctx, "response", method.Response.FromProto, "responseProto"); err != nil {
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

func generateGRPCClientServerStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	responseType, err := generateType(g, method.Response.Type)
	if err != nil {
		return fmt.Errorf("response type: %w", err)
	}
	ctx := newMappingRenderContext(g, true, "nil, err")
	if method.InlineRequest != nil {
		ctx.line("ctx, request := " + method.InlineRequest.FromInlineName + "(ctx, " + serviceInlineFieldNames(method.InlineRequest.Fields) + ")")
	}
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("stream, err := c.client." + serviceNativeMethodName(method) + "(ctx, requestProto)")
	ctx.line("if err != nil {")
	ctx.line("return nil, c.mapError(err)")
	ctx.line("}")
	ctx.line("responses := func(yield func(" + responseType + ", error) bool) {")
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
	if err := generateServiceMappedAssignment(seqCtx, "response", method.Response.FromProto, "responseProto"); err != nil {
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

func generateGRPCClientClientStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
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
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request"); err != nil {
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
	if err := generateServiceMappedAssignment(ctx, "response", method.Response.FromProto, "responseProto"); err != nil {
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

func generateGRPCClientBidiStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	_, responseType, err := generateServiceMethodTypes(g, method)
	if err != nil {
		return err
	}
	ctx := newMappingRenderContext(g, true, "nil, err")
	ctx.line("stream, err := c.client." + serviceNativeMethodName(method) + "(ctx)")
	ctx.line("if err != nil {")
	ctx.line("return nil, c.mapError(err)")
	ctx.line("}")
	ctx.line("responses := func(yield func(" + responseType + ", error) bool) {")
	ctx.line("sendErr := make(chan error, 1)")
	ctx.line("go func() {")
	ctx.line("for request, err := range requests {")
	ctx.line("if err != nil {")
	ctx.line("sendErr <- err")
	ctx.line("return")
	ctx.line("}")
	sendCtx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2SendErrorLines(), "requestProto")
	if err := generateServiceMappedAssignment(sendCtx, "requestProto", method.Request.ToProto, "request"); err != nil {
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
	if err := generateServiceMappedAssignment(recvCtx, "response", method.Response.FromProto, "responseProto"); err != nil {
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

func generateConnectService(g *protogen.GeneratedFile, service ServicePlan) error {
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

func generateConnectAdapter(g *protogen.GeneratedFile, service ServicePlan) {
	g.P("type ", service.ConnectAdapterName, " struct {")
	g.P("\tservice ", service.Name)
	g.P("\terrorMapper ", generateTegoSymbol(g, "ErrorMapper"))
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
}

func generateConnectHandlerMethod(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
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

func generateConnectAdapterMethod(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
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

func generateConnectHandlerMethodSignature(g *protogen.GeneratedFile, name string, method ServiceMethodPlan) (string, error) {
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
	g *protogen.GeneratedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto.Msg"); err != nil {
		return fmt.Errorf("request: %w", err)
	}

	call := serviceMethodCall(method, "a.service", "ctx", "request")
	if serviceResponseIsEmpty(method) {
		ctx.line("if err := " + call + "; err != nil {")
		ctx.line("return nil, a.mapError(err)")
		ctx.line("}")
		responseExpr, err := serviceEmptyStructConverterReturnExpr(g, method.Response.ToProto)
		if err != nil {
			return fmt.Errorf("response: %w", err)
		}
		ctx.line("return " + generateConnectSymbol(g, "NewResponse") + "(" + responseExpr + "), nil")
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
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("return " + generateConnectSymbol(g, "NewResponse") + "(responseProto), nil")
	return nil
}

func generateConnectAdapterServerStreamingMethodBody(
	g *protogen.GeneratedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	ctx := newMappingRenderContext(g, true, "err")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto.Msg"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
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
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("if err := stream.Send(responseProto); err != nil {")
	ctx.line("return err")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return nil")
	return nil
}

func generateConnectAdapterClientStreamingMethodBody(
	g *protogen.GeneratedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	requestType, err := generateType(g, method.Request.Type)
	if err != nil {
		return fmt.Errorf("request type: %w", err)
	}
	ctx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2ReceiveErrorLines(requestType), "requestProto")
	ctx.line("var receiveErr error")
	ctx.line("requests := func(yield func(" + requestType + ", error) bool) {")
	ctx.line("for stream.Receive() {")
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "stream.Msg()"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
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
		responseExpr, err := serviceEmptyStructConverterReturnExpr(g, method.Response.ToProto)
		if err != nil {
			return fmt.Errorf("response: %w", err)
		}
		ctx.line("return " + generateConnectSymbol(g, "NewResponse") + "(" + responseExpr + "), nil")
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
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("return " + generateConnectSymbol(g, "NewResponse") + "(responseProto), nil")
	return nil
}

func generateConnectAdapterBidiStreamingMethodBody(
	g *protogen.GeneratedFile,
	service ServicePlan,
	method ServiceMethodPlan,
) error {
	requestType, err := generateType(g, method.Request.Type)
	if err != nil {
		return fmt.Errorf("request type: %w", err)
	}
	ctx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2ReceiveErrorLines(requestType), "requestProto")
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
	if err := generateServiceMappedAssignment(ctx, "request", method.Request.FromProto, "requestProto"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
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
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("if err := stream.Send(responseProto); err != nil {")
	ctx.line("return err")
	ctx.line("}")
	ctx.line("}")
	ctx.line("}")
	ctx.line("return receiveErr")
	return nil
}

func generateConnectClientMethod(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
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

func generateConnectClientUnaryMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, serviceClientErrorReturn(g, method))
	generateServiceClientZeroValue(ctx, g, method)
	if method.InlineRequest != nil {
		ctx.line("ctx, request := " + method.InlineRequest.FromInlineName + "(ctx, " + serviceInlineFieldNames(method.InlineRequest.Fields) + ")")
	}
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request"); err != nil {
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
	if err := generateServiceMappedAssignment(ctx, "response", method.Response.FromProto, "responseProto.Msg"); err != nil {
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

func generateConnectClientServerStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	responseType, err := generateType(g, method.Response.Type)
	if err != nil {
		return fmt.Errorf("response type: %w", err)
	}
	ctx := newMappingRenderContext(g, true, "nil, err")
	if method.InlineRequest != nil {
		ctx.line("ctx, request := " + method.InlineRequest.FromInlineName + "(ctx, " + serviceInlineFieldNames(method.InlineRequest.Fields) + ")")
	}
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("stream, err := c.client." + serviceNativeMethodName(method) + "(ctx, " + generateConnectSymbol(g, "NewRequest") + "(requestProto))")
	ctx.line("if err != nil {")
	ctx.line("return nil, c.mapError(err)")
	ctx.line("}")
	ctx.line("responses := func(yield func(" + responseType + ", error) bool) {")
	ctx.line("defer stream.Close()")
	ctx.line("for stream.Receive() {")
	seqCtx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2YieldErrorLines(responseType))
	if err := generateServiceMappedAssignment(seqCtx, "response", method.Response.FromProto, "stream.Msg()"); err != nil {
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

func generateConnectClientClientStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, serviceClientErrorReturn(g, method))
	generateServiceClientZeroValue(ctx, g, method)
	ctx.line("stream := c.client." + serviceNativeMethodName(method) + "(ctx)")
	ctx.line("for request, err := range requests {")
	ctx.line("if err != nil {")
	ctx.line("return " + serviceClientErrorReturn(g, method))
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request"); err != nil {
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
	if err := generateServiceMappedAssignment(ctx, "response", method.Response.FromProto, "responseProto.Msg"); err != nil {
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

func generateConnectClientBidiStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	_, responseType, err := generateServiceMethodTypes(g, method)
	if err != nil {
		return err
	}
	ctx := newMappingRenderContext(g, true, "nil, err")
	ctx.line("stream := c.client." + serviceNativeMethodName(method) + "(ctx)")
	ctx.line("responses := func(yield func(" + responseType + ", error) bool) {")
	ctx.line("sendErr := make(chan error, 1)")
	ctx.line("go func() {")
	ctx.line("for request, err := range requests {")
	ctx.line("if err != nil {")
	ctx.line("sendErr <- err")
	ctx.line("return")
	ctx.line("}")
	sendCtx := newMappingRenderContextWithErrorLines(g, true, serviceSeq2SendErrorLines(), "requestProto")
	if err := generateServiceMappedAssignment(sendCtx, "requestProto", method.Request.ToProto, "request"); err != nil {
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
	if err := generateServiceMappedAssignment(recvCtx, "response", method.Response.FromProto, "responseProto"); err != nil {
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

func serviceClientErrorReturn(g *protogen.GeneratedFile, method ServiceMethodPlan) string {
	return serviceClientErrorReturnExpr(method, "err")
}

func serviceClientMappedErrorReturn(g *protogen.GeneratedFile, method ServiceMethodPlan) string {
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

func generateServiceClientZeroValue(ctx *mappingRenderContext, g *protogen.GeneratedFile, method ServiceMethodPlan) {
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
) error {
	expr, err := ctx.renderValueWithTempNameHint(name, plan, source)
	if err != nil {
		return err
	}
	if expr == name {
		return nil
	}
	ctx.line(name + " := " + expr)
	return nil
}

func serviceEmptyStructConverterReturnExpr(g *protogen.GeneratedFile, plan MappingValuePlan) (string, error) {
	if isEmptypbEmptyPointer(plan.Source) && plan.Target.Kind == TypeKindEmptyStruct {
		return "struct{}{}", nil
	}
	if plan.Source.Kind == TypeKindEmptyStruct && isEmptypbEmptyPointer(plan.Target) {
		return newValueExpr(g, plan.Target)
	}
	return "", fmt.Errorf("unsupported empty struct mapping")
}

func generateServiceMethodProtoTypes(g *protogen.GeneratedFile, method ServiceMethodPlan) (string, string, error) {
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

func generateGRPCStreamType(g *protogen.GeneratedFile, name string, types ...TypePlan) (string, error) {
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

func generateConnectMessageType(g *protogen.GeneratedFile, name string, types ...TypePlan) (string, error) {
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

func generateProtoServiceType(g *protogen.GeneratedFile, service ServicePlan, prefix, suffix string) string {
	ref := service.ProtoRef
	ref.Name = prefix + ref.Name + suffix
	return generateNamedType(g, ref)
}

func generateProtoServiceSymbol(g *protogen.GeneratedFile, service ServicePlan, prefix, suffix string) string {
	return generateSymbol(g, GoSymbolRef{
		ImportPath: service.ProtoRef.ImportPath,
		Name:       prefix + service.ProtoRef.Name + suffix,
	})
}

func generateConnectServiceType(g *protogen.GeneratedFile, service ServicePlan, prefix, suffix string) string {
	ref := service.ConnectRef
	ref.Name = prefix + ref.Name + suffix
	return generateNamedType(g, ref)
}

func generateConnectServiceSymbol(g *protogen.GeneratedFile, service ServicePlan, prefix, suffix string) string {
	return generateSymbol(g, GoSymbolRef{
		ImportPath: service.ConnectRef.ImportPath,
		Name:       prefix + service.ConnectRef.Name + suffix,
	})
}

func generateGRPCType(g *protogen.GeneratedFile, name string) string {
	return generateNamedType(g, GoTypeRef{ImportPath: grpcImportPath, Name: name})
}

func generateConnectType(g *protogen.GeneratedFile, name string) string {
	return generateNamedType(g, GoTypeRef{ImportPath: connectImportPath, Name: name})
}

func generateConnectSymbol(g *protogen.GeneratedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: connectImportPath, Name: name})
}

func generateFmtSymbol(g *protogen.GeneratedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: fmtImportPath, Name: name})
}

func generateErrorsSymbol(g *protogen.GeneratedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: errorsImportPath, Name: name})
}

func generateTegoSymbol(g *protogen.GeneratedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: tegoImportPath, Name: name})
}

func generateHTTPType(g *protogen.GeneratedFile, name string) string {
	return generateNamedType(g, GoTypeRef{ImportPath: httpImportPath, Name: name})
}

func generateIOSymbol(g *protogen.GeneratedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: ioImportPath, Name: name})
}
