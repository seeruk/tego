package omittable

// Of represents a value that may be intentionally omitted.
type Of[T any] struct {
	// Value is meaningful only when Valid is true.
	Value T
	// Valid reports whether Value was supplied.
	Valid bool
}

// Some returns an omittable value that is present.
func Some[T any](v T) Of[T] {
	return Of[T]{
		Value: v,
		Valid: true,
	}
}

// None returns an omittable value that is absent.
func None[T any]() Of[T] {
	return Of[T]{}
}
