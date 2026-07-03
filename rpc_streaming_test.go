package tego

import (
	"io"
	"reflect"
	"testing"
)

func TestStreamMetadata(t *testing.T) {
	requestHeader := Metadata{"request": {"one"}}
	query := Metadata{"watch": {"true"}}
	responseHeader := Metadata{"response": {"two"}}
	responseTrailer := Metadata{"trailer": {"three"}}
	peer := NewPeer(WithPeerAddr("remote"), WithPeerAuthInfo(testAuthInfo("tls")))
	spec := Spec{Procedure: "/service/method", StreamType: StreamTypeBidiStreaming}
	native := new(struct{})

	stream := NewServerBidiStream(
		func() (string, error) { return "", io.EOF },
		func(string) error { return nil },
		WithStreamRequestHeader(requestHeader),
		WithStreamQuery(query),
		WithStreamResponseHeader(responseHeader),
		WithStreamResponseTrailer(responseTrailer),
		WithStreamPeer(peer),
		WithStreamSpec(spec),
		WithNativeStream(native),
	)

	stream.RequestHeader().Set("request-live", "yes")
	gotQuery, err := stream.Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	gotQuery.Set("query-live", "yes")
	stream.ResponseHeader().Set("response-live", "yes")
	stream.ResponseTrailer().Set("trailer-live", "yes")
	if requestHeader.Get("request-live") != "yes" ||
		query.Get("query-live") != "yes" ||
		responseHeader.Get("response-live") != "yes" ||
		responseTrailer.Get("trailer-live") != "yes" {
		t.Fatalf("stream metadata maps are not live")
	}
	gotNative, err := stream.Native()
	if err != nil {
		t.Fatalf("Native() error = %v", err)
	}
	if !reflect.DeepEqual(stream.Peer(), peer) || stream.Spec() != spec || gotNative != native {
		t.Fatalf("stream peer/spec/native mismatch")
	}
	if !stream.HasQuery() || !stream.HasNative() {
		t.Fatalf("missing Has... presence for supplied stream properties")
	}
	gotAuth, err := stream.Peer().AuthInfo()
	if err != nil {
		t.Fatalf("AuthInfo() error = %v", err)
	}
	if gotAuth.AuthType() != "tls" {
		t.Fatalf("stream auth type = %q", gotAuth.AuthType())
	}
}

func TestStreamOptionalProperties(t *testing.T) {
	t.Run("zero values are supported", func(t *testing.T) {
		stream := NewServerSendStream(
			func(string) error { return nil },
			WithStreamQuery(nil),
			WithNativeStream(nil),
		)

		query, err := stream.Query()
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		query.Set("lazy-query", "yes")
		if got := query.Get("lazy-query"); got != "yes" {
			t.Fatalf("Query() = %q", got)
		}
		if got, err := stream.Native(); err != nil || got != nil {
			t.Fatalf("Native() = %#v, %v", got, err)
		}
		if !stream.HasQuery() || !stream.HasNative() {
			t.Fatalf("missing Has... presence for zero stream properties")
		}
	})

	t.Run("missing properties", func(t *testing.T) {
		stream := NewServerSendStream(func(string) error { return nil })
		assertUnsupported(t, mustErr(stream.Query()))
		assertUnsupported(t, mustErr(stream.Native()))
		if stream.HasQuery() || stream.HasNative() {
			t.Fatalf("unexpected Has... presence for missing stream properties")
		}
	})

	t.Run("metadata is always available", func(t *testing.T) {
		stream := NewServerSendStream(func(string) error { return nil })
		stream.RequestHeader().Set("request", "yes")
		stream.ResponseHeader().Set("response", "yes")
		stream.ResponseTrailer().Set("trailer", "yes")
		if stream.RequestHeader().Get("request") != "yes" ||
			stream.ResponseHeader().Get("response") != "yes" ||
			stream.ResponseTrailer().Get("trailer") != "yes" {
			t.Fatalf("stream metadata not available")
		}
	})
}

