package tego

import (
	morphpbv1 "github.com/seeruk/tego/lab/morphpb/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

type Fixture struct {
	ID     string
	People []*[]Person
}

type Person struct {
	FirstName string
	LastName  string
}

func PersonListFromProto(source *morphpbv1.PersonList, mapValue func(*morphpbv1.Person) Person) []Person {
	if source == nil {
		return nil
	}

	people := source.GetPeople()
	out := make([]Person, len(people))
	for i, value := range people {
		out[i] = mapValue(value)
	}
	return out
}

func PersonListToProto(source []Person, mapValue func(Person) *morphpbv1.Person) *morphpbv1.PersonList {
	if source == nil {
		return nil
	}

	people := make([]*morphpbv1.Person, len(source))
	for i, value := range source {
		people[i] = mapValue(value)
	}
	return morphpbv1.PersonList_builder{People: people}.Build()
}

func NullablePersonListFromProto(source *morphpbv1.NullablePersonList, mapValue func(*morphpbv1.PersonList) []Person) *[]Person {
	if source == nil {
		return nil
	}

	switch source.WhichValue() {
	case morphpbv1.NullablePersonList_People_case:
		out := mapValue(source.GetPeople())
		return &out
	default:
		return nil
	}
}

func NullablePersonListToProto(source *[]Person, mapValue func([]Person) *morphpbv1.PersonList) *morphpbv1.NullablePersonList {
	if source == nil {
		null := structpb.NullValue_NULL_VALUE
		return morphpbv1.NullablePersonList_builder{Null: &null}.Build()
	}

	return morphpbv1.NullablePersonList_builder{People: mapValue(*source)}.Build()
}

func NestedPersonListsFromProto(source *morphpbv1.NestedPersonLists, mapValue func(*morphpbv1.NullablePersonList) *[]Person) []*[]Person {
	if source == nil {
		return nil
	}

	people := source.GetPeople()
	out := make([]*[]Person, len(people))
	for i, value := range people {
		out[i] = mapValue(value)
	}
	return out
}

func NestedPersonListsToProto(source []*[]Person, mapValue func(*[]Person) *morphpbv1.NullablePersonList) *morphpbv1.NestedPersonLists {
	if source == nil {
		return nil
	}

	people := make([]*morphpbv1.NullablePersonList, len(source))
	for i, value := range source {
		people[i] = mapValue(value)
	}
	return morphpbv1.NestedPersonLists_builder{People: people}.Build()
}
