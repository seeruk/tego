package tego

import "errors"

// ErrUnsupported is returned when an attempted operation is unsupported.
var ErrUnsupported = errors.New("unsupported operation")

// Metadata contains RPC metadata values.
//
// Metadata does not normalize keys. Transport adapters are responsible for
// applying protocol-specific key rules, such as HTTP header canonicalization or
// gRPC lowercase metadata keys.
type Metadata map[string][]string

// Get returns the first value for key.
func (m Metadata) Get(key string) string {
	values := m[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Values returns a copy of all values for key.
func (m Metadata) Values(key string) []string {
	values := m[key]
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

// Set replaces the values for key.
func (m Metadata) Set(key string, values ...string) {
	m[key] = append([]string(nil), values...)
}

// Add appends values to key.
func (m Metadata) Add(key string, values ...string) {
	m[key] = append(m[key], values...)
}

// Del removes key.
func (m Metadata) Del(key string) {
	delete(m, key)
}

// Clone returns a deep copy of m.
func (m Metadata) Clone() Metadata {
	if m == nil {
		return nil
	}

	clone := make(Metadata, len(m))
	for key, values := range m {
		clone[key] = append([]string(nil), values...)
	}
	return clone
}

// AuthInfo describes transport authentication information.
type AuthInfo interface {
	AuthType() string
}

type optional[T any] struct {
	value T
	valid bool
}

func newOptional[T any](value T) optional[T] {
	return optional[T]{value: value, valid: true}
}

func (o optional[T]) HasValue() bool {
	return o.valid
}

func (o optional[T]) GetValue() T {
	return o.value
}

func valueOrUnsupported[T any](option optional[T]) (T, error) {
	if option.HasValue() {
		return option.GetValue(), nil
	}

	var zero T
	return zero, ErrUnsupported
}

// Peer describes the other party for an RPC.
type Peer struct {
	addr      optional[string]
	localAddr optional[string]
	protocol  optional[string]
	authInfo  optional[AuthInfo]
}

// PeerOption configures a Peer.
type PeerOption func(*Peer)

// WithPeerAddr configures the remote address.
func WithPeerAddr(addr string) PeerOption {
	return func(peer *Peer) {
		peer.addr = newOptional(addr)
	}
}

// WithPeerLocalAddr configures the local address.
func WithPeerLocalAddr(addr string) PeerOption {
	return func(peer *Peer) {
		peer.localAddr = newOptional(addr)
	}
}

// WithPeerProtocol configures the peer protocol.
func WithPeerProtocol(protocol string) PeerOption {
	return func(peer *Peer) {
		peer.protocol = newOptional(protocol)
	}
}

// WithPeerAuthInfo configures peer authentication information.
func WithPeerAuthInfo(authInfo AuthInfo) PeerOption {
	return func(peer *Peer) {
		peer.authInfo = newOptional(authInfo)
	}
}

// NewPeer returns a new Peer.
func NewPeer(opts ...PeerOption) Peer {
	var peer Peer
	for _, opt := range opts {
		opt(&peer)
	}
	return peer
}

// HasAddr reports whether the remote address is available.
func (p Peer) HasAddr() bool {
	return p.addr.HasValue()
}

// Addr returns the remote address.
func (p Peer) Addr() (string, error) {
	return valueOrUnsupported(p.addr)
}

// HasLocalAddr reports whether the local address is available.
func (p Peer) HasLocalAddr() bool {
	return p.localAddr.HasValue()
}

// LocalAddr returns the local address.
func (p Peer) LocalAddr() (string, error) {
	return valueOrUnsupported(p.localAddr)
}

// HasProtocol reports whether the peer protocol is available.
func (p Peer) HasProtocol() bool {
	return p.protocol.HasValue()
}

// Protocol returns the peer protocol.
func (p Peer) Protocol() (string, error) {
	return valueOrUnsupported(p.protocol)
}

// HasAuthInfo reports whether peer authentication information is available.
func (p Peer) HasAuthInfo() bool {
	return p.authInfo.HasValue()
}

// AuthInfo returns peer authentication information.
func (p Peer) AuthInfo() (AuthInfo, error) {
	return valueOrUnsupported(p.authInfo)
}

// Spec describes an RPC procedure.
type Spec struct {
	// Procedure is the fully-qualified RPC procedure, such as "/package.Service/Method".
	Procedure string
	// StreamType describes whether the procedure uses unary or streaming messages.
	StreamType StreamType
}

// StreamType describes the streaming shape of an RPC.
type StreamType uint

const (
	StreamTypeUnary StreamType = iota
	StreamTypeClientStreaming
	StreamTypeServerStreaming
	StreamTypeBidiStreaming
)