func TestStreamHooks(t *testing.T) {
	t.Run("server send stream", func(t *testing.T) {
		var sent string
		stream := NewServerSendStream(func(message string) error {
			sent = message
			return nil
		})

		if err := stream.Send("ticket"); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		if sent != "ticket" {
			t.Fatalf("Send() sent %q", sent)
		}
	})

	t.Run("server recv stream", func(t *testing.T) {
		stream := NewServerRecvStream(func() (string, error) {
			return "ticket", nil
		})

		got, err := stream.Receive()
		if err != nil {
			t.Fatalf("Receive() error = %v", err)
		}
		if got != "ticket" {
			t.Fatalf("Receive() = %q", got)
		}
	})

	t.Run("server bidi stream", func(t *testing.T) {
		var sent int
		stream := NewServerBidiStream(
			func() (string, error) { return "request", nil },
			func(message int) error {
				sent = message
				return nil
			},
		)

		got, err := stream.Receive()
		if err != nil {
			t.Fatalf("Receive() error = %v", err)
		}
		if err := stream.Send(42); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		if got != "request" || sent != 42 {
			t.Fatalf("bidi hooks got receive=%q send=%d", got, sent)
		}
	})

	t.Run("client recv stream", func(t *testing.T) {
		var closed bool
		stream := NewClientRecvStream(
			func() (string, error) { return "response", nil },
			func() error {
				closed = true
				return nil
			},
		)

		got, err := stream.Receive()
		if err != nil {
			t.Fatalf("Receive() error = %v", err)
		}
		if err := stream.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if got != "response" || !closed {
			t.Fatalf("client recv stream hooks got receive=%q closed=%v", got, closed)
		}
	})

	t.Run("client send stream", func(t *testing.T) {
		var sent string
		stream := NewClientSendStream(
			func(message string) error {
				sent = message
				return nil
			},
			func() (*Response[int], error) {
				return NewResponse(42), nil
			},
		)

		if err := stream.Send("request"); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		response, err := stream.CloseAndReceive()
		if err != nil {
			t.Fatalf("CloseAndReceive() error = %v", err)
		}
		if sent != "request" || response.Message != 42 {
			t.Fatalf("client send stream hooks got send=%q response=%d", sent, response.Message)
		}
	})

	t.Run("client bidi stream", func(t *testing.T) {
		var sent int
		var closedRequest, closedResponse bool
		stream := NewClientBidiStream(
			func(message int) error {
				sent = message
				return nil
			},
			func() (string, error) { return "response", nil },
			func() error {
				closedRequest = true
				return nil
			},
			func() error {
				closedResponse = true
				return nil
			},
		)

		if err := stream.Send(42); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		got, err := stream.Receive()
		if err != nil {
			t.Fatalf("Receive() error = %v", err)
		}
		if err := stream.CloseRequest(); err != nil {
			t.Fatalf("CloseRequest() error = %v", err)
		}
		if err := stream.CloseResponse(); err != nil {
			t.Fatalf("CloseResponse() error = %v", err)
		}
		if sent != 42 || got != "response" || !closedRequest || !closedResponse {
			t.Fatalf("client bidi hooks got send=%d receive=%q closeRequest=%v closeResponse=%v",
				sent, got, closedRequest, closedResponse)
		}
	})
}

func TestStreamMissingHooks(t *testing.T) {
	tests := map[string]func() error{
		"server send stream send": func() error {
			return NewServerSendStream[string](nil).Send("message")
		},
		"server recv stream receive": func() error {
			_, err := NewServerRecvStream[string](nil).Receive()
			return err
		},
		"server bidi stream receive": func() error {
			_, err := NewServerBidiStream[string, int](nil, nil).Receive()
			return err
		},
		"server bidi stream send": func() error {
			return NewServerBidiStream[string, int](nil, nil).Send(42)
		},
		"client recv stream receive": func() error {
			_, err := NewClientRecvStream[string](nil, nil).Receive()
			return err
		},
		"client recv stream close": func() error {
			return NewClientRecvStream[string](nil, nil).Close()
		},
		"client send stream send": func() error {
			return NewClientSendStream[string, int](nil, nil).Send("message")
		},
		"client send stream close and receive": func() error {
			_, err := NewClientSendStream[string, int](nil, nil).CloseAndReceive()
			return err
		},
		"client bidi stream send": func() error {
			return NewClientBidiStream[string, int](nil, nil, nil, nil).Send("message")
		},
		"client bidi stream receive": func() error {
			_, err := NewClientBidiStream[string, int](nil, nil, nil, nil).Receive()
			return err
		},
		"client bidi stream close request": func() error {
			return NewClientBidiStream[string, int](nil, nil, nil, nil).CloseRequest()
		},
		"client bidi stream close response": func() error {
			return NewClientBidiStream[string, int](nil, nil, nil, nil).CloseResponse()
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assertUnsupported(t, test())
		})
	}
}
