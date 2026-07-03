package tego

import "testing"

func TestCall(t *testing.T) {
	t.Run("uses supplied live metadata", func(t *testing.T) {
		header := Metadata{"authorization": {"token"}}
		call := NewCall(WithCallHeader(header))

		call.Header().Set("x-live", "yes")

		if got := header.Get("x-live"); got != "yes" {
			t.Fatalf("Header() did not expose live metadata, got %q", got)
		}
	})

	t.Run("lazily initializes metadata", func(t *testing.T) {
		call := NewCall()

		call.Header().Set("x-lazy", "yes")

		if got := call.Header().Get("x-lazy"); got != "yes" {
			t.Fatalf("Header() = %q", got)
		}
	})
}
