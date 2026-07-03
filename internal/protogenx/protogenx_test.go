package protogenx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestParameterValue(t *testing.T) {
	t.Run("returns unquoted named parameter value", func(t *testing.T) {
		value, ok := ParameterValue("paths=import,module=github.com/seeruk/tego", "module")

		require.True(t, ok)
		assert.Equal(t, "github.com/seeruk/tego", value)
	})

	t.Run("returns quoted named parameter value", func(t *testing.T) {
		value, ok := ParameterValue(`module="github.com/seeruk/tego"`, "module")

		require.True(t, ok)
		assert.Equal(t, "github.com/seeruk/tego", value)
	})

	t.Run("reports missing named parameter", func(t *testing.T) {
		_, ok := ParameterValue("paths=import", "module")

		assert.False(t, ok)
	})
}

func TestParameterValues(t *testing.T) {
	t.Run("returns comma-separated continuation values", func(t *testing.T) {
		values, ok := ParameterValues("module=example.com/project,rpc=grpc,connect,module_root=.", "rpc")

		require.True(t, ok)
		assert.Equal(t, []string{"grpc", "connect"}, values)
	})

	t.Run("keeps empty continuation values", func(t *testing.T) {
		values, ok := ParameterValues("rpc=grpc,", "rpc")

		require.True(t, ok)
		assert.Equal(t, []string{"grpc", ""}, values)
	})

	t.Run("returns repeated named parameter values", func(t *testing.T) {
		values, ok := ParameterValues("rpc=grpc,rpc=connect", "rpc")

		require.True(t, ok)
		assert.Equal(t, []string{"grpc", "connect"}, values)
	})

	t.Run("keeps quoted comma values together", func(t *testing.T) {
		values, ok := ParameterValues(`rpc="grpc,connect",module=example.com/project`, "rpc")

		require.True(t, ok)
		assert.Equal(t, []string{"grpc,connect"}, values)
	})

	t.Run("reports missing named parameter", func(t *testing.T) {
		_, ok := ParameterValues("module=example.com/project", "rpc")

		assert.False(t, ok)
	})
}
