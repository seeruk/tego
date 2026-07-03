package tego

// Request wraps a request message with RPC metadata.
type Request[T any] struct {
	Message T

	header     Metadata
	peer       Peer
	spec       Spec
	query      optional[Metadata]
	httpMethod optional[string]
	native     optional[any]
}

// requestOptions lets RequestOption stay non-generic so option helpers do not require type arguments.
type requestOptions struct {
	header     Metadata
	peer       Peer
	spec       Spec
	query      optional[Metadata]
	httpMethod optional[string]
	native     optional[any]
}

// RequestOption configures a Request.
type RequestOption func(*requestOptions)

// WithRequestHeader configures the request metadata map.
func WithRequestHeader(header Metadata) RequestOption {
	return func(opts *requestOptions) {
		opts.header = header
	}
}

// WithRequestPeer configures request peer information.
func WithRequestPeer(peer Peer) RequestOption {
	return func(opts *requestOptions) {
		opts.peer = peer
	}
}

// WithRequestSpec configures request procedure information.
func WithRequestSpec(spec Spec) RequestOption {
	return func(opts *requestOptions) {
		opts.spec = spec
	}
}

// WithRequestQuery configures the request query metadata map.
func WithRequestQuery(query Metadata) RequestOption {
	return func(opts *requestOptions) {
		opts.query = newOptional(query)
	}
}

// WithRequestHTTPMethod configures the request HTTP method.
func WithRequestHTTPMethod(method string) RequestOption {
	return func(opts *requestOptions) {
		opts.httpMethod = newOptional(method)
	}
}

// WithNativeRequest configures the native request value.
func WithNativeRequest(native any) RequestOption {
	return func(opts *requestOptions) {
		opts.native = newOptional(native)
	}
}

// NewRequest returns a new Request for message.
func NewRequest[T any](message T, opts ...RequestOption) *Request[T] {
	var options requestOptions
	for _, opt := range opts {
		opt(&options)
	}

	return &Request[T]{
		Message:    message,
		header:     options.header,
		peer:       options.peer,
		spec:       options.spec,
		query:      options.query,
		httpMethod: options.httpMethod,
		native:     options.native,
	}
}

// Header returns mutable request metadata.
func (r *Request[T]) Header() Metadata {
	if r.header == nil {
		r.header = make(Metadata)
	}
	return r.header
}

// Peer returns request peer information.
func (r *Request[T]) Peer() Peer {
	return r.peer
}

// Spec returns request procedure information.
func (r *Request[T]) Spec() Spec {
	return r.spec
}

// HasQuery reports whether request query metadata is available.
func (r *Request[T]) HasQuery() bool {
	return r.query.HasValue()
}

// Query returns mutable request query metadata.
func (r *Request[T]) Query() (Metadata, error) {
	query, err := valueOrUnsupported(r.query)
	if err != nil {
		return nil, err
	}
	if query == nil {
		query = make(Metadata)
		r.query.value = query
	}
	return query, nil
}

// HasHTTPMethod reports whether the HTTP method is available.
func (r *Request[T]) HasHTTPMethod() bool {
	return r.httpMethod.HasValue()
}

// HTTPMethod returns the HTTP method used for the request, when available.
func (r *Request[T]) HTTPMethod() (string, error) {
	return valueOrUnsupported(r.httpMethod)
}

// HasNative reports whether the native request value is available.
func (r *Request[T]) HasNative() bool {
	return r.native.HasValue()
}

// Native returns the native request value, when one was supplied.
func (r *Request[T]) Native() (any, error) {
	return valueOrUnsupported(r.native)
}

// Any returns the wrapped message as any.
func (r *Request[T]) Any() any {
	return r.Message
}

// Response wraps a response message with RPC metadata.
type Response[T any] struct {
	Message T

	header  Metadata
	trailer Metadata
	native  optional[any]
}

type responseOptions struct {
	header  Metadata
	trailer Metadata
	native  optional[any]
}

// ResponseOption configures a Response.
type ResponseOption func(*responseOptions)

// WithResponseHeader configures the response header metadata map.
func WithResponseHeader(header Metadata) ResponseOption {
	return func(opts *responseOptions) {
		opts.header = header
	}
}

// WithResponseTrailer configures the response trailer metadata map.
func WithResponseTrailer(trailer Metadata) ResponseOption {
	return func(opts *responseOptions) {
		opts.trailer = trailer
	}
}

// WithNativeResponse configures the native response value.
func WithNativeResponse(native any) ResponseOption {
	return func(opts *responseOptions) {
		opts.native = newOptional(native)
	}
}

// NewResponse returns a new Response for message.
func NewResponse[T any](message T, opts ...ResponseOption) *Response[T] {
	var options responseOptions
	for _, opt := range opts {
		opt(&options)
	}

	return &Response[T]{
		Message: message,
		header:  options.header,
		trailer: options.trailer,
		native:  options.native,
	}
}

// Header returns mutable response header metadata.
func (r *Response[T]) Header() Metadata {
	if r.header == nil {
		r.header = make(Metadata)
	}
	return r.header
}

// Trailer returns mutable response trailer metadata.
func (r *Response[T]) Trailer() Metadata {
	if r.trailer == nil {
		r.trailer = make(Metadata)
	}
	return r.trailer
}

// HasNative reports whether the native response value is available.
func (r *Response[T]) HasNative() bool {
	return r.native.HasValue()
}

// Native returns the native response value, when one was supplied.
func (r *Response[T]) Native() (any, error) {
	return valueOrUnsupported(r.native)
}

// Any returns the wrapped message as any.
func (r *Response[T]) Any() any {
	return r.Message
}
