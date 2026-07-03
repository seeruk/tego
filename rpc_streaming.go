package tego

type streamOptions struct {
	requestHeader   Metadata
	responseHeader  Metadata
	responseTrailer Metadata
	peer            Peer
	spec            Spec
	query           optional[Metadata]
	native          optional[any]
}

// StreamOption configures a stream wrapper.
type StreamOption func(*streamOptions)

// WithStreamRequestHeader configures the stream request metadata map.
func WithStreamRequestHeader(header Metadata) StreamOption {
	return func(opts *streamOptions) {
		opts.requestHeader = header
	}
}

// WithStreamQuery configures the stream query metadata map.
func WithStreamQuery(query Metadata) StreamOption {
	return func(opts *streamOptions) {
		opts.query = newOptional(query)
	}
}

// WithStreamResponseHeader configures the stream response header metadata map.
func WithStreamResponseHeader(header Metadata) StreamOption {
	return func(opts *streamOptions) {
		opts.responseHeader = header
	}
}

// WithStreamResponseTrailer configures the stream response trailer metadata map.
func WithStreamResponseTrailer(trailer Metadata) StreamOption {
	return func(opts *streamOptions) {
		opts.responseTrailer = trailer
	}
}

// WithStreamPeer configures stream peer information.
func WithStreamPeer(peer Peer) StreamOption {
	return func(opts *streamOptions) {
		opts.peer = peer
	}
}

// WithStreamSpec configures stream procedure information.
func WithStreamSpec(spec Spec) StreamOption {
	return func(opts *streamOptions) {
		opts.spec = spec
	}
}

// WithNativeStream configures the native stream value.
func WithNativeStream(native any) StreamOption {
	return func(opts *streamOptions) {
		opts.native = newOptional(native)
	}
}

type streamBase struct {
	requestHeader   Metadata
	responseHeader  Metadata
	responseTrailer Metadata
	peer            Peer
	spec            Spec
	query           optional[Metadata]
	native          optional[any]
}

func newStreamBase(opts []StreamOption) streamBase {
	var options streamOptions
	for _, opt := range opts {
		opt(&options)
	}
	return streamBase(options)
}

func (s *streamBase) RequestHeader() Metadata {
	if s.requestHeader == nil {
		s.requestHeader = make(Metadata)
	}
	return s.requestHeader
}

func (s *streamBase) ResponseHeader() Metadata {
	if s.responseHeader == nil {
		s.responseHeader = make(Metadata)
	}
	return s.responseHeader
}

func (s *streamBase) ResponseTrailer() Metadata {
	if s.responseTrailer == nil {
		s.responseTrailer = make(Metadata)
	}
	return s.responseTrailer
}

func (s *streamBase) Peer() Peer {
	return s.peer
}

func (s *streamBase) Spec() Spec {
	return s.spec
}

func (s *streamBase) HasQuery() bool {
	return s.query.HasValue()
}

func (s *streamBase) Query() (Metadata, error) {
	query, err := valueOrUnsupported(s.query)
	if err != nil {
		return nil, err
	}
	if query == nil {
		query = make(Metadata)
		s.query.value = query
	}
	return query, nil
}

func (s *streamBase) HasNative() bool {
	return s.native.HasValue()
}

func (s *streamBase) Native() (any, error) {
	return valueOrUnsupported(s.native)
}

// ServerSendStream is the handler side of a server-streaming RPC.
type ServerSendStream[T any] struct {
	streamBase
	send func(T) error
}

// NewServerSendStream returns a new ServerSendStream.
func NewServerSendStream[T any](send func(T) error, opts ...StreamOption) *ServerSendStream[T] {
	return &ServerSendStream[T]{
		streamBase: newStreamBase(opts),
		send:       send,
	}
}

// Send sends a message to the client.
func (s *ServerSendStream[T]) Send(message T) error {
	if s.send == nil {
		return ErrUnsupported
	}
	return s.send(message)
}

// ServerRecvStream is the handler side of a client-streaming RPC.
type ServerRecvStream[T any] struct {
	streamBase
	receive func() (T, error)
}

// NewServerRecvStream returns a new ServerRecvStream.
func NewServerRecvStream[T any](receive func() (T, error), opts ...StreamOption) *ServerRecvStream[T] {
	return &ServerRecvStream[T]{
		streamBase: newStreamBase(opts),
		receive:    receive,
	}
}

// Receive receives a message from the client.
func (s *ServerRecvStream[T]) Receive() (T, error) {
	if s.receive == nil {
		var zero T
		return zero, ErrUnsupported
	}
	return s.receive()
}

