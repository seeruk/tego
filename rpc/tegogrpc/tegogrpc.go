package tegogrpc

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/seeruk/tego"
)

const protocol = "grpc"

// MetadataFromMD returns a Tego copy of md.
func MetadataFromMD(md metadata.MD) tego.Metadata {
	if md == nil {
		return nil
	}

	out := make(tego.Metadata, len(md))
	for key, values := range md {
		out[key] = append([]string(nil), values...)
	}
	return out
}

// MDFromMetadata returns a gRPC metadata copy of source.
func MDFromMetadata(in tego.Metadata) metadata.MD {
	if in == nil {
		return nil
	}

	md := make(metadata.MD, len(in))
	for key, values := range in {
		md[strings.ToLower(key)] = append([]string(nil), values...)
	}
	return md
}

// MetadataFromIncomingContext returns request metadata from ctx.
func MetadataFromIncomingContext(ctx context.Context) tego.Metadata {
	md, _ := metadata.FromIncomingContext(ctx)
	return MetadataFromMD(md)
}

// MetadataFromOutgoingContext returns request metadata from ctx.
func MetadataFromOutgoingContext(ctx context.Context) tego.Metadata {
	md, _ := metadata.FromOutgoingContext(ctx)
	return MetadataFromMD(md)
}

// NewIncomingContext returns a context with incoming metadata.
func NewIncomingContext(ctx context.Context, source tego.Metadata) context.Context {
	return metadata.NewIncomingContext(ctx, MDFromMetadata(source))
}

// NewOutgoingContext returns a context with outgoing metadata.
func NewOutgoingContext(ctx context.Context, source tego.Metadata) context.Context {
	return metadata.NewOutgoingContext(ctx, MDFromMetadata(source))
}

// PeerFromContext converts gRPC peer information from ctx into a Tego peer.
func PeerFromContext(ctx context.Context) tego.Peer {
	opts := []tego.PeerOption{tego.WithPeerProtocol(protocol)}
	if native, ok := peer.FromContext(ctx); ok {
		if native.Addr != nil {
			opts = append(opts, tego.WithPeerAddr(native.Addr.String()))
		}
		if native.LocalAddr != nil {
			opts = append(opts, tego.WithPeerLocalAddr(native.LocalAddr.String()))
		}
		if native.AuthInfo != nil {
			opts = append(opts, tego.WithPeerAuthInfo(native.AuthInfo))
		}
	}
	return tego.NewPeer(opts...)
}

// Spec returns a Tego RPC spec.
func Spec(procedure string, streamType tego.StreamType) tego.Spec {
	return tego.Spec{Procedure: procedure, StreamType: streamType}
}

// SpecFromContext returns a Tego RPC spec using the gRPC method in ctx when available.
func SpecFromContext(ctx context.Context, streamType tego.StreamType) tego.Spec {
	procedure, _ := grpc.Method(ctx)
	return Spec(procedure, streamType)
}

// SpecFromServerStream returns a Tego RPC spec using the gRPC method on stream when available.
func SpecFromServerStream(stream grpc.ServerStream, streamType tego.StreamType) tego.Spec {
	procedure, _ := grpc.MethodFromServerStream(stream)
	return Spec(procedure, streamType)
}

// NewRequest adapts a gRPC request into a Tego request with an already-mapped message.
func NewRequest[T any](ctx context.Context, message T, spec tego.Spec) *tego.Request[T] {
	return tego.NewRequest(
		message,
		tego.WithRequestHeader(MetadataFromIncomingContext(ctx)),
		tego.WithRequestPeer(PeerFromContext(ctx)),
		tego.WithRequestSpec(spec),
		tego.WithNativeRequest(ctx),
	)
}

// NewResponse adapts captured gRPC response metadata into a Tego response.
func NewResponse[T any](
	message T,
	header metadata.MD,
	trailer metadata.MD,
	native any,
) *tego.Response[T] {
	return tego.NewResponse(
		message,
		tego.WithResponseHeader(MetadataFromMD(header)),
		tego.WithResponseTrailer(MetadataFromMD(trailer)),
		tego.WithNativeResponse(native),
	)
}

// ApplyResponseMetadata applies response headers and trailers to a gRPC unary response.
func ApplyResponseMetadata[T any](ctx context.Context, response *tego.Response[T]) error {
	if err := grpc.SetHeader(ctx, MDFromMetadata(response.Header())); err != nil {
		return err
	}
	return grpc.SetTrailer(ctx, MDFromMetadata(response.Trailer()))
}

// StreamResponseMetadata is implemented by Tego stream wrappers that carry response metadata.
type StreamResponseMetadata interface {
	ResponseHeader() tego.Metadata
	ResponseTrailer() tego.Metadata
}

