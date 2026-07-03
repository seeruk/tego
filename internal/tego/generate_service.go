package tego

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
)

const (
	connectImportPath     = "connectrpc.com/connect"
	contextImportPath     = "context"
	grpcImportPath        = "google.golang.org/grpc"
	httpImportPath        = "net/http"
	metadataImportPath    = "google.golang.org/grpc/metadata"
	tegoconnectImportPath = "github.com/seeruk/tego/rpc/tegoconnect"
	tegogrpcImportPath    = "github.com/seeruk/tego/rpc/tegogrpc"
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

	generateComment(g, "", service.ClientName+" is the client interface for "+service.Name+".")
	g.P("type ", service.ClientName, " interface {")
	for _, method := range service.Methods {
		signature, err := generateServiceClientMethodSignature(g, method)
		if err != nil {
			return fmt.Errorf("service client %s method %s: %w", service.ProtoName, method.ProtoName, err)
		}
		generateComment(g, "\t", method.Comment)
		g.P("\t", signature)
	}
	g.P("}")
	g.P()

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
		return fmt.Sprintf(
			"%s(%s, *%s[%s]) (*%s[%s], error)",
			method.Name,
			contextType,
			generateTegoType(g, "Request"),
			request,
			generateTegoType(g, "Response"),
			response,
		), nil
	case ServiceStreamTypeServerStreaming:
		return fmt.Sprintf(
			"%s(%s, *%s[%s], *%s[%s]) error",
			method.Name,
			contextType,
			generateTegoType(g, "Request"),
			request,
			generateTegoType(g, "ServerSendStream"),
			response,
		), nil
	case ServiceStreamTypeClientStreaming:
		return fmt.Sprintf(
			"%s(%s, *%s[%s]) (*%s[%s], error)",
			method.Name,
			contextType,
			generateTegoType(g, "ServerRecvStream"),
			request,
			generateTegoType(g, "Response"),
			response,
		), nil
	case ServiceStreamTypeBidiStreaming:
		return fmt.Sprintf(
			"%s(%s, *%s[%s, %s]) error",
			method.Name,
			contextType,
			generateTegoType(g, "ServerBidiStream"),
			request,
			response,
		), nil
	default:
		return "", fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
}

