package types

import (
	"fmt"
	"sort"
	"strings"

	"github.com/seeruk/tego/examples/custom-types/custompbv1"
)

type UUID [16]byte

func UUIDFromProto(uuid *custompbv1.UUID) (UUID, error) {
	if len(uuid.GetValue()) != 16 {
		return UUID{}, fmt.Errorf("UUID length must be 16")
	}
	var out UUID
	copy(out[:], uuid.GetValue())
	return out, nil
}

func UUIDToProto(uuid UUID) *custompbv1.UUID {
	return custompbv1.UUID_builder{
		Value: new(string(uuid[:])),
	}.Build()
}

type Email string

func EmailFromProto(value string) (Email, error) {
	if !strings.Contains(value, "@") {
		return "", fmt.Errorf("invalid email %q", value)
	}
	return Email(value), nil
}

func EmailToProto(value Email) (string, error) {
	if !strings.Contains(string(value), "@") {
		return "", fmt.Errorf("invalid email %q", value)
	}
	return string(value), nil
}

type Money struct {
	Cents int64
}

func MoneyFromProto(cents int64) Money {
	return Money{Cents: cents}
}

func MoneyToProto(value Money) int64 {
	return value.Cents
}

type DisplayName string

func DisplayNameFromProto(value string) *DisplayName {
	displayName := DisplayName(value)
	return &displayName
}

func DisplayNameToProto(value *DisplayName) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

type Label string

type Set[T comparable] map[T]struct{}

func LabelSetFromProto(values []string) Set[Label] {
	if values == nil {
		return nil
	}
	set := make(Set[Label], len(values))
	for _, value := range values {
		set[Label(value)] = struct{}{}
	}
	return set
}

func LabelSetToProto(value Set[Label]) []string {
	if value == nil {
		return nil
	}
	values := make([]string, 0, len(value))
	for label := range value {
		values = append(values, string(label))
	}
	sort.Strings(values)
	return values
}

type Box[T any] struct {
	Value T
}

func ContactAliasesFromProto(values []string) (Box[*[]*Email], error) {
	if values == nil {
		return Box[*[]*Email]{}, nil
	}
	aliases := make([]*Email, 0, len(values))
	for _, value := range values {
		email, err := EmailFromProto(value)
		if err != nil {
			return Box[*[]*Email]{}, err
		}
		aliases = append(aliases, new(email))
	}
	return Box[*[]*Email]{Value: &aliases}, nil
}

func ContactAliasesToProto(value Box[*[]*Email]) ([]string, error) {
	if value.Value == nil {
		return nil, nil
	}
	values := make([]string, 0, len(*value.Value))
	for _, alias := range *value.Value {
		if alias == nil {
			return nil, fmt.Errorf("contact alias must not be nil")
		}
		email, err := EmailToProto(*alias)
		if err != nil {
			return nil, err
		}
		values = append(values, email)
	}
	return values, nil
}
