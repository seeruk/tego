package protogenx

import (
	"strconv"
	"strings"
)

// HasParameterValue checks if the given named parameter has the specified value within the given
// raw parameter string, in the format supplied by protogen.Plugin.Request.GetParameter().
func HasParameterValue(params, name, value string) bool {
	parameterValue, ok := ParameterValue(params, name)
	return ok && parameterValue == value
}

// ParameterValue returns the value of a named parameter within the given raw parameter string, in
// the format supplied by protogen.Plugin.Request.GetParameter().
func ParameterValue(params, name string) (string, bool) {
	for _, param := range strings.Split(params, ",") {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) == 2 {
			if strings.TrimSpace(parts[0]) != name {
				continue
			}

			value := strings.TrimSpace(parts[1])
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}

			return value, true
		}
	}
	return "", false
}
