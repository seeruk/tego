package tegogrpc

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/seeruk/tego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	grpcpeer "google.golang.org/grpc/peer"
)

func TestMetadata(t *testing.T) {
	t.Run("copies to grpc metadata", func(t *testing.T) {
		source := tego.Metadata{"X-Ticket": {"one", "two"}}
		md := MDFromMetadata(source)

		source.Set("X-Ticket", "changed")

		assert.Equal(t, []string{"one", "two"}, md.Get("x-ticket"))
	})

	t.Run("copies from grpc metadata", func(t *testing.T) {
		source := metadata.Pairs("x-ticket", "one", "x-ticket", "two")
		metadata := MetadataFromMD(source)

		source.Set("x-ticket", "changed")

		assert.Equal(t, []string{"one", "two"}, metadata.Values("x-ticket"))
	})
}

func TestUnary(t *testing.T) {
	t.Run("adapts request", func(t *testing.T) {
		ctx := newServerContext(&fakeServerTransportStream{method: "/service/Get"})
		spec := SpecFromContext(ctx, tego.StreamTypeUnary)

		request := NewRequest(ctx, "request", spec)

		assert.Equal(t, "request", request.Message)
		assert.Equal(t, "one", request.Header().Get("x-request"))
		assert.Equal(t, spec, request.Spec())
		assert.True(t, request.HasNative())
	})

	t.Run("adapts peer", func(t *testing.T) {
		request := NewRequest(
			newServerContext(&fakeServerTransportStream{method: "/service/Get"}),
			"request",
			tego.Spec{},
		)

		addr, err := request.Peer().Addr()
		require.NoError(t, err)
		assert.Equal(t, "remote", addr)

		gotProtocol, err := request.Peer().Protocol()
		require.NoError(t, err)
		assert.Equal(t, protocol, gotProtocol)
	})

	t.Run("applies response metadata", func(t *testing.T) {
		transport := &fakeServerTransportStream{method: "/service/Get"}
		ctx := newServerContext(transport)
		response := tego.NewResponse("response")
		response.Header().Set("X-Response", "yes")
		response.Trailer().Set("X-Trailer", "done")

		require.NoError(t, ApplyResponseMetadata(ctx, response))

		assert.Equal(t, "yes", firstMD(transport.header, "x-response"))
		assert.Equal(t, "done", firstMD(transport.trailer, "x-trailer"))
	})
}

func TestHandlerStreams(t *testing.T) {
	t.Run("receives mapped request", func(t *testing.T) {
		stream := newFakeGRPCServerStream(ptr("request"))
		adapted := NewServerBidiStream(stream, bidiSpec(), nativeToString, stringToNative)

		got, err := adapted.Receive()

		require.NoError(t, err)
		assert.Equal(t, "request-tego", got)
	})

	t.Run("sends mapped response", func(t *testing.T) {
		stream := newFakeGRPCServerStream()
		adapted := NewServerBidiStream(stream, bidiSpec(), nativeToString, stringToNative)

		require.NoError(t, adapted.Send("response"))

		require.Len(t, stream.sent, 1)
		assert.Equal(t, "response-native", *stream.sent[0])
	})

	t.Run("sends response headers", func(t *testing.T) {
		stream := newFakeGRPCServerStream()
		adapted := NewServerBidiStream(stream, bidiSpec(), nativeToString, stringToNative)
		adapted.ResponseHeader().Set("X-Response", "yes")

		require.NoError(t, adapted.Send("response"))

		assert.Equal(t, "yes", firstMD(stream.header, "x-response"))
	})

	t.Run("applies response trailers", func(t *testing.T) {
		stream := newFakeGRPCServerStream()
		adapted := NewServerBidiStream(stream, bidiSpec(), nativeToString, stringToNative)
		adapted.ResponseTrailer().Set("X-Trailer", "done")

		require.NoError(t, ApplyStreamResponseMetadata(stream, adapted))

		assert.Equal(t, "done", firstMD(stream.trailer, "x-trailer"))
	})
}