// ApplyStreamResponseMetadata applies response headers and trailers to a gRPC stream.
func ApplyStreamResponseMetadata(stream grpc.ServerStream, source StreamResponseMetadata) error {
	if err := stream.SetHeader(MDFromMetadata(source.ResponseHeader())); err != nil {
		return err
	}
	stream.SetTrailer(MDFromMetadata(source.ResponseTrailer()))
	return nil
}

// HandlerServerStream is the gRPC handler side of a server-streaming RPC.
type HandlerServerStream[Response any] interface {
	Send(*Response) error
	grpc.ServerStream
}

// HandlerClientStream is the gRPC handler side of a client-streaming RPC.
type HandlerClientStream[Request any] interface {
	Recv() (*Request, error)
	grpc.ServerStream
}

// HandlerBidiStream is the gRPC handler side of a bidirectional-streaming RPC.
type HandlerBidiStream[Request, Response any] interface {
	Recv() (*Request, error)
	Send(*Response) error
	grpc.ServerStream
}

// ClientServerStream is the gRPC client side of a server-streaming RPC.
type ClientServerStream[Response any] interface {
	Recv() (*Response, error)
	grpc.ClientStream
}

// ClientClientStream is the gRPC client side of a client-streaming RPC.
type ClientClientStream[Request, Response any] interface {
	Send(*Request) error
	CloseAndRecv() (*Response, error)
	grpc.ClientStream
}

// ClientBidiStream is the gRPC client side of a bidirectional-streaming RPC.
type ClientBidiStream[Request, Response any] interface {
	Send(*Request) error
	Recv() (*Response, error)
	grpc.ClientStream
}

// NewServerSendStream adapts a gRPC handler server stream.
func NewServerSendStream[T, P any](
	stream HandlerServerStream[P],
	spec tego.Spec,
	toNative func(T) (*P, error),
) *tego.ServerSendStream[T] {
	var result *tego.ServerSendStream[T]
	var setHeader func() error
	result = tego.NewServerSendStream(
		func(message T) error {
			if err := setHeader(); err != nil {
				return err
			}
			return sendMapped(message, toNative, stream.Send)
		},
		serverStreamOptions(stream, spec, stream)...,
	)
	setHeader = setHeaderOnce(stream, result.ResponseHeader)
	return result
}

// NewServerRecvStream adapts a gRPC handler client stream.
func NewServerRecvStream[T, P any](
	stream HandlerClientStream[P],
	spec tego.Spec,
	fromNative func(*P) (T, error),
) *tego.ServerRecvStream[T] {
	return tego.NewServerRecvStream(
		func() (T, error) {
			return receiveMapped(stream.Recv, fromNative)
		},
		serverStreamOptions(stream, spec, stream)...,
	)
}

// NewServerBidiStream adapts a gRPC handler bidirectional stream.
func NewServerBidiStream[ReqT, ResT, ReqP, ResP any](
	stream HandlerBidiStream[ReqP, ResP],
	spec tego.Spec,
	fromNative func(*ReqP) (ReqT, error),
	toNative func(ResT) (*ResP, error),
) *tego.ServerBidiStream[ReqT, ResT] {
	var result *tego.ServerBidiStream[ReqT, ResT]
	var setHeader func() error
	result = tego.NewServerBidiStream(
		func() (ReqT, error) {
			return receiveMapped(stream.Recv, fromNative)
		},
		func(message ResT) error {
			if err := setHeader(); err != nil {
				return err
			}
			return sendMapped(message, toNative, stream.Send)
		},
		serverStreamOptions(stream, spec, stream)...,
	)
	setHeader = setHeaderOnce(stream, result.ResponseHeader)
	return result
}

// NewClientRecvStream adapts a gRPC client server stream.
func NewClientRecvStream[T, P any](
	stream ClientServerStream[P],
	spec tego.Spec,
	fromNative func(*P) (T, error),
) *tego.ClientRecvStream[T] {
	responseHeader, responseTrailer := responseMetadata()
	return tego.NewClientRecvStream(
		func() (T, error) {
			native, err := stream.Recv()
			copyResponseMetadata(stream, responseHeader, responseTrailer)
			return mapReceived(native, err, fromNative)
		},
		func() error {
			err := stream.CloseSend()
			copyResponseMetadata(stream, responseHeader, responseTrailer)
			return err
		},
		clientStreamOptions(stream, spec, responseHeader, responseTrailer)...,
	)
}

