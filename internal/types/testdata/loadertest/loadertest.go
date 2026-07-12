package loadertest

import "time"

type Example struct {
	Value string
}

type Set[T any] struct {
	Values []T
}

type Box[T any] struct {
	Value T
}

type Pair[K comparable, V any] struct {
	Key   K
	Value V
}

func Convert(value string) Example {
	return Example{Value: value}
}

func MonthFromProto(value int32) time.Month {
	return time.Month(value)
}

func MonthToProto(value time.Month) int32 {
	return int32(value)
}

func (Example) ValueReceiver() string {
	return ""
}

func (*Example) PointerReceiver() string {
	return ""
}
