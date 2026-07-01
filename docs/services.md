# Services

Tego may eventually generate a service mapping layer, but it should not try to replace the gRPC or
Connect transport APIs.

Generated protobuf service interfaces already carry useful RPC behaviour: request metadata, response
headers, trailers, peer information, stream lifecycle, and protocol-specific error handling. A
message-only abstraction such as an iterator can make streaming APIs pleasant, but it also hides
some of that transport surface.

The preferred shape is a Tego-native service interface plus generated adapters for the concrete
transport:

- application code implements an interface in terms of Tego request and response types;
- generated Connect and/or gRPC adapters implement the protobuf service interfaces;
- adapters map requests from protobuf to Tego at the boundary;
- adapters map responses from Tego back to protobuf at the boundary;
- request, response, and stream wrappers forward transport features such as headers and trailers.

This means application code avoids hand-written mapping in every RPC method while still keeping the
underlying RPC capabilities available.

For Connect, wrappers should hold the original `connect.Request`, `connect.Response`, or stream
value rather than trying to construct replacement Connect values containing Tego messages. Connect
types contain transport state that is created by the framework.

For gRPC, the same principle applies. The generated protobuf service implementation should remain
the transport-facing type, and a generated adapter should delegate to the Tego-native service
interface.

This keeps Tego responsible for type and mapping ergonomics, while gRPC and Connect remain
responsible for transport semantics.
