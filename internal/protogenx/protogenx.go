package protogenx

import (
	"fmt"
	"strconv"
	"strings"
)

// HasParameterValue checks if the given named parameter has the specified value within the given
// raw parameter string, in the format supplied by protogen.Plugin.Request.GetParameter().
func HasParameterValue(params, name, value string) bool {
	for _, param := range strings.Split(params, ",") {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) == 2 {
			isPathsPart := strings.TrimSpace(parts[0]) == name
			if !isPathsPart {
				continue
			}

			pathsValue := strings.TrimSpace(parts[1])
			if unquoted, err := strconv.Unquote(pathsValue); err == nil {
				pathsValue = unquoted
			} else {
				fmt.Println(err)
			}

			if pathsValue == value {
				return true
			}
		}
	}
	return false
}