func TestClientStreams(t *testing.T) {
	t.Run("server stream receives mapped response", func(t *testing.T) {
		stream := newFakeGRPCClientStream(withReceived(ptr("response")))
		adapted := NewClientRecvStream(stream, bidiSpec(), nativeToString)

		got, err := adapted.Receive()

		require.NoError(t, err)
		assert.Equal(t, "response-tego", got)
	})

	t.Run("server stream copies response headers", func(t *testing.T) {
		stream := newFakeGRPCClientStream(
			withHeader(metadata.Pairs("x-response", "yes")),
			withReceived(ptr("response")),
		)
		adapted := NewClientRecvStream(stream, bidiSpec(), nativeToString)

		_, err := adapted.Receive()
		require.NoError(t, err)

		assert.Equal(t, "yes", adapted.ResponseHeader().Get("x-response"))
	})

	t.Run("server stream closes native stream", func(t *testing.T) {
		stream := newFakeGRPCClientStream()
		adapted := NewClientRecvStream(stream, bidiSpec(), nativeToString)

		require.NoError(t, adapted.Close())

		assert.True(t, stream.closed)
	})

	t.Run("client stream sends mapped request", func(t *testing.T) {
		stream := newFakeGRPCClientStream(withResponse(ptr("response")))
		adapted := NewClientSendStream(stream, bidiSpec(), stringToNative, nativeToString)

		require.NoError(t, adapted.Send("request"))

		require.Len(t, stream.sent, 1)
		assert.Equal(t, "request-native", *stream.sent[0])
	})

	t.Run("client stream receives mapped response", func(t *testing.T) {
		stream := newFakeGRPCClientStream(withResponse(ptr("response")))
		adapted := NewClientSendStream(stream, bidiSpec(), stringToNative, nativeToString)

		response, err := adapted.CloseAndReceive()

		require.NoError(t, err)
		assert.Equal(t, "response-tego", response.Message)
	})

	t.Run("client stream exposes request header snapshot", func(t *testing.T) {
		callHeader := tego.Metadata{"authorization": {"token"}}
		requestHeader := callHeader.Clone()
		stream := newFakeGRPCClientStream()
		adapted := NewClientSendStream(
			stream,
			bidiSpec(),
			stringToNative,
			nativeToString,
			tego.WithStreamRequestHeader(requestHeader),
		)

		adapted.RequestHeader().Set("x-live", "yes")

		assert.Equal(t, "token", adapted.RequestHeader().Get("authorization"))
		assert.Empty(t, callHeader.Get("x-live"))
	})

	t.Run("bidi stream sends mapped request", func(t *testing.T) {
		stream := newFakeGRPCClientStream()
		adapted := NewClientBidiStream(stream, bidiSpec(), stringToNative, nativeToString)

		require.NoError(t, adapted.Send("request"))

		require.Len(t, stream.sent, 1)
		assert.Equal(t, "request-native", *stream.sent[0])
	})

	t.Run("bidi stream receives mapped response", func(t *testing.T) {
		stream := newFakeGRPCClientStream(withReceived(ptr("response")))
		adapted := NewClientBidiStream(stream, bidiSpec(), stringToNative, nativeToString)

		got, err := adapted.Receive()

		require.NoError(t, err)
		assert.Equal(t, "response-tego", got)
	})

	t.Run("bidi stream does not support closing response side", func(t *testing.T) {
		stream := newFakeGRPCClientStream()
		adapted := NewClientBidiStream(stream, bidiSpec(), stringToNative, nativeToString)

		err := adapted.CloseResponse()

		require.ErrorIs(t, err, tego.ErrUnsupported)
	})
}

func ptr(value string) *string {
	return &value
}

func nativeToString(value *string) (string, error) {
	return *value + "-tego", nil
}

func stringToNative(value string) (*string, error) {
	return ptr(value + "-native"), nil
}

func bidiSpec() tego.Spec {
	return tego.Spec{Procedure: "/service/Bidi", StreamType: tego.StreamTypeBidiStreaming}
}

