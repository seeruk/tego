package protogenx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasParameterValue(t *testing.T) {
	tt := []struct {
		input string
		param string
		value string
	}{
		{input: "paths=source_relative", param: "paths", value: "source_relative"},
		{input: "paths = source_relative", param: "paths", value: "source_relative"},
		{input: "paths=\"source_relative\"", param: "paths", value: "source_relative"},
		{input: "paths = \"source_relative\"", param: "paths", value: "source_relative"},
		{input: "paths=source_relative,other_param=other_value", param: "paths", value: "source_relative"},
		{input: "paths=source_relative, other_param='other_value'", param: "paths", value: "source_relative"},
	}

	for _, tc := range tt {
		t.Run(tc.input, func(t *testing.T) {
			assert.True(t, HasParameterValue(tc.input, tc.param, tc.value))
		})
	}
}
