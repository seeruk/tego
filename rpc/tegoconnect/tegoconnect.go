package tegoconnect

import (
	"io"
	"net/http"
	"net/url"

	"connectrpc.com/connect"

	"github.com/seeruk/tego"
)

// MetadataFromHeader returns a live Tego view of header.
func MetadataFromHeader(header http.Header) tego.Metadata {
	return tego.Metadata(header)
}

// HeaderFromMetadata returns a live HTTP header view of metadata.
func HeaderFromMetadata(metadata tego.Metadata) http.Header {
	return http.Header(metadata)
}

// MetadataFromValues returns a live Tego view of values.
func MetadataFromValues(values url.Values) tego.Metadata {
	return tego.Metadata(values)
}

// CopyMetadataToHeader copies metadata into header.
func CopyMetadataToHeader(header http.Header, metadata tego.Metadata) {
	for key, values := range metadata {
		header[key] = append([]string(nil), values...)
	}
}

// Peer converts a Connect peer into a Tego peer.
func Peer(peer connect.Peer) tego.Peer {
	return tego.NewPeer(
		tego.WithPeerAddr(peer.Addr),
		tego.WithPeerProtocol(peer.Protocol),
	)
}

// Spec converts a Connect spec into a Tego spec.
func Spec(spec connect.Spec) tego.Spec {
	return tego.Spec{
		Procedure:  spec.Procedure,
		StreamType: StreamType(spec.StreamType),
	}
}

// StreamType converts a Connect stream type into a Tego stream type.
func StreamType(streamType connect.StreamType) tego.StreamType {
	switch streamType {
	case connect.StreamTypeClient:
		return tego.StreamTypeClientStreaming
	case connect.StreamTypeServer:
		return tego.StreamTypeServerStreaming
	case connect.StreamTypeBidi:
		return tego.StreamTypeBidiStreaming
	default:
		return tego.StreamTypeUnary
	}
}

// NewRequest adapts a Connect request into a Tego request with an already-mapped message.
func NewRequest[T, P any](message T, request *connect.Request[P]) *tego.Request[T] {
	peer := request.Peer()
	return tego.NewRequest(
		message,
		tego.WithRequestHeader(MetadataFromHeader(request.Header())),
		tego.WithRequestQuery(MetadataFromValues(peer.Query)),
		tego.WithRequestPeer(Peer(peer)),
		tego.WithRequestSpec(Spec(request.Spec())),
		tego.WithRequestHTTPMethod(request.HTTPMethod()),
		tego.WithNativeRequest(request),
	)
}

// NewNativeRequest adapts a Tego request and already-mapped message into a Connect request.
func NewNativeRequest[P, T any](
	message *P,
	request *tego.Request[T],
) *connect.Request[P] {
	native := connect.NewRequest(message)
	CopyMetadataToHeader(native.Header(), request.Header())
	return native
}

// NewResponse adapts a Connect response into a Tego response with an already-mapped message.
func NewResponse[T, P any](message T, response *connect.Response[P]) *tego.Response[T] {
	return tego.NewResponse(
		message,
		tego.WithResponseHeader(MetadataFromHeader(response.Header())),
		tego.WithResponseTrailer(MetadataFromHeader(response.Trailer())),
		tego.WithNativeResponse(response),
	)
}

// NewNativeResponse adapts a Tego response and already-mapped message into a Connect response.
func NewNativeResponse[P, T any](
	message *P,
	response *tego.Response[T],
) *connect.Response[P] {
	native := connect.NewResponse(message)
	CopyMetadataToHeader(native.Header(), response.Header())
	CopyMetadataToHeader(native.Trailer(), response.Trailer())
	return native
}

// HandlerServerStream is the Connect handler side of a server-streaming RPC.
type HandlerServerStream[Response any] interface {
	Conn() connect.StreamingHandlerConn
	Send(*Response) error
	ResponseHeader() http.Header
	ResponseTrailer() http.Header
}

// HandlerClientStream is the Connect handler side of a client-streaming RPC.
type HandlerClientStream[Request any] interface {
	Conn() connect.StreamingHandlerConn
	Receive() bool
	Msg() *Request
	Err() error
	RequestHeader() http.Header
	Peer() connect.Peer
	Spec() connect.Spec
}