func generateServiceClientMethodSignature(g *protogen.GeneratedFile, method ServiceMethodPlan) (string, error) {
	request, response, err := generateServiceMethodTypes(g, method)
	if err != nil {
		return "", err
	}

	contextType := generateContextType(g)
	switch method.StreamType {
	case ServiceStreamTypeUnary:
		return fmt.Sprintf(
			"%s(%s, *%s[%s]) (*%s[%s], error)",
			method.Name,
			contextType,
			generateTegoType(g, "Request"),
			request,
			generateTegoType(g, "Response"),
			response,
		), nil
	case ServiceStreamTypeServerStreaming:
		return fmt.Sprintf(
			"%s(%s, *%s[%s]) (*%s[%s], error)",
			method.Name,
			contextType,
			generateTegoType(g, "Request"),
			request,
			generateTegoType(g, "ClientRecvStream"),
			response,
		), nil
	case ServiceStreamTypeClientStreaming:
		return fmt.Sprintf(
			"%s(%s, ...%s) (*%s[%s, %s], error)",
			method.Name,
			contextType,
			generateTegoType(g, "CallOption"),
			generateTegoType(g, "ClientSendStream"),
			request,
			response,
		), nil
	case ServiceStreamTypeBidiStreaming:
		return fmt.Sprintf(
			"%s(%s, ...%s) (*%s[%s, %s], error)",
			method.Name,
			contextType,
			generateTegoType(g, "CallOption"),
			generateTegoType(g, "ClientBidiStream"),
			request,
			response,
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

func generateTegoType(g *protogen.GeneratedFile, name string) string {
	return generateNamedType(g, GoTypeRef{ImportPath: tegoImportPath, Name: name})
}

func generateGRPCService(g *protogen.GeneratedFile, service ServicePlan) error {
	g.P("func ", service.GRPCRegisterName, "(", "registrar ", generateGRPCType(g, "ServiceRegistrar"), ", service ", service.Name, ") {")
	g.P("\t", generateProtoServiceSymbol(g, service, "Register", "Server"), "(registrar, &", service.GRPCServerName, "{service: service})")
	g.P("}")
	g.P()

	g.P("type ", service.GRPCServerName, " struct {")
	g.P("\t", generateProtoServiceType(g, service, "Unimplemented", "Server"))
	g.P("\tservice ", service.Name)
	g.P("}")
	g.P()

	for _, method := range service.Methods {
		if err := generateGRPCServerMethod(g, service, method); err != nil {
			return fmt.Errorf("gRPC server method %s: %w", method.ProtoName, err)
		}
	}

	g.P("func ", service.GRPCNewClientName, "(client ", generateProtoServiceType(g, service, "", "Client"), ") ", service.ClientName, " {")
	g.P("\treturn &", service.GRPCClientName, "{client: client}")
	g.P("}")
	g.P()

	g.P("type ", service.GRPCClientName, " struct {")
	g.P("\tclient ", generateProtoServiceType(g, service, "", "Client"))
	g.P("}")
	g.P()

	for _, method := range service.Methods {
		if err := generateGRPCClientMethod(g, service, method); err != nil {
			return fmt.Errorf("gRPC client method %s: %w", method.ProtoName, err)
		}
	}

	return nil
}

func generateConnectService(g *protogen.GeneratedFile, service ServicePlan) error {
	g.P("func ", service.ConnectNewHandlerName, "(service ", service.Name, ", opts ...", generateConnectType(g, "HandlerOption"), ") (string, ", generateHTTPType(g, "Handler"), ") {")
	g.P("\treturn ", generateConnectServiceSymbol(g, service, "New", "Handler"), "(&", service.ConnectHandlerName, "{service: service}, opts...)")
	g.P("}")
	g.P()

	g.P("type ", service.ConnectHandlerName, " struct {")
	g.P("\tservice ", service.Name)
	g.P("}")
	g.P()

	for _, method := range service.Methods {
		if err := generateConnectHandlerMethod(g, service, method); err != nil {
			return fmt.Errorf("connect handler method %s: %w", method.ProtoName, err)
		}
	}

	g.P("func ", service.ConnectNewClientName, "(client ", generateConnectServiceType(g, service, "", "Client"), ") ", service.ClientName, " {")
	g.P("\treturn &", service.ConnectClientName, "{client: client}")
	g.P("}")
	g.P()

	g.P("type ", service.ConnectClientName, " struct {")
	g.P("\tclient ", generateConnectServiceType(g, service, "", "Client"))
	g.P("}")
	g.P()

	for _, method := range service.Methods {
		if err := generateConnectClientMethod(g, service, method); err != nil {
			return fmt.Errorf("connect client method %s: %w", method.ProtoName, err)
		}
	}

	return nil
}

func generateGRPCServerMethod(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	signature, err := generateGRPCServerMethodSignature(g, service, method)
	if err != nil {
		return err
	}

	g.P("func (s *", service.GRPCServerName, ") ", signature, " {")
	switch method.StreamType {
	case ServiceStreamTypeUnary:
		err = generateGRPCServerUnaryMethodBody(g, service, method)
	case ServiceStreamTypeServerStreaming:
		err = generateGRPCServerServerStreamingMethodBody(g, service, method)
	case ServiceStreamTypeClientStreaming:
		err = generateGRPCServerClientStreamingMethodBody(g, service, method)
	case ServiceStreamTypeBidiStreaming:
		err = generateGRPCServerBidiStreamingMethodBody(g, service, method)
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
	service ServicePlan,
	method ServiceMethodPlan,
) (string, error) {
	contextType := generateContextType(g)
	requestProto, responseProto, err := generateServiceMethodProtoTypes(g, method)
	if err != nil {
		return "", err
	}

	switch method.StreamType {
	case ServiceStreamTypeUnary:
		return fmt.Sprintf("%s(ctx %s, requestProto %s) (%s, error)", method.Name, contextType, requestProto, responseProto), nil
	case ServiceStreamTypeServerStreaming:
		stream, err := generateGRPCStreamType(g, "ServerStreamingServer", method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(requestProto %s, stream %s) error", method.Name, requestProto, stream), nil
	case ServiceStreamTypeClientStreaming:
		stream, err := generateGRPCStreamType(g, "ClientStreamingServer", method.Request.ProtoType, method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(stream %s) error", method.Name, stream), nil
	case ServiceStreamTypeBidiStreaming:
		stream, err := generateGRPCStreamType(g, "BidiStreamingServer", method.Request.ProtoType, method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(stream %s) error", method.Name, stream), nil
	default:
		return "", fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
}

func generateGRPCServerUnaryMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceMappedAssignment(ctx, "requestTego", method.Request.FromProto, "requestProto"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("response, err := s.service." + method.Name + "(ctx, " + generateTegogrpcSymbol(g, "NewRequest") + "(ctx, requestTego, " + generateGRPCServiceSpec(g, service, method) + "))")
	ctx.line("if err != nil {")
	ctx.line("return nil, err")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response.Message"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("if err := " + generateTegogrpcSymbol(g, "ApplyResponseMetadata") + "(ctx, response); err != nil {")
	ctx.line("return nil, err")
	ctx.line("}")
	ctx.line("return responseProto, nil")
	return nil
}

func generateGRPCServerServerStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "err")
	ctx.line("ctx := stream.Context()")
	if err := generateServiceMappedAssignment(ctx, "requestTego", method.Request.FromProto, "requestProto"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	if err := generateServiceConverterFunc(g, "responseToProto", method.Response.ToProto, "responseTego", "responseProto"); err != nil {
		return fmt.Errorf("response converter: %w", err)
	}
	ctx.line("tegoStream := " + generateTegogrpcSymbol(g, "NewServerSendStream") + "(stream, " + generateGRPCServiceSpec(g, service, method) + ", responseToProto)")
	ctx.line("if err := s.service." + method.Name + "(ctx, " + generateTegogrpcSymbol(g, "NewRequest") + "(ctx, requestTego, " + generateGRPCServiceSpec(g, service, method) + "), tegoStream); err != nil {")
	ctx.line("return err")
	ctx.line("}")
	ctx.line("stream.SetTrailer(" + generateTegogrpcSymbol(g, "MDFromMetadata") + "(tegoStream.ResponseTrailer()))")
	ctx.line("return nil")
	return nil
}

func generateGRPCServerClientStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "err")
	if err := generateServiceConverterFunc(g, "requestFromProto", method.Request.FromProto, "requestProto", "requestTego"); err != nil {
		return fmt.Errorf("request converter: %w", err)
	}
	ctx.line("tegoStream := " + generateTegogrpcSymbol(g, "NewServerRecvStream") + "(stream, " + generateGRPCServiceSpec(g, service, method) + ", requestFromProto)")
	ctx.line("response, err := s.service." + method.Name + "(stream.Context(), tegoStream)")
	ctx.line("if err != nil {")
	ctx.line("return err")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response.Message"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("if err := stream.SetHeader(" + generateTegogrpcSymbol(g, "MDFromMetadata") + "(response.Header())); err != nil {")
	ctx.line("return err")
	ctx.line("}")
	ctx.line("stream.SetTrailer(" + generateTegogrpcSymbol(g, "MDFromMetadata") + "(response.Trailer()))")
	ctx.line("return stream.SendAndClose(responseProto)")
	return nil
}

func generateGRPCServerBidiStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "err")
	if err := generateServiceConverterFunc(g, "requestFromProto", method.Request.FromProto, "requestProto", "requestTego"); err != nil {
		return fmt.Errorf("request converter: %w", err)
	}
	if err := generateServiceConverterFunc(g, "responseToProto", method.Response.ToProto, "responseTego", "responseProto"); err != nil {
		return fmt.Errorf("response converter: %w", err)
	}
	ctx.line("tegoStream := " + generateTegogrpcSymbol(g, "NewServerBidiStream") + "(stream, " + generateGRPCServiceSpec(g, service, method) + ", requestFromProto, responseToProto)")
	ctx.line("if err := s.service." + method.Name + "(stream.Context(), tegoStream); err != nil {")
	ctx.line("return err")
	ctx.line("}")
	ctx.line("stream.SetTrailer(" + generateTegogrpcSymbol(g, "MDFromMetadata") + "(tegoStream.ResponseTrailer()))")
	ctx.line("return nil")
	return nil
}

func generateGRPCClientMethod(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	signature, err := generateClientMethodSignature(g, method)
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

func generateClientMethodSignature(g *protogen.GeneratedFile, method ServiceMethodPlan) (string, error) {
	contextType := generateContextType(g)
	request, response, err := generateServiceMethodTypes(g, method)
	if err != nil {
		return "", err
	}

	switch method.StreamType {
	case ServiceStreamTypeUnary:
		return fmt.Sprintf("%s(ctx %s, request *%s[%s]) (*%s[%s], error)", method.Name, contextType, generateTegoType(g, "Request"), request, generateTegoType(g, "Response"), response), nil
	case ServiceStreamTypeServerStreaming:
		return fmt.Sprintf("%s(ctx %s, request *%s[%s]) (*%s[%s], error)", method.Name, contextType, generateTegoType(g, "Request"), request, generateTegoType(g, "ClientRecvStream"), response), nil
	case ServiceStreamTypeClientStreaming:
		return fmt.Sprintf("%s(ctx %s, opts ...%s) (*%s[%s, %s], error)", method.Name, contextType, generateTegoType(g, "CallOption"), generateTegoType(g, "ClientSendStream"), request, response), nil
	case ServiceStreamTypeBidiStreaming:
		return fmt.Sprintf("%s(ctx %s, opts ...%s) (*%s[%s, %s], error)", method.Name, contextType, generateTegoType(g, "CallOption"), generateTegoType(g, "ClientBidiStream"), request, response), nil
	default:
		return "", fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
}

func generateGRPCClientUnaryMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request.Message"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("ctx = " + generateTegogrpcSymbol(g, "NewOutgoingContext") + "(ctx, request.Header())")
	ctx.line("var header, trailer " + generateMetadataType(g, "MD"))
	ctx.line("responseProto, err := c.client." + method.Name + "(ctx, requestProto, " + generateGRPCSymbol(g, "Header") + "(&header), " + generateGRPCSymbol(g, "Trailer") + "(&trailer))")
	ctx.line("if err != nil {")
	ctx.line("return nil, err")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseTego", method.Response.FromProto, "responseProto"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("return " + generateTegogrpcSymbol(g, "NewResponse") + "(responseTego, header, trailer, responseProto), nil")
	return nil
}

func generateGRPCClientServerStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request.Message"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	if err := generateServiceConverterFunc(g, "responseFromProto", method.Response.FromProto, "responseProto", "responseTego"); err != nil {
		return fmt.Errorf("response converter: %w", err)
	}
	ctx.line("ctx = " + generateTegogrpcSymbol(g, "NewOutgoingContext") + "(ctx, request.Header())")
	ctx.line("stream, err := c.client." + method.Name + "(ctx, requestProto)")
	ctx.line("if err != nil {")
	ctx.line("return nil, err")
	ctx.line("}")
	ctx.line("return " + generateTegogrpcSymbol(g, "NewClientRecvStream") + "(stream, " + generateGRPCServiceSpec(g, service, method) + ", responseFromProto), nil")
	return nil
}

func generateGRPCClientClientStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceConverterFunc(g, "requestToProto", method.Request.ToProto, "requestTego", "requestProto"); err != nil {
		return fmt.Errorf("request converter: %w", err)
	}
	if err := generateServiceConverterFunc(g, "responseFromProto", method.Response.FromProto, "responseProto", "responseTego"); err != nil {
		return fmt.Errorf("response converter: %w", err)
	}
	ctx.line("call := " + generateTegoSymbol(g, "NewCall") + "(opts...)")
	ctx.line("requestHeader := call.Header().Clone()")
	ctx.line("ctx = " + generateTegogrpcSymbol(g, "NewOutgoingContext") + "(ctx, requestHeader)")
	ctx.line("stream, err := c.client." + method.Name + "(ctx)")
	ctx.line("if err != nil {")
	ctx.line("return nil, err")
	ctx.line("}")
	ctx.line("return " + generateTegogrpcSymbol(g, "NewClientSendStream") + "(stream, " + generateGRPCServiceSpec(g, service, method) + ", requestToProto, responseFromProto, " + generateTegoSymbol(g, "WithStreamRequestHeader") + "(requestHeader)), nil")
	return nil
}

func generateGRPCClientBidiStreamingMethodBody(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceConverterFunc(g, "requestToProto", method.Request.ToProto, "requestTego", "requestProto"); err != nil {
		return fmt.Errorf("request converter: %w", err)
	}
	if err := generateServiceConverterFunc(g, "responseFromProto", method.Response.FromProto, "responseProto", "responseTego"); err != nil {
		return fmt.Errorf("response converter: %w", err)
	}
	ctx.line("call := " + generateTegoSymbol(g, "NewCall") + "(opts...)")
	ctx.line("requestHeader := call.Header().Clone()")
	ctx.line("ctx = " + generateTegogrpcSymbol(g, "NewOutgoingContext") + "(ctx, requestHeader)")
	ctx.line("stream, err := c.client." + method.Name + "(ctx)")
	ctx.line("if err != nil {")
	ctx.line("return nil, err")
	ctx.line("}")
	ctx.line("return " + generateTegogrpcSymbol(g, "NewClientBidiStream") + "(stream, " + generateGRPCServiceSpec(g, service, method) + ", requestToProto, responseFromProto, " + generateTegoSymbol(g, "WithStreamRequestHeader") + "(requestHeader)), nil")
	return nil
}

func generateConnectHandlerMethod(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	signature, err := generateConnectHandlerMethodSignature(g, method)
	if err != nil {
		return err
	}

	g.P("func (s *", service.ConnectHandlerName, ") ", signature, " {")
	switch method.StreamType {
	case ServiceStreamTypeUnary:
		err = generateConnectHandlerUnaryMethodBody(g, method)
	case ServiceStreamTypeServerStreaming:
		err = generateConnectHandlerServerStreamingMethodBody(g, method)
	case ServiceStreamTypeClientStreaming:
		err = generateConnectHandlerClientStreamingMethodBody(g, method)
	case ServiceStreamTypeBidiStreaming:
		err = generateConnectHandlerBidiStreamingMethodBody(g, method)
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

func generateConnectHandlerMethodSignature(g *protogen.GeneratedFile, method ServiceMethodPlan) (string, error) {
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
		return fmt.Sprintf("%s(ctx %s, requestProto %s) (%s, error)", method.Name, contextType, request, response), nil
	case ServiceStreamTypeServerStreaming:
		request, err := generateConnectMessageType(g, "Request", method.Request.ProtoType)
		if err != nil {
			return "", err
		}
		stream, err := generateConnectMessageType(g, "ServerStream", method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(ctx %s, requestProto %s, stream %s) error", method.Name, contextType, request, stream), nil
	case ServiceStreamTypeClientStreaming:
		stream, err := generateConnectMessageType(g, "ClientStream", method.Request.ProtoType)
		if err != nil {
			return "", err
		}
		response, err := generateConnectMessageType(g, "Response", method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(ctx %s, stream %s) (%s, error)", method.Name, contextType, stream, response), nil
	case ServiceStreamTypeBidiStreaming:
		stream, err := generateConnectMessageType(g, "BidiStream", method.Request.ProtoType, method.Response.ProtoType)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s(ctx %s, stream %s) error", method.Name, contextType, stream), nil
	default:
		return "", fmt.Errorf("unsupported stream type %d", method.StreamType)
	}
}

func generateConnectHandlerUnaryMethodBody(g *protogen.GeneratedFile, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceMappedAssignment(ctx, "requestTego", method.Request.FromProto, "requestProto.Msg"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("response, err := s.service." + method.Name + "(ctx, " + generateTegoconnectSymbol(g, "NewRequest") + "(requestTego, requestProto))")
	ctx.line("if err != nil {")
	ctx.line("return nil, err")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response.Message"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("return " + generateTegoconnectSymbol(g, "NewNativeResponse") + "(responseProto, response), nil")
	return nil
}

func generateConnectHandlerServerStreamingMethodBody(g *protogen.GeneratedFile, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "err")
	if err := generateServiceMappedAssignment(ctx, "requestTego", method.Request.FromProto, "requestProto.Msg"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	if err := generateServiceConverterFunc(g, "responseToProto", method.Response.ToProto, "responseTego", "responseProto"); err != nil {
		return fmt.Errorf("response converter: %w", err)
	}
	ctx.line("tegoStream := " + generateTegoconnectSymbol(g, "NewServerSendStream") + "(stream, responseToProto)")
	ctx.line("return s.service." + method.Name + "(ctx, " + generateTegoconnectSymbol(g, "NewRequest") + "(requestTego, requestProto), tegoStream)")
	return nil
}

func generateConnectHandlerClientStreamingMethodBody(g *protogen.GeneratedFile, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceConverterFunc(g, "requestFromProto", method.Request.FromProto, "requestProto", "requestTego"); err != nil {
		return fmt.Errorf("request converter: %w", err)
	}
	ctx.line("tegoStream := " + generateTegoconnectSymbol(g, "NewServerRecvStream") + "(stream, requestFromProto)")
	ctx.line("response, err := s.service." + method.Name + "(ctx, tegoStream)")
	ctx.line("if err != nil {")
	ctx.line("return nil, err")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseProto", method.Response.ToProto, "response.Message"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("return " + generateTegoconnectSymbol(g, "NewNativeResponse") + "(responseProto, response), nil")
	return nil
}

func generateConnectHandlerBidiStreamingMethodBody(g *protogen.GeneratedFile, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "err")
	if err := generateServiceConverterFunc(g, "requestFromProto", method.Request.FromProto, "requestProto", "requestTego"); err != nil {
		return fmt.Errorf("request converter: %w", err)
	}
	if err := generateServiceConverterFunc(g, "responseToProto", method.Response.ToProto, "responseTego", "responseProto"); err != nil {
		return fmt.Errorf("response converter: %w", err)
	}
	ctx.line("tegoStream := " + generateTegoconnectSymbol(g, "NewServerBidiStream") + "(stream, requestFromProto, responseToProto)")
	ctx.line("return s.service." + method.Name + "(ctx, tegoStream)")
	return nil
}

func generateConnectClientMethod(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) error {
	signature, err := generateClientMethodSignature(g, method)
	if err != nil {
		return err
	}

	g.P("func (c *", service.ConnectClientName, ") ", signature, " {")
	switch method.StreamType {
	case ServiceStreamTypeUnary:
		err = generateConnectClientUnaryMethodBody(g, method)
	case ServiceStreamTypeServerStreaming:
		err = generateConnectClientServerStreamingMethodBody(g, method)
	case ServiceStreamTypeClientStreaming:
		err = generateConnectClientClientStreamingMethodBody(g, method)
	case ServiceStreamTypeBidiStreaming:
		err = generateConnectClientBidiStreamingMethodBody(g, method)
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

func generateConnectClientUnaryMethodBody(g *protogen.GeneratedFile, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request.Message"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	ctx.line("nativeRequest := " + generateTegoconnectSymbol(g, "NewNativeRequest") + "(requestProto, request)")
	ctx.line("responseProto, err := c.client." + method.Name + "(ctx, nativeRequest)")
	ctx.line("if err != nil {")
	ctx.line("return nil, err")
	ctx.line("}")
	if err := generateServiceMappedAssignment(ctx, "responseTego", method.Response.FromProto, "responseProto.Msg"); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	ctx.line("return " + generateTegoconnectSymbol(g, "NewResponse") + "(responseTego, responseProto), nil")
	return nil
}

func generateConnectClientServerStreamingMethodBody(g *protogen.GeneratedFile, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceMappedAssignment(ctx, "requestProto", method.Request.ToProto, "request.Message"); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	if err := generateServiceConverterFunc(g, "responseFromProto", method.Response.FromProto, "responseProto", "responseTego"); err != nil {
		return fmt.Errorf("response converter: %w", err)
	}
	ctx.line("nativeRequest := " + generateTegoconnectSymbol(g, "NewNativeRequest") + "(requestProto, request)")
	ctx.line("stream, err := c.client." + method.Name + "(ctx, nativeRequest)")
	ctx.line("if err != nil {")
	ctx.line("return nil, err")
	ctx.line("}")
	ctx.line("return " + generateTegoconnectSymbol(g, "NewClientRecvStream") + "(stream, responseFromProto), nil")
	return nil
}

func generateConnectClientClientStreamingMethodBody(g *protogen.GeneratedFile, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceConverterFunc(g, "requestToProto", method.Request.ToProto, "requestTego", "requestProto"); err != nil {
		return fmt.Errorf("request converter: %w", err)
	}
	if err := generateServiceConverterFunc(g, "responseFromProto", method.Response.FromProto, "responseProto", "responseTego"); err != nil {
		return fmt.Errorf("response converter: %w", err)
	}
	ctx.line("call := " + generateTegoSymbol(g, "NewCall") + "(opts...)")
	ctx.line("stream := c.client." + method.Name + "(ctx)")
	ctx.line(generateTegoconnectSymbol(g, "CopyMetadataToHeader") + "(stream.RequestHeader(), call.Header())")
	ctx.line("return " + generateTegoconnectSymbol(g, "NewClientSendStream") + "(stream, requestToProto, responseFromProto), nil")
	return nil
}

func generateConnectClientBidiStreamingMethodBody(g *protogen.GeneratedFile, method ServiceMethodPlan) error {
	ctx := newMappingRenderContext(g, true, "nil, err")
	if err := generateServiceConverterFunc(g, "requestToProto", method.Request.ToProto, "requestTego", "requestProto"); err != nil {
		return fmt.Errorf("request converter: %w", err)
	}
	if err := generateServiceConverterFunc(g, "responseFromProto", method.Response.FromProto, "responseProto", "responseTego"); err != nil {
		return fmt.Errorf("response converter: %w", err)
	}
	ctx.line("call := " + generateTegoSymbol(g, "NewCall") + "(opts...)")
	ctx.line("stream := c.client." + method.Name + "(ctx)")
	ctx.line(generateTegoconnectSymbol(g, "CopyMetadataToHeader") + "(stream.RequestHeader(), call.Header())")
	ctx.line("return " + generateTegoconnectSymbol(g, "NewClientBidiStream") + "(stream, requestToProto, responseFromProto), nil")
	return nil
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

func generateServiceConverterFunc(
	g *protogen.GeneratedFile,
	name string,
	plan MappingValuePlan,
	sourceName string,
	targetName string,
) error {
	sourceType, err := generateType(g, plan.Source)
	if err != nil {
		return fmt.Errorf("source type: %w", err)
	}
	targetType, err := generateType(g, plan.Target)
	if err != nil {
		return fmt.Errorf("target type: %w", err)
	}

	g.P(name, " := func(", sourceName, " ", sourceType, ") (", targetType, ", error) {")
	if expr, returnsError, ok, err := serviceConverterReturnExpr(g, plan, sourceName); err != nil {
		return err
	} else if ok {
		if returnsError {
			g.P("return ", expr)
		} else {
			g.P("return ", expr, ", nil")
		}
		g.P("}")
		return nil
	}

	ctx := newMappingRenderContext(g, true, targetName+", err", sourceName)
	expr, err := ctx.renderValueWithTempNameHint(targetName, plan, sourceName)
	if err != nil {
		return err
	}
	g.P("return ", expr, ", nil")
	g.P("}")
	return nil
}

func serviceConverterReturnExpr(
	g *protogen.GeneratedFile,
	plan MappingValuePlan,
	source string,
) (expr string, returnsError bool, ok bool, err error) {
	switch plan.Kind {
	case MappingValueKindDirect:
		return source, false, true, nil
	case MappingValueKindScalarCast, MappingValueKindEnum:
		target, err := generateType(g, plan.Target)
		if err != nil {
			return "", false, false, err
		}
		return target + "(" + source + ")", false, true, nil
	case MappingValueKindStruct:
		if plan.Struct == nil {
			return "", false, false, fmt.Errorf("struct mapping is missing a ref")
		}
		name := plan.Struct.Name
		if plan.Struct.Ref.Name != "" {
			name = generateSymbol(g, plan.Struct.Ref)
		}
		return name + "(" + source + ")", plan.CanError, true, nil
	case MappingValueKindCustom:
		if plan.Custom == nil {
			return "", false, false, fmt.Errorf("custom mapping is missing conversion refs")
		}
		expr := serviceCustomConverterReturnExpr(g, plan, source)
		return expr, plan.CanError, true, nil
	case MappingValueKindEmptyStruct:
		expr, err := serviceEmptyStructConverterReturnExpr(g, plan)
		return expr, false, err == nil, err
	default:
		return "", false, false, nil
	}
}

func serviceCustomConverterReturnExpr(g *protogen.GeneratedFile, plan MappingValuePlan, source string) string {
	ref := plan.Custom.FromProto
	if top, ok := topCustomType(plan.Source); ok && top.ToProto.Name != "" {
		ref = plan.Custom.ToProto
	}

	if ref.Receiver != "" {
		return source + "." + ref.Name + "()"
	}
	return generateSymbol(g, ref) + "(" + source + ")"
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

func generateGRPCServiceSpec(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) string {
	return generateTegoType(g, "Spec") + "{Procedure: " + generateGRPCFullMethodNameSymbol(g, service, method) + ", StreamType: " + generateTegoStreamType(g, method.StreamType) + "}"
}

func generateGRPCFullMethodNameSymbol(g *protogen.GeneratedFile, service ServicePlan, method ServiceMethodPlan) string {
	methodGoName := method.ProtoGoName
	if methodGoName == "" {
		methodGoName = method.Name
	}

	return generateSymbol(g, GoSymbolRef{
		ImportPath: service.ProtoRef.ImportPath,
		Name:       service.ProtoRef.Name + "_" + methodGoName + "_FullMethodName",
	})
}

func generateTegoStreamType(g *protogen.GeneratedFile, streamType ServiceStreamType) string {
	switch streamType {
	case ServiceStreamTypeClientStreaming:
		return generateTegoSymbol(g, "StreamTypeClientStreaming")
	case ServiceStreamTypeServerStreaming:
		return generateTegoSymbol(g, "StreamTypeServerStreaming")
	case ServiceStreamTypeBidiStreaming:
		return generateTegoSymbol(g, "StreamTypeBidiStreaming")
	default:
		return generateTegoSymbol(g, "StreamTypeUnary")
	}
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

func generateHTTPType(g *protogen.GeneratedFile, name string) string {
	return generateNamedType(g, GoTypeRef{ImportPath: httpImportPath, Name: name})
}

func generateGRPCSymbol(g *protogen.GeneratedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: grpcImportPath, Name: name})
}

func generateMetadataType(g *protogen.GeneratedFile, name string) string {
	return generateNamedType(g, GoTypeRef{ImportPath: metadataImportPath, Name: name})
}

func generateTegoSymbol(g *protogen.GeneratedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: tegoImportPath, Name: name})
}

func generateTegogrpcSymbol(g *protogen.GeneratedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: tegogrpcImportPath, Name: name})
}

func generateTegoconnectSymbol(g *protogen.GeneratedFile, name string) string {
	return generateSymbol(g, GoSymbolRef{ImportPath: tegoconnectImportPath, Name: name})
}