func firstMD(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func newFakeGRPCServerStream(received ...*string) *fakeGRPCServerStream {
	return &fakeGRPCServerStream{
		ctx:      newServerContext(&fakeServerTransportStream{method: "/service/Bidi"}),
		received: received,
	}
}

type fakeGRPCClientStreamOption func(*fakeGRPCClientStream)

func newFakeGRPCClientStream(opts ...fakeGRPCClientStreamOption) *fakeGRPCClientStream {
	stream := &fakeGRPCClientStream{}
	for _, opt := range opts {
		opt(stream)
	}
	return stream
}

func withHeader(header metadata.MD) fakeGRPCClientStreamOption {
	return func(stream *fakeGRPCClientStream) {
		stream.header = header
	}
}

func withReceived(messages ...*string) fakeGRPCClientStreamOption {
	return func(stream *fakeGRPCClientStream) {
		stream.received = messages
	}
}

func withResponse(response *string) fakeGRPCClientStreamOption {
	return func(stream *fakeGRPCClientStream) {
		stream.response = response
	}
}

func newServerContext(transport *fakeServerTransportStream) context.Context {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request", "one"))
	ctx = grpcpeer.NewContext(ctx, &grpcpeer.Peer{
		Addr:      testAddr("remote"),
		LocalAddr: testAddr("local"),
		AuthInfo:  testAuthInfo("tls"),
	})
	return grpc.NewContextWithServerTransportStream(ctx, transport)
}

type testAddr string

func (a testAddr) Network() string { return "test" }
func (a testAddr) String() string  { return string(a) }

var _ net.Addr = testAddr("")

type testAuthInfo string

func (a testAuthInfo) AuthType() string {
	return string(a)
}

type fakeServerTransportStream struct {
	method  string
	header  metadata.MD
	trailer metadata.MD
}

func (s *fakeServerTransportStream) Method() string { return s.method }
func (s *fakeServerTransportStream) SetHeader(md metadata.MD) error {
	s.header = metadata.Join(s.header, md)
	return nil
}

func (s *fakeServerTransportStream) SendHeader(md metadata.MD) error {
	s.header = metadata.Join(s.header, md)
	return nil
}

func (s *fakeServerTransportStream) SetTrailer(md metadata.MD) error {
	s.trailer = metadata.Join(s.trailer, md)
	return nil
}

type fakeGRPCServerStream struct {
	ctx      context.Context
	header   metadata.MD
	trailer  metadata.MD
	sent     []*string
	received []*string
}

func (s *fakeGRPCServerStream) SetHeader(md metadata.MD) error {
	s.header = metadata.Join(s.header, md)
	return nil
}

func (s *fakeGRPCServerStream) SendHeader(md metadata.MD) error {
	s.header = metadata.Join(s.header, md)
	return nil
}

func (s *fakeGRPCServerStream) SetTrailer(md metadata.MD) {
	s.trailer = metadata.Join(s.trailer, md)
}
func (s *fakeGRPCServerStream) Context() context.Context { return s.ctx }
func (s *fakeGRPCServerStream) SendMsg(any) error        { return nil }
func (s *fakeGRPCServerStream) RecvMsg(any) error        { return io.EOF }
func (s *fakeGRPCServerStream) Send(message *string) error {
	s.sent = append(s.sent, message)
	return nil
}

func (s *fakeGRPCServerStream) Recv() (*string, error) {
	if len(s.received) == 0 {
		return nil, io.EOF
	}
	message := s.received[0]
	s.received = s.received[1:]
	return message, nil
}

type fakeGRPCClientStream struct {
	ctx      context.Context
	header   metadata.MD
	trailer  metadata.MD
	sent     []*string
	received []*string
	response *string
	closed   bool
}

func (s *fakeGRPCClientStream) Header() (metadata.MD, error) { return s.header, nil }
func (s *fakeGRPCClientStream) Trailer() metadata.MD         { return s.trailer }
func (s *fakeGRPCClientStream) CloseSend() error {
	s.closed = true
	return nil
}

func (s *fakeGRPCClientStream) Context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func (s *fakeGRPCClientStream) SendMsg(message any) error {
	native, ok := message.(*string)
	if !ok {
		return nil
	}
	return s.Send(native)
}

func (s *fakeGRPCClientStream) RecvMsg(message any) error {
	native, ok := message.(*string)
	if !ok {
		return nil
	}
	received, err := s.Recv()
	if err != nil {
		return err
	}
	*native = *received
	return nil
}

func (s *fakeGRPCClientStream) Send(message *string) error {
	s.sent = append(s.sent, message)
	return nil
}

func (s *fakeGRPCClientStream) Recv() (*string, error) {
	if len(s.received) == 0 {
		return nil, io.EOF
	}
	message := s.received[0]
	s.received = s.received[1:]
	return message, nil
}

func (s *fakeGRPCClientStream) CloseAndRecv() (*string, error) {
	s.closed = true
	if s.response == nil {
		return nil, errors.New("missing response")
	}
	return s.response, nil
}