// HandlerBidiStream is the Connect handler side of a bidirectional-streaming RPC.
type HandlerBidiStream[Request, Response any] interface {
	Conn() connect.StreamingHandlerConn
	Receive() (*Request, error)
	Send(*Response) error
	RequestHeader() http.Header
	ResponseHeader() http.Header
	ResponseTrailer() http.Header
	Peer() connect.Peer
	Spec() connect.Spec
}

// ClientServerStream is the Connect client side of a server-streaming RPC.
type ClientServerStream[Response any] interface {
	Conn() (connect.StreamingClientConn, error)
	Receive() bool
	Msg() *Response
	Err() error
	ResponseHeader() http.Header
	ResponseTrailer() http.Header
	Close() error
}

// ClientClientStream is the Connect client side of a client-streaming RPC.
type ClientClientStream[Request, Response any] interface {
	Conn() (connect.StreamingClientConn, error)
	Send(*Request) error
	CloseAndReceive() (*connect.Response[Response], error)
	RequestHeader() http.Header
	Peer() connect.Peer
	Spec() connect.Spec
}

// ClientBidiStream is the Connect client side of a bidirectional-streaming RPC.
type ClientBidiStream[Request, Response any] interface {
	Conn() (connect.StreamingClientConn, error)
	Send(*Request) error
	Receive() (*Response, error)
	CloseRequest() error
	CloseResponse() error
	RequestHeader() http.Header
	ResponseHeader() http.Header
	ResponseTrailer() http.Header
	Peer() connect.Peer
	Spec() connect.Spec
}

// NewServerSendStream adapts a Connect handler server stream.
func NewServerSendStream[T, P any](
	stream HandlerServerStream[P],
	toNative func(T) (*P, error),
) *tego.ServerSendStream[T] {
	return tego.NewServerSendStream(
		func(message T) error {
			return sendMapped(message, toNative, stream.Send)
		},
		handlerStreamOptions(stream, stream.Conn(), stream.ResponseHeader(), stream.ResponseTrailer())...,
	)
}

// NewServerRecvStream adapts a Connect handler client stream.
func NewServerRecvStream[T, P any](
	stream HandlerClientStream[P],
	fromNative func(*P) (T, error),
) *tego.ServerRecvStream[T] {
	return tego.NewServerRecvStream(
		func() (T, error) {
			return receiveConnectMessage(stream, fromNative)
		},
		handlerStreamOptions(stream, stream.Conn(), nil, nil)...,
	)
}

// NewServerBidiStream adapts a Connect handler bidirectional stream.
func NewServerBidiStream[ReqT, ResT, ReqP, ResP any](
	stream HandlerBidiStream[ReqP, ResP],
	fromNative func(*ReqP) (ReqT, error),
	toNative func(ResT) (*ResP, error),
) *tego.ServerBidiStream[ReqT, ResT] {
	return tego.NewServerBidiStream(
		func() (ReqT, error) {
			return receiveMapped(stream.Receive, fromNative)
		},
		func(message ResT) error {
			return sendMapped(message, toNative, stream.Send)
		},
		handlerStreamOptions(stream, stream.Conn(), stream.ResponseHeader(), stream.ResponseTrailer())...,
	)
}

// NewClientRecvStream adapts a Connect client server stream.
func NewClientRecvStream[T, P any](
	stream ClientServerStream[P],
	fromNative func(*P) (T, error),
) *tego.ClientRecvStream[T] {
	return tego.NewClientRecvStream(
		func() (T, error) {
			return receiveConnectMessage(stream, fromNative)
		},
		stream.Close,
		clientStreamOptions(stream, stream.ResponseHeader(), stream.ResponseTrailer())...,
	)
}

// NewClientSendStream adapts a Connect client client stream.
func NewClientSendStream[ReqT, ResT, ReqP, ResP any](
	stream ClientClientStream[ReqP, ResP],
	toNative func(ReqT) (*ReqP, error),
	fromNative func(*ResP) (ResT, error),
) *tego.ClientSendStream[ReqT, ResT] {
	return tego.NewClientSendStream(
		func(message ReqT) error {
			return sendMapped(message, toNative, stream.Send)
		},
		func() (*tego.Response[ResT], error) {
			response, err := stream.CloseAndReceive()
			if err != nil {
				return nil, err
			}
			message, err := fromNative(response.Msg)
			if err != nil {
				return nil, err
			}
			return NewResponse(message, response), nil
		},
		clientStreamOptions(stream, nil, nil)...,
	)
}

