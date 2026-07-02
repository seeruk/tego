package yiratypes

import (
	"sort"
	"time"
)

type Date time.Time

type Label string

type Set[T comparable] map[T]struct{}

func DateFromProto(value string) (Date, error) {
	if value == "" {
		return Date{}, nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return Date{}, err
	}
	return Date(parsed), nil
}

func DateToProto(value Date) (string, error) {
	date := time.Time(value)
	if date.IsZero() {
		return "", nil
	}
	return date.Format(time.DateOnly), nil
}

func LabelSetFromProto(values []string) (Set[Label], error) {
	if values == nil {
		return nil, nil
	}
	set := make(Set[Label], len(values))
	for _, value := range values {
		set[Label(value)] = struct{}{}
	}
	return set, nil
}

func LabelSetToProto(value Set[Label]) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	values := make([]string, 0, len(value))
	for label := range value {
		values = append(values, string(label))
	}
	sort.Strings(values)
	return values, nil
}
