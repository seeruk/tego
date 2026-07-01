package tego

// Value is the JSON-like dynamic value domain represented by google.protobuf.Value.
type Value = any

// Struct is the JSON-like object domain represented by google.protobuf.Struct.
type Struct = map[string]Value

// ListValue is the JSON-like list domain represented by google.protobuf.ListValue.
type ListValue = []Value