// NewClientBidiStream adapts a Connect client bidirectional stream.
func NewClientBidiStream[ReqT, ResT, ReqP, ResP any](
	stream ClientBidiStream[ReqP, ResP],
	toNative func(ReqT) (*ReqP, error),
	fromNative func(*ResP) (ResT, error),
) *tego.ClientBidiStream[ReqT, ResT] {
	responseHeader := make(tego.Metadata)
	responseTrailer := make(tego.Metadata)
	copyResponseMetadata := func() {
		copyHeaderToMetadata(responseHeader, stream.ResponseHeader())
		copyHeaderToMetadata(responseTrailer, stream.ResponseTrailer())
	}

	return tego.NewClientBidiStream(
		func(message ReqT) error {
			return sendMapped(message, toNative, stream.Send)
		},
		func() (ResT, error) {
			message, err := receiveMapped(stream.Receive, fromNative)
			copyResponseMetadata()
			return message, err
		},
		stream.CloseRequest,
		stream.CloseResponse,
		tego.WithStreamRequestHeader(MetadataFromHeader(stream.RequestHeader())),
		tego.WithStreamResponseHeader(responseHeader),
		tego.WithStreamResponseTrailer(responseTrailer),
		tego.WithStreamPeer(Peer(stream.Peer())),
		tego.WithStreamSpec(Spec(stream.Spec())),
		tego.WithNativeStream(stream),
	)
}

func copyHeaderToMetadata(target tego.Metadata, source http.Header) {
	for key := range target {
		delete(target, key)
	}
	CopyMetadataToHeader(http.Header(target), MetadataFromHeader(source))
}

type receiveBoolStream[P any] interface {
	Receive() bool
	Msg() *P
	Err() error
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
	if err != nil {
		var zero T
		return zero, err
	}
	return fromNative(native)
}

func receiveConnectMessage[T, P any](
	stream receiveBoolStream[P],
	fromNative func(*P) (T, error),
) (T, error) {
	if !stream.Receive() {
		if err := stream.Err(); err != nil {
			var zero T
			return zero, err
		}
		var zero T
		return zero, io.EOF
	}
	return fromNative(stream.Msg())
}

func handlerStreamOptions(
	native any,
	conn connect.StreamingHandlerConn,
	responseHeader http.Header,
	responseTrailer http.Header,
) []tego.StreamOption {
	peer := conn.Peer()
	return []tego.StreamOption{
		tego.WithStreamRequestHeader(MetadataFromHeader(conn.RequestHeader())),
		tego.WithStreamQuery(MetadataFromValues(peer.Query)),
		tego.WithStreamResponseHeader(MetadataFromHeader(responseHeader)),
		tego.WithStreamResponseTrailer(MetadataFromHeader(responseTrailer)),
		tego.WithStreamPeer(Peer(peer)),
		tego.WithStreamSpec(Spec(conn.Spec())),
		tego.WithNativeStream(native),
	}
}

type clientStreamInfo interface {
	Conn() (connect.StreamingClientConn, error)
}

func clientStreamOptions(native clientStreamInfo, responseHeader, responseTrailer http.Header) []tego.StreamOption {
	conn, err := native.Conn()
	if err != nil {
		return []tego.StreamOption{
			tego.WithStreamResponseHeader(MetadataFromHeader(responseHeader)),
			tego.WithStreamResponseTrailer(MetadataFromHeader(responseTrailer)),
			tego.WithNativeStream(native),
		}
	}

	return []tego.StreamOption{
		tego.WithStreamRequestHeader(MetadataFromHeader(conn.RequestHeader())),
		tego.WithStreamResponseHeader(MetadataFromHeader(responseHeader)),
		tego.WithStreamResponseTrailer(MetadataFromHeader(responseTrailer)),
		tego.WithStreamPeer(Peer(conn.Peer())),
		tego.WithStreamSpec(Spec(conn.Spec())),
		tego.WithNativeStream(native),
	}
}
