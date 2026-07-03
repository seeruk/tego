package tego

import (
	"errors"
	"reflect"
	"testing"
)

type testAuthInfo string

func (a testAuthInfo) AuthType() string {
	return string(a)
}

func assertUnsupported(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want %v", err, ErrUnsupported)
	}
}

func mustErr[T any](_ T, err error) error {
	return err
}

func TestMetadata(t *testing.T) {
	metadata := Metadata{}
	metadata.Set("x-ticket", "one", "two")
	metadata.Add("x-ticket", "three")

	if got := metadata.Get("x-ticket"); got != "one" {
		t.Fatalf("Get() = %q, want %q", got, "one")
	}
	if got := metadata.Values("x-ticket"); !reflect.DeepEqual(got, []string{"one", "two", "three"}) {
		t.Fatalf("Values() = %#v", got)
	}

	values := metadata.Values("x-ticket")
	values[0] = "changed"
	if got := metadata.Get("x-ticket"); got != "one" {
		t.Fatalf("Values() exposed mutable values; Get() = %q", got)
	}

	clone := metadata.Clone()
	clone.Set("x-ticket", "clone")
	if got := metadata.Get("x-ticket"); got != "one" {
		t.Fatalf("Clone() shared values; Get() = %q", got)
	}

	metadata.Del("x-ticket")
	if got := metadata.Get("x-ticket"); got != "" {
		t.Fatalf("Get() after Del() = %q", got)
	}
}

func TestPeer(t *testing.T) {
	t.Run("supplied properties", func(t *testing.T) {
		authInfo := testAuthInfo("tls")
		peer := NewPeer(
			WithPeerAddr("remote"),
			WithPeerLocalAddr("local"),
			WithPeerProtocol("h2"),
			WithPeerAuthInfo(authInfo),
		)

		if got, err := peer.Addr(); err != nil || got != "remote" {
			t.Fatalf("Addr() = %q, %v", got, err)
		}
		if got, err := peer.LocalAddr(); err != nil || got != "local" {
			t.Fatalf("LocalAddr() = %q, %v", got, err)
		}
		if got, err := peer.Protocol(); err != nil || got != "h2" {
			t.Fatalf("Protocol() = %q, %v", got, err)
		}
		if got, err := peer.AuthInfo(); err != nil || got != authInfo {
			t.Fatalf("AuthInfo() = %#v, %v", got, err)
		}
		if !peer.HasAddr() || !peer.HasLocalAddr() || !peer.HasProtocol() || !peer.HasAuthInfo() {
			t.Fatalf("missing Has... presence for supplied peer properties")
		}
	})

	t.Run("zero values are supported", func(t *testing.T) {
		peer := NewPeer(
			WithPeerAddr(""),
			WithPeerLocalAddr(""),
			WithPeerProtocol(""),
			WithPeerAuthInfo(nil),
		)

		if got, err := peer.Addr(); err != nil || got != "" {
			t.Fatalf("Addr() = %q, %v", got, err)
		}
		if got, err := peer.LocalAddr(); err != nil || got != "" {
			t.Fatalf("LocalAddr() = %q, %v", got, err)
		}
		if got, err := peer.Protocol(); err != nil || got != "" {
			t.Fatalf("Protocol() = %q, %v", got, err)
		}
		if got, err := peer.AuthInfo(); err != nil || got != nil {
			t.Fatalf("AuthInfo() = %#v, %v", got, err)
		}
		if !peer.HasAddr() || !peer.HasLocalAddr() || !peer.HasProtocol() || !peer.HasAuthInfo() {
			t.Fatalf("missing Has... presence for zero peer properties")
		}
	})

	t.Run("missing properties", func(t *testing.T) {
		peer := NewPeer()

		if _, err := peer.Addr(); err == nil {
			t.Fatalf("Addr() error = nil")
		} else {
			assertUnsupported(t, err)
		}
		if _, err := peer.LocalAddr(); err == nil {
			t.Fatalf("LocalAddr() error = nil")
		} else {
			assertUnsupported(t, err)
		}
		if _, err := peer.Protocol(); err == nil {
			t.Fatalf("Protocol() error = nil")
		} else {
			assertUnsupported(t, err)
		}
		if _, err := peer.AuthInfo(); err == nil {
			t.Fatalf("AuthInfo() error = nil")
		} else {
			assertUnsupported(t, err)
		}
		if peer.HasAddr() || peer.HasLocalAddr() || peer.HasProtocol() || peer.HasAuthInfo() {
			t.Fatalf("unexpected Has... presence for missing peer properties")
		}
	})
}
