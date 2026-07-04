# Custom Types

This example shows Tego mapping to user-owned Go types through explicit conversion functions.

Tego allows you to specify `go_type` overrides at both the field and message level.

There are a couple of generic examples in here too. `labels` maps a repeated protobuf `string` field
to a `types.Set[types.Label]`. `contact_aliases` is intentionally more dramatic: it maps the same
protobuf shape to `types.Box[*[]*types.Email]`, using `Box[*[]*T]` in the proto option and then
binding `T` to `types.Email`.

That second one is a bit of extreme example, but it's just trying to highlight that Tego's `go_type`
references can carry real generic type expressions, including pointers, slices, and type arguments
inside each other (though, not much more, for now).

Good files to start with:

- `proto/custom/v1/custom.proto`
- `types/types.go`
- `custom/custom.tego.go`

From this example folder, regenerate with:

```sh
buf generate --config ../../buf.dev.yaml --template buf.gen.yaml ../.. --path proto/custom/v1/custom.proto
```
