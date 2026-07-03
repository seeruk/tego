package tegoconnect

import (
	"io"
	"net/http"
	"net/url"
	"testing"

	"connectrpc.com/connect"
	"github.com/seeruk/tego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
	t.Run("header view is live", func(t *testing.T) {
		header := http.Header{"X-Ticket": {"one"}}
		metadata := MetadataFromHeader(header)

		metadata.Set("X-Ticket", "two")

		assert.Equal(t, "two", header.Get("X-Ticket"))
	})

	t.Run("values view is live", func(t *testing.T) {
		values := url.Values{"watch": {"true"}}
		metadata := MetadataFromValues(values)

		metadata.Set("watch", "false")

		assert.Equal(t, "false", values.Get("watch"))
	})

	t.Run("copy does not share values", func(t *testing.T) {
		source := MetadataFromHeader(http.Header{"X-Ticket": {"one"}})
		target := http.Header{}

		CopyMetadataToHeader(target, source)
		source.Set("X-Ticket", "changed")

		assert.Equal(t, "one", target.Get("X-Ticket"))
	})
}

func TestPeerSpec(t *testing.T) {
	t.Run("converts peer", func(t *testing.T) {
		peer := Peer(connect.Peer{Addr: "remote", Protocol: connect.ProtocolGRPC})

		addr, err := peer.Addr()
		require.NoError(t, err)
		assert.Equal(t, "remote", addr)

		protocol, err := peer.Protocol()
		require.NoError(t, err)
		assert.Equal(t, connect.ProtocolGRPC, protocol)
	})

	t.Run("converts spec", func(t *testing.T) {
		spec := Spec(connect.Spec{
			Procedure:  "/yirapb.v1.TicketService/WatchTicketEvents",
			StreamType: connect.StreamTypeBidi,
		})

		assert.Equal(t, tego.Spec{
			Procedure:  "/yirapb.v1.TicketService/WatchTicketEvents",
			StreamType: tego.StreamTypeBidiStreaming,
		}, spec)
	})

	t.Run("converts stream types", func(t *testing.T) {
		tests := map[string]struct {
			streamType connect.StreamType
			want       tego.StreamType
		}{
			"unary":  {streamType: connect.StreamTypeUnary, want: tego.StreamTypeUnary},
			"client": {streamType: connect.StreamTypeClient, want: tego.StreamTypeClientStreaming},
			"server": {streamType: connect.StreamTypeServer, want: tego.StreamTypeServerStreaming},
			"bidi":   {streamType: connect.StreamTypeBidi, want: tego.StreamTypeBidiStreaming},
		}

		for name, tt := range tests {
			t.Run(name, func(t *testing.T) {
				assert.Equal(t, tt.want, StreamType(tt.streamType))
			})
		}
	})
}

func TestUnary(t *testing.T) {
	t.Run("adapts request", func(t *testing.T) {
		request := NewRequest("request", connect.NewRequest(new(string)))

		assert.Equal(t, "request", request.Message)
		assert.True(t, request.HasQuery())
		assert.True(t, request.HasHTTPMethod())
		assert.True(t, request.HasNative())
	})

	t.Run("request headers are live", func(t *testing.T) {
		native := connect.NewRequest(new(string))
		request := NewRequest("request", native)

		request.Header().Set("X-Request", "two")

		assert.Equal(t, "two", native.Header().Get("X-Request"))
	})

	t.Run("adapts native request", func(t *testing.T) {
		message := new(string)
		request := tego.NewRequest("request")
		request.Header().Set("X-Request", "one")

		native := NewNativeRequest(message, request)
		request.Header().Set("X-Request", "changed")

		assert.Same(t, message, native.Msg)
		assert.Equal(t, "one", native.Header().Get("X-Request"))
	})

	t.Run("adapts response headers", func(t *testing.T) {
		native := NewNativeResponse(new(string), NewResponse("response", connect.NewResponse(new(string))))
		response := NewResponse("adapted", native)

		response.Header().Set("X-Response", "two")

		assert.Equal(t, "two", native.Header().Get("X-Response"))
	})
}

