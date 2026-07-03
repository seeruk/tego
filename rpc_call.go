package tego

// Call carries request metadata for RPCs that do not have a unary request envelope.
type Call struct {
	header Metadata
}

// CallOption configures a Call.
type CallOption func(*Call)

// WithCallHeader configures the call request metadata map.
func WithCallHeader(header Metadata) CallOption {
	return func(call *Call) {
		call.header = header
	}
}

// NewCall returns a new Call configured with opts.
func NewCall(opts ...CallOption) *Call {
	call := new(Call)
	for _, opt := range opts {
		opt(call)
	}
	return call
}

// Header returns mutable call request metadata.
func (c *Call) Header() Metadata {
	if c.header == nil {
		c.header = make(Metadata)
	}
	return c.header
}
