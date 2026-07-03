# Services

Tego service support is an RPC transport layer with Tego-native payload types. The Tego plugin
generates service and client interfaces, plus Connect and gRPC adapters. RPC output is enabled by
default, but can be controlled with the `rpc` plugin parameter. Tego does not try to hide that an
RPC is happening, it serves as a convenience layer to avoid manual mapping function calls and
potential duplication.

The root `tego` package provides transport-neutral runtime wrappers for the RPC layer:

- `Request[T]` and `Response[T]` wrap a typed `Message` with RPC metadata
- `ServerSendStream[T]`, `ServerRecvStream[T]`, and `ServerBidiStream[In, Out]` wrap
  server-side streams
- `ClientRecvStream[T]`, `ClientSendStream[Out, In]`, and `ClientBidiStream[Out, In]` wrap
  client-side streams
- `Metadata` represents request headers, query values, response headers, and trailers;
- `Peer` and `Spec` expose call information;
- `Native()` exposes the native Connect/gRPC object when the common Tego surface is not enough

Generated service adapters use these wrappers while Connect and gRPC remain responsible for their
own transport semantics. The adapters map protobuf messages at the boundary, translate metadata, and
pass native send/receive operations into Tego stream wrappers.

Runtime adapter helpers live outside the root package so transport dependencies stay explicit:

- `github.com/seeruk/tego/rpc/tegoconnect`
- `github.com/seeruk/tego/rpc/tegogrpc`

Connect exposes metadata as HTTP headers, so Tego adapters preserve live metadata maps where the
transport exposes them. gRPC pushes metadata through contexts and `SetHeader` / `SetTrailer`, so the
gRPC adapter copies metadata and provides explicit apply helpers for response metadata.

Client-streaming and bidirectional client calls do not have a unary request envelope, so generated
Tego clients accept `CallOption` values to seed request metadata before opening the stream. For
gRPC, those headers are cloned and applied to the outgoing context before the native stream is
opened (as they're no longer modifiable after that point). The Tego stream's `RequestHeader()` then
reflects the metadata snapshot that was applied; mutating it after creation only changes the Tego
wrapper map, not the gRPC transport. Connect exposes a live stream request header and sends it with
the first `Send`, so generated Connect clients copy `CallOption` metadata into that live header
before returning the Tego stream.

Some details remain transport-specific. Common metadata surfaces are always available as Tego-owned
maps, but transport-specific properties return `ErrUnsupported` when an adapter cannot provide them.
This keeps supported zero values distinct from unsupported values. Presence helpers such as
`HasQuery()` and `HasNative()` are available for optional capabilities when callers want to branch
before fetching the value.

`Request.HTTPMethod()` and `Query()` are meaningful for HTTP-shaped transports such as Connect and
are unsupported for gRPC. gRPC exposes transport authentication through an `AuthInfo` interface on
peer information; Connect does not expose the same concept on its peer type. gRPC metadata has
lowercase key conventions and reserved `grpc-` keys.
