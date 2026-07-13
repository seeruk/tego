package plannertest

type Description string

type CustomString string

type Set[T any] struct {
	Values []T
}

type Box[T any] struct {
	Value T
}

type MonthlyArray[T any] [12]T

func StringToInt(value string) int {
	return len(value)
}

func DescriptionFromProto(value string) (*Description, error) {
	return new(Description(value)), nil
}

func DescriptionToProto(value *Description) (string, error) {
	if value == nil {
		return "", nil
	}
	return string(*value), nil
}

func CustomStringFromProto(value string) CustomString {
	return CustomString(value)
}

func CustomStringToProto(value CustomString) string {
	return string(value)
}

func CustomStringPointerFromProto(value string) *CustomString {
	return new(CustomString(value))
}

func CustomStringPointerToProto(value *CustomString) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func CustomStringSetFromProto(values []string) (Set[CustomString], error) {
	set := Set[CustomString]{Values: make([]CustomString, 0, len(values))}
	for _, value := range values {
		set.Values = append(set.Values, CustomString(value))
	}
	return set, nil
}

func CustomStringSetToProto(value Set[CustomString]) ([]string, error) {
	values := make([]string, 0, len(value.Values))
	for _, item := range value.Values {
		values = append(values, string(item))
	}
	return values, nil
}

func CustomStringSetPointerFromProto(value string) *Set[CustomString] {
	return &Set[CustomString]{Values: []CustomString{CustomString(value)}}
}

func CustomStringSetPointerToProto(value *Set[CustomString]) string {
	if value == nil || len(value.Values) == 0 {
		return ""
	}
	return string(value.Values[0])
}

func CustomStringBoxFromProto(value string) Box[*[]*CustomString] {
	values := []*CustomString{new(CustomString(value))}
	return Box[*[]*CustomString]{Value: &values}
}

func CustomStringBoxToProto(value Box[*[]*CustomString]) string {
	if value.Value == nil || len(*value.Value) == 0 || (*value.Value)[0] == nil {
		return ""
	}
	return string(*(*value.Value)[0])
}

func UintArrayFromProto(values []uint64) [12]uint {
	var result [12]uint
	for i, value := range values {
		if i >= len(result) {
			break
		}
		result[i] = uint(value)
	}
	return result
}

func UintArrayToProto(values [12]uint) []uint64 {
	result := make([]uint64, len(values))
	for i, value := range values {
		result[i] = uint64(value)
	}
	return result
}

func MonthlyUintArrayFromProto(values []uint64) MonthlyArray[uint] {
	return MonthlyArray[uint](UintArrayFromProto(values))
}

func MonthlyUintArrayToProto(values MonthlyArray[uint]) []uint64 {
	return UintArrayToProto([12]uint(values))
}

func UintMapFromProto(values map[string]uint64) map[string]uint {
	result := make(map[string]uint, len(values))
	for key, value := range values {
		result[key] = uint(value)
	}
	return result
}

func UintMapToProto(values map[string]uint) map[string]uint64 {
	result := make(map[string]uint64, len(values))
	for key, value := range values {
		result[key] = uint64(value)
	}
	return result
}

func (value CustomString) ToProtoMethod() string {
	return string(value)
}

func (value *CustomString) ToProtoPointerMethod() string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func MissingParameter() CustomString {
	return ""
}

func WrongParameter(value int) CustomString {
	return CustomString(value)
}

func WrongReturn(value string) int {
	return len(value)
}

func WrongError(value string) (CustomString, string) {
	return CustomString(value), value
}