func TestHandlerStreams(t *testing.T) {
	t.Run("server stream sends mapped response", func(t *testing.T) {
		stream := newFakeHandlerServerStream()
		adapted := NewServerSendStream(stream, stringToNative)

		require.NoError(t, adapted.Send("ticket"))

		require.Len(t, stream.sent, 1)
		assert.Equal(t, "ticket-native", *stream.sent[0])
	})

	t.Run("server stream exposes response headers", func(t *testing.T) {
		stream := newFakeHandlerServerStream()
		adapted := NewServerSendStream(stream, stringToNative)

		adapted.ResponseHeader().Set("X-Response", "yes")

		assert.Equal(t, "yes", stream.responseHeader.Get("X-Response"))
	})

	t.Run("client stream receives mapped request", func(t *testing.T) {
		stream := newFakeHandlerClientStream(ptr("ticket"))
		adapted := NewServerRecvStream(stream, nativeToString)

		got, err := adapted.Receive()

		require.NoError(t, err)
		assert.Equal(t, "ticket-tego", got)
	})

	t.Run("client stream returns EOF", func(t *testing.T) {
		stream := newFakeHandlerClientStream()
		adapted := NewServerRecvStream(stream, nativeToString)

		_, err := adapted.Receive()

		require.ErrorIs(t, err, io.EOF)
	})

	t.Run("bidi stream receives mapped request", func(t *testing.T) {
		stream := newFakeHandlerBidiStream(ptr("request"))
		adapted := NewServerBidiStream(stream, nativeToString, stringToNative)

		got, err := adapted.Receive()

		require.NoError(t, err)
		assert.Equal(t, "request-tego", got)
	})

	t.Run("bidi stream sends mapped response", func(t *testing.T) {
		stream := newFakeHandlerBidiStream()
		adapted := NewServerBidiStream(stream, nativeToString, stringToNative)

		require.NoError(t, adapted.Send("response"))

		require.Len(t, stream.sent, 1)
		assert.Equal(t, "response-native", *stream.sent[0])
	})
}