// NewClientSendStream adapts a gRPC client's client stream.
// RequestHeader on the returned stream is informational because gRPC request metadata is fixed when
// the native stream opens.
func NewClientSendStream[ReqT, ResT, ReqP, ResP any](
	stream ClientClientStream[ReqP, ResP],
	spec tego.Spec,
	toNative func(ReqT) (*ReqP, error),
	fromNative func(*ResP) (ResT, error),
	opts ...tego.StreamOption,
) *tego.ClientSendStream[ReqT, ResT] {
	responseHeader, responseTrailer := responseMetadata()
	return tego.NewClientSendStream(
		func(message ReqT) error {
			return sendMapped(message, toNative, stream.Send)
		},
		func() (*tego.Response[ResT], error) {
			native, err := stream.CloseAndRecv()
			copyResponseMetadata(stream, responseHeader, responseTrailer)
			if err != nil {
				return nil, err
			}
			message, err := fromNative(native)
			if err != nil {
				return nil, err
			}
			return tego.NewResponse(
				message,
				tego.WithResponseHeader(responseHeader),
				tego.WithResponseTrailer(responseTrailer),
				tego.WithNativeResponse(stream),
			), nil
		},
		clientStreamOptions(stream, spec, responseHeader, responseTrailer, opts...)...,
	)
}

// NewClientBidiStream adapts a gRPC client bidirectional stream.
// RequestHeader on the returned stream is informational because gRPC request metadata is fixed when
// the native stream opens.
func NewClientBidiStream[ReqT, ResT, ReqP, ResP any](
	stream ClientBidiStream[ReqP, ResP],
	spec tego.Spec,
	toNative func(ReqT) (*ReqP, error),
	fromNative func(*ResP) (ResT, error),
	opts ...tego.StreamOption,
) *tego.ClientBidiStream[ReqT, ResT] {
	responseHeader, responseTrailer := responseMetadata()
	return tego.NewClientBidiStream(
		func(message ReqT) error {
			return sendMapped(message, toNative, stream.Send)
		},
		func() (ResT, error) {
			native, err := stream.Recv()
			copyResponseMetadata(stream, responseHeader, responseTrailer)
			return mapReceived(native, err, fromNative)
		},
		stream.CloseSend,
		nil,
		clientStreamOptions(stream, spec, responseHeader, responseTrailer, opts...)...,
	)
}

func sendMapped[T, P any](
	message T,
	toNative func(T) (*P, error),
	send func(*P) error,
) error {
	native, err := toNative(message)
	if err != nil {
		return err
	}
	return send(native)
}

func receiveMapped[T, P any](
	receive func() (*P, error),
	fromNative func(*P) (T, error),
) (T, error) {
	native, err := receive()
	return mapReceived(native, err, fromNative)
}

func mapReceived[T, P any](
	native *P,
	err error,
	fromNative func(*P) (T, error),
) (T, error) {
	if err != nil {
		var zero T
		return zero, err
	}
	return fromNative(native)
}

func setHeaderOnce(stream grpc.ServerStream, header func() tego.Metadata) func() error {
	applied := false
	return func() error {
		if applied {
			return nil
		}
		applied = true
		return stream.SetHeader(MDFromMetadata(header()))
	}
}

func responseMetadata() (tego.Metadata, tego.Metadata) {
	return make(tego.Metadata), make(tego.Metadata)
}

func serverStreamOptions(native any, spec tego.Spec, stream grpc.ServerStream) []tego.StreamOption {
	return []tego.StreamOption{
		tego.WithStreamRequestHeader(MetadataFromIncomingContext(stream.Context())),
		tego.WithStreamPeer(PeerFromContext(stream.Context())),
		tego.WithStreamSpec(spec),
		tego.WithNativeStream(native),
	}
}

func clientStreamOptions(
	native any,
	spec tego.Spec,
	responseHeader tego.Metadata,
	responseTrailer tego.Metadata,
	opts ...tego.StreamOption,
) []tego.StreamOption {
	options := []tego.StreamOption{
		tego.WithStreamResponseHeader(responseHeader),
		tego.WithStreamResponseTrailer(responseTrailer),
		tego.WithStreamSpec(spec),
		tego.WithNativeStream(native),
	}
	return append(options, opts...)
}

func copyResponseMetadata(stream grpc.ClientStream, header, trailer tego.Metadata) {
	nativeHeader, err := stream.Header()
	if err == nil {
		copyMetadata(header, MetadataFromMD(nativeHeader))
	}
	copyMetadata(trailer, MetadataFromMD(stream.Trailer()))
}

func copyMetadata(target, source tego.Metadata) {
	for key := range target {
		delete(target, key)
	}
	for key, values := range source {
		target[key] = append([]string(nil), values...)
	}
}
