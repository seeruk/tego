# Custom Types

This example shows Tego mapping to user-owned Go types through explicit conversion functions.

Tego allows you to specify `go_type` overrides at both the field and message level.

There are generic and container examples in here too. `labels` maps a repeated protobuf `string`
field to a `types.Set[types.Label]`. `contact_aliases` is intentionally quite extreme: it maps the
protobuf generated string slice to `types.Box[*[]*types.Email]`, using `Box[*[]*T]` in the proto 
option and then binding `T` to `types.Email`. The numeric fields demonstrate direct `[12]uint` and
`map[string]uint` expressions, plus a generic `types.MonthlyArray[uint]` whose `T` argument is bound
to the predeclared `uint` type.

Like it says above, some of these examples are a bit extreme, but it's just trying to highlight that
Tego's `go_type` references can carry a subset of real Go type expressions, including predeclared 
types, pointers, slices, fixed arrays, maps, and type arguments nested inside each other.

Good files to start with:

- `proto/custom/v1/custom.proto`
- `types/types.go`
- `custom/custom.tego.go`

From this example folder, regenerate with:

```sh
buf generate
```
