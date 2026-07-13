package tego

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageNameRegistry(t *testing.T) {
	registry := newPackageNameRegistry()
	require.NoError(t, registry.add("example.com/types", "types"))
	require.NoError(t, registry.add("example.com/types", "types"))

	err := registry.add("example.com/types", "model")
	require.Error(t, err)

	assert.Contains(t, err.Error(), `package "example.com/types" has conflicting names "types" and "model"`)
}
