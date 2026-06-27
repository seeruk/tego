# Shapes

Shapes are protobuf message conventions that Tego can flatten into more idiomatic Go types. They let
schema authors express types that protobuf cannot model directly, while still keeping the protobuf
wire format explicit and portable.

Shapes compose recursively. A protobuf wrapper can become a Go slice, then another wrapper can make
that slice nullable, then another wrapper can repeat it again.

Tego infers shapes by default. If a message should stay as an ordinary struct even though it looks
like a shape, shape inference can be disabled with `(tego.message).infer_shape = false`.

## Nullable

Nullable shapes mark another value as explicitly nullable. There are two forms which can be used.

The oneof form uses exactly one oneof with two choices, where one choice is
`google.protobuf.NullValue`:

```protobuf
message NullablePerson {
  oneof value {
    Person person = 1;
    google.protobuf.NullValue null = 2;
  }
}
```

The value/valid form uses exactly two fields named `value` and `valid`:

```protobuf
message NullablePerson {
  Person value = 1;
  bool valid = 2;
}
```

Both forms produce pointer-like Go output:

```go
*Person
*[]Person
*map[Person][]Ticket
```

This is useful when presence alone is not enough, or when the nullable thing is itself a shaped type
such as a slice or map.

## Slice

Slice shapes wrap exactly one repeated field.

```protobuf
message People {
  repeated Person people = 1;
}
```

That wrapper can become a plain Go slice:

```go
[]Person
```

You may initially look at this and wonder what advantage this has over just using a regular repeated
field. With slice shapes, Tego can model Go types such as:

```go
[][]string
[]*[]Person
```

Tego will flatten other shapes too, and the resulting Go code is not possible with plain
Protobuf alone, including the slice-of-slice example above.

## Map

Map shapes wrap a repeated nested `Map` message with exactly two fields named `key` and `value`.

```protobuf
message ColleaguesByManager {
  message Map {
    Person key = 1;
    People value = 2;
  }

  repeated Map entries = 1;
}
```

That wrapper can become a Go map:

```go
map[Person][]*[]Person
```

This is primarily intended to be used to allow things which are possible in Go, but not in Protobuf,
such as removing restrictions on the types available for use as keys. Native protobuf maps only
allow a restricted set of key types, and their values cannot represent every shaped Go type
directly. Tego map shapes can use richer keys and values, as long as the generated Go key type is
comparable.

For custom Go replacement types (specified using the `go_type` options), Tego cannot prove
comparability from protobuf alone. Value custom types need an explicit `comparable: true` override
before they can be used as map keys. Custom types planned with `as_pointer: true` are also accepted,
because Go pointers are comparable. Go compilation remains the final check.

## Composition

Shapes can be composed together using multiple nested messages.

```protobuf
edition = "2024";

// ...

message Person {
  string first_name = 1;
  string last_name = 2;
}

message People {
  repeated Person people = 1;
}

message NullablePeople {
  oneof value {
    People people = 1;
    google.protobuf.NullValue null = 2;
  }
}

message ListOfNullablePeople {
  repeated NullablePeople people = 1;
}
```

Tego sees that as three shape layers:

| Protobuf message       | Go shape      |
|------------------------|---------------|
| `People`               | `[]Person`    |
| `NullablePeople`       | `*[]Person`   |
| `ListOfNullablePeople` | `[]*[]Person` |

Each shape unwraps one layer. The final Go type is the result of flattening those layers from the
inside out.

## Mapping With Morph

Tego owns shape inference and code generation. Morph does not need to understand Tego's shape rules
directly.

Instead, Tego will generate bespoke helper functions for each shape that is actually used. Morph can
treat those helpers as ordinary callables and compose them with normal struct mapping. Morph will
then take care of the proto-to-Go mapping, and vice versa.
