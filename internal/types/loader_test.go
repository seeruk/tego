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

func TestLoaderTypeExpr(t *testing.T) {
	t.Run("resolves generic type expression", func(t *testing.T) {
		loader := NewLoader()

		expr, err := loader.TypeExpr(loaderTestPkg+".Set[T]", map[string]string{
			"T": loaderTestPkg + ".Example",
		})

		require.NoError(t, err)
		assert.Equal(t, "Set", expr.Name)
		require.Len(t, expr.Args, 1)
		assert.Equal(t, "Example", expr.Args[0].Name)
		assert.NotNil(t, expr.Type)
	})

	t.Run("resolves pointer and slice type arguments", func(t *testing.T) {
		loader := NewLoader()

		expr, err := loader.TypeExpr(loaderTestPkg+".Box[*[]*T]", map[string]string{
			"T": loaderTestPkg + ".Example",
		})

		require.NoError(t, err)
		require.Len(t, expr.Args, 1)
		arg := expr.Args[0]
		require.Equal(t, TypeExprKindPointer, arg.Kind)
		require.Equal(t, TypeExprKindSlice, arg.Elem.Kind)
		require.Equal(t, TypeExprKindPointer, arg.Elem.Elem.Kind)
		assert.Equal(t, "Example", arg.Elem.Elem.Elem.Name)
	})

	t.Run("rejects missing and unused type arguments", func(t *testing.T) {
		loader := NewLoader()

		_, missingErr := loader.TypeExpr(loaderTestPkg+".Set[T]", nil)
		_, unusedErr := loader.TypeExpr(loaderTestPkg+".Example", map[string]string{"T": loaderTestPkg + ".Example"})

		require.Error(t, missingErr)
		assert.Contains(t, missingErr.Error(), "no type argument")
		require.Error(t, unusedErr)
		assert.Contains(t, unusedErr.Error(), "unused")
	})

	t.Run("rejects malformed expressions and arity mismatches", func(t *testing.T) {
		loader := NewLoader()

		_, malformedErr := loader.TypeExpr("map[string]string", nil)
		_, arityErr := loader.TypeExpr(loaderTestPkg+".Pair[T]", map[string]string{
			"T": loaderTestPkg + ".Example",
		})

		require.Error(t, malformedErr)
		assert.Contains(t, malformedErr.Error(), "cannot have type arguments")
		require.Error(t, arityErr)
		assert.Contains(t, arityErr.Error(), "requires 2 type arguments")
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

func TestLoaderOptions(t *testing.T) {
	t.Run("sets package loading dir", func(t *testing.T) {
		loader := NewLoader(WithDir("."))

		require.NotNil(t, loader.config)
		assert.Equal(t, ".", loader.config.Dir)
	})

	t.Run("ignores empty dir", func(t *testing.T) {
		loader := NewLoader(WithDir(""))

		assert.Nil(t, loader.config)
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
