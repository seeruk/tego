package omittable

type Of[T any] struct {
	Value T
	Valid bool
}

func Some[T any](v T) Of[T] {
	return Of[T]{
		Value: v,
		Valid: true,
	}
}

func None[T any]() Of[T] {
	return Of[T]{}
}
