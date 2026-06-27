package plannertest

type Description string

type CustomString string

func DescriptionFromProto(value string) (*Description, error) {
	description := Description(value)
	return &description, nil
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
	custom := CustomString(value)
	return &custom
}

func CustomStringPointerToProto(value *CustomString) string {
	if value == nil {
		return ""
	}
	return string(*value)
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
