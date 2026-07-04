# Shapes

This example shows how small protobuf wrapper messages become more natural Go shapes when Tego can
infer intent.

For Tego-enabled .proto files (files with the `(tego.file).go_package` option), Tego will
automatically infer shapes. This can be explicitly disabled for a message with the following option:

```protobuf
option (tego.message).infer_shape = false;
```

Good files to start with:

- `proto/shapes/v1/shapes.proto`
- `shapes/shapes.tego.go`

From this example folder, regenerate with:

```sh
buf generate --config ../../buf.dev.yaml --template buf.gen.yaml ../.. --path proto/shapes/v1/shapes.proto
```
