package tego

import (
	"reflect"
	"testing"
)

func TestRequest(t *testing.T) {
	header := Metadata{"authorization": {"token"}}
	query := Metadata{"watch": {"true"}}
	peer := NewPeer(
		WithPeerAddr("remote"),
		WithPeerLocalAddr("local"),
		WithPeerProtocol("connect"),
		WithPeerAuthInfo(testAuthInfo("tls")),
	)
	spec := Spec{Procedure: "/yirapb.v1.TicketService/GetTicket", StreamType: StreamTypeUnary}
	native := new(struct{})

	request := NewRequest(
		"message",
		WithRequestHeader(header),
		WithRequestQuery(query),
		WithRequestPeer(peer),
		WithRequestSpec(spec),
		WithRequestHTTPMethod("POST"),
		WithNativeRequest(native),
	)

	request.Header().Set("x-live", "yes")
	gotQuery, err := request.Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	gotQuery.Set("query-live", "yes")
	if request.Message != "message" || request.Any() != "message" {
		t.Fatalf("request message mismatch")
	}
	if got := header.Get("x-live"); got != "yes" {
		t.Fatalf("Header() did not expose live metadata, got %q", got)
	}
	if got := query.Get("query-live"); got != "yes" {
		t.Fatalf("Query() did not expose live metadata, got %q", got)
	}
	if !reflect.DeepEqual(request.Peer(), peer) || request.Spec() != spec {
		t.Fatalf("request peer/spec mismatch")
	}
	gotAuth, err := request.Peer().AuthInfo()
	if err != nil {
		t.Fatalf("AuthInfo() error = %v", err)
	}
	if gotAuth.AuthType() != "tls" {
		t.Fatalf("request auth type = %q", gotAuth.AuthType())
	}
	gotMethod, err := request.HTTPMethod()
	if err != nil {
		t.Fatalf("HTTPMethod() error = %v", err)
	}
	gotNative, err := request.Native()
	if err != nil {
		t.Fatalf("Native() error = %v", err)
	}
	if gotMethod != "POST" || gotNative != native {
		t.Fatalf("request method/native mismatch")
	}
	if !request.HasQuery() || !request.HasHTTPMethod() || !request.HasNative() {
		t.Fatalf("missing Has... presence for supplied request properties")
	}
}

func TestRequestOptionalProperties(t *testing.T) {
	t.Run("zero values are supported", func(t *testing.T) {
		request := NewRequest(
			"message",
			WithRequestQuery(nil),
			WithRequestHTTPMethod(""),
			WithNativeRequest(nil),
		)

		query, err := request.Query()
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		query.Set("lazy-query", "yes")
		if got := query.Get("lazy-query"); got != "yes" {
			t.Fatalf("Query() = %q", got)
		}
		if got, err := request.HTTPMethod(); err != nil || got != "" {
			t.Fatalf("HTTPMethod() = %q, %v", got, err)
		}
		if got, err := request.Native(); err != nil || got != nil {
			t.Fatalf("Native() = %#v, %v", got, err)
		}
		if !request.HasQuery() || !request.HasHTTPMethod() || !request.HasNative() {
			t.Fatalf("missing Has... presence for zero request properties")
		}
	})

	t.Run("missing properties", func(t *testing.T) {
		request := NewRequest("message")

		assertUnsupported(t, mustErr(request.Query()))
		assertUnsupported(t, mustErr(request.HTTPMethod()))
		assertUnsupported(t, mustErr(request.Native()))
		if request.HasQuery() || request.HasHTTPMethod() || request.HasNative() {
			t.Fatalf("unexpected Has... presence for missing request properties")
		}
	})

	t.Run("header is always available", func(t *testing.T) {
		request := NewRequest("lazy")
		request.Header().Set("x-lazy", "yes")
		if got := request.Header().Get("x-lazy"); got != "yes" {
			t.Fatalf("Header() = %q", got)
		}
	})
}

func TestResponse(t *testing.T) {
	header := Metadata{"x-header": {"one"}}
	trailer := Metadata{"x-trailer": {"two"}}
	native := new(struct{})

	response := NewResponse(
		"message",
		WithResponseHeader(header),
		WithResponseTrailer(trailer),
		WithNativeResponse(native),
	)

	response.Header().Set("x-live-header", "yes")
	response.Trailer().Set("x-live-trailer", "yes")
	if response.Message != "message" || response.Any() != "message" {
		t.Fatalf("response message mismatch")
	}
	if got := header.Get("x-live-header"); got != "yes" {
		t.Fatalf("Header() did not expose live metadata, got %q", got)
	}
	if got := trailer.Get("x-live-trailer"); got != "yes" {
		t.Fatalf("Trailer() did not expose live metadata, got %q", got)
	}
	gotNative, err := response.Native()
	if err != nil {
		t.Fatalf("Native() error = %v", err)
	}
	if gotNative != native {
		t.Fatalf("response native mismatch")
	}
	if !response.HasNative() {
		t.Fatalf("missing HasNative presence for supplied response native")
	}
}

func TestResponseOptionalProperties(t *testing.T) {
	t.Run("zero native is supported", func(t *testing.T) {
		response := NewResponse("message", WithNativeResponse(nil))
		if got, err := response.Native(); err != nil || got != nil {
			t.Fatalf("Native() = %#v, %v", got, err)
		}
		if !response.HasNative() {
			t.Fatalf("missing HasNative presence for zero response native")
		}
	})

	t.Run("missing native", func(t *testing.T) {
		response := NewResponse("message")
		assertUnsupported(t, mustErr(response.Native()))
		if response.HasNative() {
			t.Fatalf("unexpected HasNative presence for missing response native")
		}
	})

	t.Run("metadata is always available", func(t *testing.T) {
		response := NewResponse("lazy")
		response.Header().Set("x-lazy-header", "yes")
		response.Trailer().Set("x-lazy-trailer", "yes")
		if got := response.Header().Get("x-lazy-header"); got != "yes" {
			t.Fatalf("Header() = %q", got)
		}
		if got := response.Trailer().Get("x-lazy-trailer"); got != "yes" {
			t.Fatalf("Trailer() = %q", got)
		}
	})
}
