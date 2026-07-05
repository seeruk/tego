package tego

import "connectrpc.com/connect"

// GRPCServerOption configures a generated gRPC server.
type GRPCServerOption interface {
	applyGRPCServerOption(*transportOptions)
}

// GRPCAdapterOption configures a generated gRPC adapter.
type GRPCAdapterOption interface {
	applyGRPCAdapterOption(*transportOptions)
}

// GRPCClientOption configures a generated gRPC client.
type GRPCClientOption interface {
	applyGRPCClientOption(*transportOptions)
}

// ConnectHandlerOption configures a generated Connect handler.
type ConnectHandlerOption interface {
	applyConnectHandlerOption(*transportOptions)
}

// ConnectAdapterOption configures a generated Connect adapter.
type ConnectAdapterOption interface {
	applyConnectAdapterOption(*transportOptions)
}

// ConnectClientOption configures a generated Connect client.
type ConnectClientOption interface {
	applyConnectClientOption(*transportOptions)
}

// TransportOptions contains resolved generated transport configuration.
type TransportOptions struct {
	errorMapper           ErrorMapper
	connectHandlerOptions []connect.HandlerOption
}

// NewGRPCServerOptions resolves generated gRPC server options.
func NewGRPCServerOptions(opts ...GRPCServerOption) TransportOptions {
	var options transportOptions
	for _, opt := range opts {
		opt.applyGRPCServerOption(&options)
	}
	return options.export()
}

// NewGRPCAdapterOptions resolves generated gRPC adapter options.
func NewGRPCAdapterOptions(opts ...GRPCAdapterOption) TransportOptions {
	var options transportOptions
	for _, opt := range opts {
		opt.applyGRPCAdapterOption(&options)
	}
	return options.export()
}

// NewGRPCClientOptions resolves generated gRPC client options.
func NewGRPCClientOptions(opts ...GRPCClientOption) TransportOptions {
	var options transportOptions
	for _, opt := range opts {
		opt.applyGRPCClientOption(&options)
	}
	return options.export()
}

// NewConnectHandlerOptions resolves generated Connect handler options.
func NewConnectHandlerOptions(opts ...ConnectHandlerOption) TransportOptions {
	var options transportOptions
	for _, opt := range opts {
		opt.applyConnectHandlerOption(&options)
	}
	return options.export()
}

// NewConnectAdapterOptions resolves generated Connect adapter options.
func NewConnectAdapterOptions(opts ...ConnectAdapterOption) TransportOptions {
	var options transportOptions
	for _, opt := range opts {
		opt.applyConnectAdapterOption(&options)
	}
	return options.export()
}

// NewConnectClientOptions resolves generated Connect client options.
func NewConnectClientOptions(opts ...ConnectClientOption) TransportOptions {
	var options transportOptions
	for _, opt := range opts {
		opt.applyConnectClientOption(&options)
	}
	return options.export()
}

// ErrorMapper returns the configured error mapper, or defaultMapper when none is configured.
func (o TransportOptions) ErrorMapper(defaultMapper ErrorMapper) ErrorMapper {
	if o.errorMapper != nil {
		return o.errorMapper
	}
	return defaultMapper
}

// ConnectHandlerOptions returns the configured native Connect handler options.
func (o TransportOptions) ConnectHandlerOptions() []connect.HandlerOption {
	return append([]connect.HandlerOption(nil), o.connectHandlerOptions...)
}

type transportOptions struct {
	errorMapper           ErrorMapper
	connectHandlerOptions []connect.HandlerOption
}

func (o transportOptions) export() TransportOptions {
	return TransportOptions{
		errorMapper:           o.errorMapper,
		connectHandlerOptions: append([]connect.HandlerOption(nil), o.connectHandlerOptions...),
	}
}

type errorMapperOption struct {
	errorMapper ErrorMapper
}

// WithErrorMapper configures generated transports to map boundary errors.
func WithErrorMapper(errorMapper ErrorMapper) errorMapperOption {
	return errorMapperOption{errorMapper: errorMapper}
}

func (o errorMapperOption) applyGRPCServerOption(options *transportOptions) {
	options.errorMapper = o.errorMapper
}

func (o errorMapperOption) applyGRPCAdapterOption(options *transportOptions) {
	options.errorMapper = o.errorMapper
}

func (o errorMapperOption) applyGRPCClientOption(options *transportOptions) {
	options.errorMapper = o.errorMapper
}

func (o errorMapperOption) applyConnectHandlerOption(options *transportOptions) {
	options.errorMapper = o.errorMapper
}

func (o errorMapperOption) applyConnectAdapterOption(options *transportOptions) {
	options.errorMapper = o.errorMapper
}

func (o errorMapperOption) applyConnectClientOption(options *transportOptions) {
	options.errorMapper = o.errorMapper
}

type connectHandlerOptionsOption struct {
	handlerOptions []connect.HandlerOption
}

// WithConnectHandlerOptions passes native Connect handler options to generated Connect handlers.
func WithConnectHandlerOptions(handlerOptions ...connect.HandlerOption) connectHandlerOptionsOption {
	return connectHandlerOptionsOption{
		handlerOptions: append([]connect.HandlerOption(nil), handlerOptions...),
	}
}

func (o connectHandlerOptionsOption) applyConnectHandlerOption(options *transportOptions) {
	options.connectHandlerOptions = append(options.connectHandlerOptions, o.handlerOptions...)
}
