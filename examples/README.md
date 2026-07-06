# Tego Examples

These examples are small on purpose. Each one keeps the proto source and generated output close
together, with handwritten usage code only where it helps, so you can read the shape of Tego
without deciphering a large demo app.

## Learning Path

1. [quickstart-grpc](quickstart-grpc): unary gRPC service using the facade interface and facade
   client.
2. [quickstart-connect](quickstart-connect): the same shape using Connect.
3. [error-mapping](error-mapping): mapping ordinary Go domain errors to gRPC and Connect errors.
4. [shapes](shapes): generated Go shapes for slices, maps, nullable values, flattening, and
   composition.
5. [options](options): naming, comments, tags, omit, nullable, omittable, enums, and service/method
   option syntax.
6. [custom-types](custom-types): custom Go types and conversion functions.
7. [presence-patch](presence-patch): patch inputs that distinguish leave unchanged, set value, and
   clear value.
8. [streaming-grpc](streaming-grpc): server, client, and bidi streaming through facade iterators.
9. [hooks](hooks): adapter-level mapping hooks for typed service hooks and reusable interface
   hooks.
10. [transport-override](transport-override): overriding one native gRPC method while delegating to
   the generated adapter.
11. [kitchen-sink](kitchen-sink): a reference-style type coverage example.

Most examples keep their own `buf.gen.yaml`, proto source, and generated output in one folder. The
gRPC and Connect quickstarts deliberately share the same generated protobuf/Tego package in
`quickstart`, with separate handwritten clients and servers in `quickstart-grpc` and
`quickstart-connect`. The runnable examples include a little handwritten Go under `cmd/`.