// ServerBidiStream is the handler side of a bidirectional-streaming RPC.
type ServerBidiStream[In, Out any] struct {
	streamBase
	receive func() (In, error)
	send    func(Out) error
}

// NewServerBidiStream returns a new ServerBidiStream.
func NewServerBidiStream[In, Out any](
	receive func() (In, error),
	send func(Out) error,
	opts ...StreamOption,
) *ServerBidiStream[In, Out] {
	return &ServerBidiStream[In, Out]{
		streamBase: newStreamBase(opts),
		receive:    receive,
		send:       send,
	}
}

// Receive receives a message from the client.
func (s *ServerBidiStream[In, Out]) Receive() (In, error) {
	if s.receive == nil {
		var zero In
		return zero, ErrUnsupported
	}
	return s.receive()
}

// Send sends a message to the client.
func (s *ServerBidiStream[In, Out]) Send(message Out) error {
	if s.send == nil {
		return ErrUnsupported
	}
	return s.send(message)
}

// ClientRecvStream is the client side of a server-streaming RPC.
type ClientRecvStream[T any] struct {
	streamBase
	receive func() (T, error)
	close   func() error
}

// NewClientRecvStream returns a new ClientRecvStream.
func NewClientRecvStream[T any](
	receive func() (T, error),
	close func() error,
	opts ...StreamOption,
) *ClientRecvStream[T] {
	return &ClientRecvStream[T]{
		streamBase: newStreamBase(opts),
		receive:    receive,
		close:      close,
	}
}

// Receive receives a message from the server.
func (s *ClientRecvStream[T]) Receive() (T, error) {
	if s.receive == nil {
		var zero T
		return zero, ErrUnsupported
	}
	return s.receive()
}

// Close closes the stream.
func (s *ClientRecvStream[T]) Close() error {
	if s.close == nil {
		return ErrUnsupported
	}
	return s.close()
}

// ClientSendStream is the client side of a client-streaming RPC.
type ClientSendStream[Out, In any] struct {
	streamBase
	send            func(Out) error
	closeAndReceive func() (*Response[In], error)
}

// NewClientSendStream returns a new ClientSendStream.
func NewClientSendStream[Out, In any](
	send func(Out) error,
	closeAndReceive func() (*Response[In], error),
	opts ...StreamOption,
) *ClientSendStream[Out, In] {
	return &ClientSendStream[Out, In]{
		streamBase:      newStreamBase(opts),
		send:            send,
		closeAndReceive: closeAndReceive,
	}
}

// Send sends a message to the server.
func (s *ClientSendStream[Out, In]) Send(message Out) error {
	if s.send == nil {
		return ErrUnsupported
	}
	return s.send(message)
}

// CloseAndReceive closes the send side and receives the response.
func (s *ClientSendStream[Out, In]) CloseAndReceive() (*Response[In], error) {
	if s.closeAndReceive == nil {
		return nil, ErrUnsupported
	}
	return s.closeAndReceive()
}

// ClientBidiStream is the client side of a bidirectional-streaming RPC.
type ClientBidiStream[Out, In any] struct {
	streamBase
	send          func(Out) error
	receive       func() (In, error)
	closeRequest  func() error
	closeResponse func() error
}

// NewClientBidiStream returns a new ClientBidiStream.
func NewClientBidiStream[Out, In any](
	send func(Out) error,
	receive func() (In, error),
	closeRequest func() error,
	closeResponse func() error,
	opts ...StreamOption,
) *ClientBidiStream[Out, In] {
	return &ClientBidiStream[Out, In]{
		streamBase:    newStreamBase(opts),
		send:          send,
		receive:       receive,
		closeRequest:  closeRequest,
		closeResponse: closeResponse,
	}
}

// Send sends a message to the server.
func (s *ClientBidiStream[Out, In]) Send(message Out) error {
	if s.send == nil {
		return ErrUnsupported
	}
	return s.send(message)
}

// Receive receives a message from the server.
func (s *ClientBidiStream[Out, In]) Receive() (In, error) {
	if s.receive == nil {
		var zero In
		return zero, ErrUnsupported
	}
	return s.receive()
}

// CloseRequest closes the request side of the stream.
func (s *ClientBidiStream[Out, In]) CloseRequest() error {
	if s.closeRequest == nil {
		return ErrUnsupported
	}
	return s.closeRequest()
}

// CloseResponse closes the response side of the stream.
func (s *ClientBidiStream[Out, In]) CloseResponse() error {
	if s.closeResponse == nil {
		return ErrUnsupported
	}
	return s.closeResponse()
}
