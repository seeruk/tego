package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const loaderTestPkg = "github.com/seeruk/tego/internal/types/testdata/loadertest"

func TestLoaderType(t *testing.T) {
	t.Run("resolves named type", func(t *testing.T) {
		loader := NewLoader()

		typ, err := loader.Type(loaderTestPkg + ".Example")

		require.NoError(t, err)
		assert.Equal(t, loaderTestPkg, typ.ImportPath)
		assert.Equal(t, "Example", typ.Name)
		assert.NotNil(t, typ.TypeName)
		assert.NotNil(t, typ.Named)
	})

	t.Run("returns cached type", func(t *testing.T) {
		loader := NewLoader()
		first, err := loader.Type(loaderTestPkg + ".Example")
		require.NoError(t, err)

		second, err := loader.Type(loaderTestPkg + ".Example")

		require.NoError(t, err)
		assert.Same(t, first, second)
	})
}

func TestLoaderFunction(t *testing.T) {
	t.Run("resolves package function", func(t *testing.T) {
		loader := NewLoader()

		function, err := loader.Function(loaderTestPkg + ".Convert")

		require.NoError(t, err)
		assert.Equal(t, loaderTestPkg, function.ImportPath)
		assert.Equal(t, "Convert", function.Name)
		assert.NotNil(t, function.Signature)
	})

	t.Run("returns cached function", func(t *testing.T) {
		loader := NewLoader()
		first, err := loader.Function(loaderTestPkg + ".Convert")
		require.NoError(t, err)

		second, err := loader.Function(loaderTestPkg + ".Convert")

		require.NoError(t, err)
		assert.Same(t, first, second)
	})

	t.Run("rejects method reference", func(t *testing.T) {
		loader := NewLoader()

		_, err := loader.Function(loaderTestPkg + ".Example.ValueReceiver")

		require.Error(t, err)
	})
}

func TestLoaderMethod(t *testing.T) {
	t.Run("resolves value receiver method", func(t *testing.T) {
		loader := NewLoader()

		method, err := loader.Method(loaderTestPkg + ".Example.ValueReceiver")

		require.NoError(t, err)
		assert.Equal(t, "Example", method.Receiver)
		assert.Equal(t, "ValueReceiver", method.Name)
		assert.NotNil(t, method.Signature)
	})

	t.Run("resolves pointer receiver method", func(t *testing.T) {
		loader := NewLoader()

		method, err := loader.Method(loaderTestPkg + ".Example.PointerReceiver")

		require.NoError(t, err)
		assert.Equal(t, "Example", method.Receiver)
		assert.Equal(t, "PointerReceiver", method.Name)
		assert.NotNil(t, method.Signature)
	})

	t.Run("returns cached method", func(t *testing.T) {
		loader := NewLoader()
		first, err := loader.Method(loaderTestPkg + ".Example.ValueReceiver")
		require.NoError(t, err)

		second, err := loader.Method(loaderTestPkg + ".Example.ValueReceiver")

		require.NoError(t, err)
		assert.Same(t, first, second)
	})
}

func TestLoaderErrors(t *testing.T) {
	t.Run("rejects malformed ref", func(t *testing.T) {
		loader := NewLoader()

		_, err := loader.Type("Example")

		require.Error(t, err)
	})

	t.Run("reports missing package", func(t *testing.T) {
		loader := NewLoader()

		_, err := loader.Type("github.com/seeruk/tego/internal/types/testdata/missing.Example")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "load package")
	})

	t.Run("reports missing symbol", func(t *testing.T) {
		loader := NewLoader()

		_, err := loader.Type(loaderTestPkg + ".Missing")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("reports wrong symbol kind", func(t *testing.T) {
		loader := NewLoader()

		_, err := loader.Type(loaderTestPkg + ".Convert")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a type")
	})

	t.Run("rejects function reference as method", func(t *testing.T) {
		loader := NewLoader()

		_, err := loader.Method(loaderTestPkg + ".Convert")

		require.Error(t, err)
	})
}
