package protogenx

import (
	"strconv"
	"strings"
)

type parameter struct {
	name     string
	value    string
	hasValue bool
}

// HasParameterValue checks if the given named parameter has the specified value within the given
// raw parameter string, in the format supplied by protogen.Plugin.Request.GetParameter().
func HasParameterValue(params, name, value string) bool {
	parameterValue, ok := ParameterValue(params, name)
	return ok && parameterValue == value
}

// ParameterValue returns the value of a named parameter within the given raw parameter string, in
// the format supplied by protogen.Plugin.Request.GetParameter().
func ParameterValue(params, name string) (string, bool) {
	for _, param := range parseParameters(params) {
		if param.hasValue && param.name == name {
			return param.value, true
		}
	}
	return "", false
}

// ParameterValues returns all values for a named parameter. Bare comma-separated values after a
// matching parameter are treated as continuations so list parameters can use forms like
// `rpc=grpc,connect`.
func ParameterValues(params, name string) ([]string, bool) {
	var values []string
	collect := false
	for _, param := range parseParameters(params) {
		if param.hasValue {
			collect = false
			if param.name == name {
				values = append(values, param.value)
				collect = true
			}
			continue
		}

		if collect {
			values = append(values, param.name)
		}
	}

	return values, len(values) > 0
}

func parseParameters(params string) []parameter {
	var parameters []parameter
	for _, raw := range splitParameters(params) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			parameters = append(parameters, parameter{})
			continue
		}

		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			parameters = append(parameters, parameter{name: raw})
			continue
		}

		parameters = append(parameters, parameter{
			name:     strings.TrimSpace(parts[0]),
			value:    parameterValue(parts[1]),
			hasValue: true,
		})
	}

	return parameters
}

func splitParameters(params string) []string {
	var fields []string
	var current strings.Builder
	var quote rune
	escaped := false

	for _, r := range params {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if quote != 0 && r == '\\' {
			current.WriteRune(r)
			escaped = true
			continue
		}

		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '"' || r == '\''):
			quote = r
		case quote == 0 && r == ',':
			fields = append(fields, current.String())
			current.Reset()
			continue
		}

		current.WriteRune(r)
	}

	fields = append(fields, current.String())
	return fields
}

func parameterValue(value string) string {
	value = strings.TrimSpace(value)
	if unquoted, err := strconv.Unquote(value); err == nil {
		return unquoted
	}

	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}

	return value
}