func TestClientStreams(t *testing.T) {
	t.Run("server stream receives mapped response", func(t *testing.T) {
		stream := newFakeClientServerStream(ptr("response"))
		adapted := NewClientRecvStream(stream, nativeToString)

		got, err := adapted.Receive()

		require.NoError(t, err)
		assert.Equal(t, "response-tego", got)
	})

	t.Run("server stream closes native stream", func(t *testing.T) {
		stream := newFakeClientServerStream()
		adapted := NewClientRecvStream(stream, nativeToString)

		require.NoError(t, adapted.Close())

		assert.True(t, stream.closed)
	})

	t.Run("client stream sends mapped request", func(t *testing.T) {
		stream := newFakeClientClientStream(ptr("response"))
		adapted := NewClientSendStream(stream, stringToNative, nativeToString)

		require.NoError(t, adapted.Send("request"))

		require.Len(t, stream.sent, 1)
		assert.Equal(t, "request-native", *stream.sent[0])
	})

	t.Run("client stream receives mapped response", func(t *testing.T) {
		stream := newFakeClientClientStream(ptr("response"))
		adapted := NewClientSendStream(stream, stringToNative, nativeToString)

		response, err := adapted.CloseAndReceive()

		require.NoError(t, err)
		assert.Equal(t, "response-tego", response.Message)
	})

	t.Run("bidi stream sends mapped request", func(t *testing.T) {
		stream := newFakeClientBidiStream()
		adapted := NewClientBidiStream(stream, stringToNative, nativeToString)

		require.NoError(t, adapted.Send("request"))

		require.Len(t, stream.sent, 1)
		assert.Equal(t, "request-native", *stream.sent[0])
	})

	t.Run("bidi stream does not read response metadata at construction", func(t *testing.T) {
		stream := newFakeClientBidiStream()

		_ = NewClientBidiStream(stream, stringToNative, nativeToString)

		assert.Zero(t, stream.responseHeaderCalls)
		assert.Zero(t, stream.responseTrailerCalls)
	})

	t.Run("bidi stream receives mapped response", func(t *testing.T) {
		stream := newFakeClientBidiStream(ptr("response"))
		adapted := NewClientBidiStream(stream, stringToNative, nativeToString)

		got, err := adapted.Receive()

		require.NoError(t, err)
		assert.Equal(t, "response-tego", got)
	})

	t.Run("bidi stream closes native stream", func(t *testing.T) {
		stream := newFakeClientBidiStream()
		adapted := NewClientBidiStream(stream, stringToNative, nativeToString)

		require.NoError(t, adapted.CloseRequest())
		require.NoError(t, adapted.CloseResponse())

		assert.True(t, stream.closedRequest)
		assert.True(t, stream.closedResponse)
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

type fakeHandlerConn struct {
	spec            connect.Spec
	peer            connect.Peer
	requestHeader   http.Header
	responseHeader  http.Header
	responseTrailer http.Header
}

func newFakeHandlerConn() *fakeHandlerConn {
	return &fakeHandlerConn{
		spec:            connect.Spec{Procedure: "/service/method", StreamType: connect.StreamTypeBidi},
		peer:            connect.Peer{Addr: "remote", Protocol: connect.ProtocolConnect, Query: url.Values{"watch": {"true"}}},
		requestHeader:   http.Header{"X-Request": {"one"}},
		responseHeader:  http.Header{},
		responseTrailer: http.Header{},
	}
}

func (c *fakeHandlerConn) Spec() connect.Spec           { return c.spec }
func (c *fakeHandlerConn) Peer() connect.Peer           { return c.peer }
func (c *fakeHandlerConn) Receive(any) error            { return io.EOF }
func (c *fakeHandlerConn) RequestHeader() http.Header   { return c.requestHeader }
func (c *fakeHandlerConn) Send(any) error               { return nil }
func (c *fakeHandlerConn) ResponseHeader() http.Header  { return c.responseHeader }
func (c *fakeHandlerConn) ResponseTrailer() http.Header { return c.responseTrailer }

type fakeHandlerServerStream struct {
	conn            *fakeHandlerConn
	sent            []*string
	responseHeader  http.Header
	responseTrailer http.Header
}

func newFakeHandlerServerStream() *fakeHandlerServerStream {
	return &fakeHandlerServerStream{conn: newFakeHandlerConn()}
}

func (s *fakeHandlerServerStream) Conn() connect.StreamingHandlerConn { return s.conn }
func (s *fakeHandlerServerStream) Send(message *string) error {
	s.sent = append(s.sent, message)
	return nil
}

func (s *fakeHandlerServerStream) ResponseHeader() http.Header {
	if s.responseHeader == nil {
		s.responseHeader = http.Header{}
	}
	return s.responseHeader
}

func (s *fakeHandlerServerStream) ResponseTrailer() http.Header {
	if s.responseTrailer == nil {
		s.responseTrailer = http.Header{}
	}
	return s.responseTrailer
}

type fakeHandlerClientStream struct {
	conn     *fakeHandlerConn
	messages []*string
	msg      *string
	err      error
}

func newFakeHandlerClientStream(messages ...*string) *fakeHandlerClientStream {
	return &fakeHandlerClientStream{
		conn:     newFakeHandlerConn(),
		messages: messages,
	}
}

func (s *fakeHandlerClientStream) Conn() connect.StreamingHandlerConn { return s.conn }
func (s *fakeHandlerClientStream) Receive() bool {
	if len(s.messages) == 0 {
		return false
	}
	s.msg = s.messages[0]
	s.messages = s.messages[1:]
	return true
}
func (s *fakeHandlerClientStream) Msg() *string               { return s.msg }
func (s *fakeHandlerClientStream) Err() error                 { return s.err }
func (s *fakeHandlerClientStream) RequestHeader() http.Header { return s.conn.requestHeader }
func (s *fakeHandlerClientStream) Peer() connect.Peer         { return s.conn.peer }
func (s *fakeHandlerClientStream) Spec() connect.Spec         { return s.conn.spec }

type fakeHandlerBidiStream struct {
	conn     *fakeHandlerConn
	messages []*string
	sent     []*string
}

func newFakeHandlerBidiStream(messages ...*string) *fakeHandlerBidiStream {
	return &fakeHandlerBidiStream{
		conn:     newFakeHandlerConn(),
		messages: messages,
	}
}

func (s *fakeHandlerBidiStream) Conn() connect.StreamingHandlerConn { return s.conn }
func (s *fakeHandlerBidiStream) Receive() (*string, error) {
	if len(s.messages) == 0 {
		return nil, io.EOF
	}
	message := s.messages[0]
	s.messages = s.messages[1:]
	return message, nil
}

func (s *fakeHandlerBidiStream) Send(message *string) error {
	s.sent = append(s.sent, message)
	return nil
}
func (s *fakeHandlerBidiStream) RequestHeader() http.Header   { return s.conn.requestHeader }
func (s *fakeHandlerBidiStream) ResponseHeader() http.Header  { return s.conn.responseHeader }
func (s *fakeHandlerBidiStream) ResponseTrailer() http.Header { return s.conn.responseTrailer }
func (s *fakeHandlerBidiStream) Peer() connect.Peer           { return s.conn.peer }
func (s *fakeHandlerBidiStream) Spec() connect.Spec           { return s.conn.spec }

type fakeClientConn struct {
	spec            connect.Spec
	peer            connect.Peer
	requestHeader   http.Header
	responseHeader  http.Header
	responseTrailer http.Header
}

func newFakeClientConn() *fakeClientConn {
	return &fakeClientConn{
		spec:            connect.Spec{Procedure: "/service/method", StreamType: connect.StreamTypeBidi},
		peer:            connect.Peer{Addr: "server", Protocol: connect.ProtocolGRPC},
		requestHeader:   http.Header{},
		responseHeader:  http.Header{},
		responseTrailer: http.Header{},
	}
}

func (c *fakeClientConn) Spec() connect.Spec           { return c.spec }
func (c *fakeClientConn) Peer() connect.Peer           { return c.peer }
func (c *fakeClientConn) Send(any) error               { return nil }
func (c *fakeClientConn) RequestHeader() http.Header   { return c.requestHeader }
func (c *fakeClientConn) CloseRequest() error          { return nil }
func (c *fakeClientConn) Receive(any) error            { return io.EOF }
func (c *fakeClientConn) ResponseHeader() http.Header  { return c.responseHeader }
func (c *fakeClientConn) ResponseTrailer() http.Header { return c.responseTrailer }
func (c *fakeClientConn) CloseResponse() error         { return nil }

type fakeClientServerStream struct {
	conn     *fakeClientConn
	messages []*string
	msg      *string
	closed   bool
}

func newFakeClientServerStream(messages ...*string) *fakeClientServerStream {
	return &fakeClientServerStream{
		conn:     newFakeClientConn(),
		messages: messages,
	}
}

func (s *fakeClientServerStream) Conn() (connect.StreamingClientConn, error) { return s.conn, nil }
func (s *fakeClientServerStream) Receive() bool {
	if len(s.messages) == 0 {
		return false
	}
	s.msg = s.messages[0]
	s.messages = s.messages[1:]
	return true
}
func (s *fakeClientServerStream) Msg() *string                 { return s.msg }
func (s *fakeClientServerStream) Err() error                   { return nil }
func (s *fakeClientServerStream) ResponseHeader() http.Header  { return s.conn.responseHeader }
func (s *fakeClientServerStream) ResponseTrailer() http.Header { return s.conn.responseTrailer }
func (s *fakeClientServerStream) Close() error {
	s.closed = true
	return nil
}

type fakeClientClientStream struct {
	conn     *fakeClientConn
	sent     []*string
	response *string
}

func newFakeClientClientStream(response *string) *fakeClientClientStream {
	return &fakeClientClientStream{
		conn:     newFakeClientConn(),
		response: response,
	}
}

func (s *fakeClientClientStream) Conn() (connect.StreamingClientConn, error) { return s.conn, nil }
func (s *fakeClientClientStream) Send(message *string) error {
	s.sent = append(s.sent, message)
	return nil
}

func (s *fakeClientClientStream) CloseAndReceive() (*connect.Response[string], error) {
	return connect.NewResponse(s.response), nil
}
func (s *fakeClientClientStream) RequestHeader() http.Header { return s.conn.requestHeader }
func (s *fakeClientClientStream) Peer() connect.Peer         { return s.conn.peer }
func (s *fakeClientClientStream) Spec() connect.Spec         { return s.conn.spec }

type fakeClientBidiStream struct {
	conn                 *fakeClientConn
	sent                 []*string
	messages             []*string
	closedRequest        bool
	closedResponse       bool
	responseHeaderCalls  int
	responseTrailerCalls int
}

func newFakeClientBidiStream(messages ...*string) *fakeClientBidiStream {
	return &fakeClientBidiStream{
		conn:     newFakeClientConn(),
		messages: messages,
	}
}

func (s *fakeClientBidiStream) Conn() (connect.StreamingClientConn, error) { return s.conn, nil }
func (s *fakeClientBidiStream) Send(message *string) error {
	s.sent = append(s.sent, message)
	return nil
}

func (s *fakeClientBidiStream) Receive() (*string, error) {
	if len(s.messages) == 0 {
		return nil, io.EOF
	}
	message := s.messages[0]
	s.messages = s.messages[1:]
	return message, nil
}

func (s *fakeClientBidiStream) CloseRequest() error {
	s.closedRequest = true
	return nil
}

func (s *fakeClientBidiStream) CloseResponse() error {
	s.closedResponse = true
	return nil
}
func (s *fakeClientBidiStream) RequestHeader() http.Header { return s.conn.requestHeader }
func (s *fakeClientBidiStream) ResponseHeader() http.Header {
	s.responseHeaderCalls++
	return s.conn.responseHeader
}

func (s *fakeClientBidiStream) ResponseTrailer() http.Header {
	s.responseTrailerCalls++
	return s.conn.responseTrailer
}
func (s *fakeClientBidiStream) Peer() connect.Peer { return s.conn.peer }
func (s *fakeClientBidiStream) Spec() connect.Spec { return s.conn.spec }
