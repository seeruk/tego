# Presence Patch

This example focuses on patch-style inputs, AKA "omittable nullable" fields. The types required in
plain protobuf for this scenario are a little awkward; Tego aims to simplify this into an obvious
and idiomatic form.

Tego generates `omittable.Value[T]` for patch fields and `*T` for nullable shapes, so callers can
distinguish absence from explicit null.

Good files to start with:

- `proto/patch/v1/patch.proto`
- `patch/patch.tego.go`

From this example folder, regenerate with:

```sh
buf generate
```
