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
	t.Run("resolves predeclared types", func(t *testing.T) {
		loader := NewLoader()
		predeclared := []string{
			"bool", "string",
			"int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
			"byte", "rune", "float32", "float64", "complex64", "complex128",
			"any", "error",
		}

		for _, name := range predeclared {
			t.Run(name, func(t *testing.T) {
				expr, err := loader.TypeExpr(name, nil)

				require.NoError(t, err)
				assert.Equal(t, TypeExprKindPredeclared, expr.Kind)
				assert.Equal(t, name, expr.Name)
				assert.NotNil(t, expr.Type)
			})
		}
	})

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

	t.Run("resolves predeclared generic arguments", func(t *testing.T) {
		loader := NewLoader()

		direct, err := loader.TypeExpr(loaderTestPkg+".Set[uint]", nil)
		require.NoError(t, err)
		require.Len(t, direct.Args, 1)
		assert.Equal(t, TypeExprKindPredeclared, direct.Args[0].Kind)
		assert.Equal(t, "uint", direct.Args[0].Name)

		substituted, err := loader.TypeExpr(loaderTestPkg+".Set[T]", map[string]string{"T": "uint"})
		require.NoError(t, err)
		require.Len(t, substituted.Args, 1)
		assert.Equal(t, TypeExprKindPredeclared, substituted.Args[0].Kind)
		assert.Equal(t, "uint", substituted.Args[0].Name)
	})

	t.Run("does not collide normalized refs with type parameter names", func(t *testing.T) {
		loader := NewLoader()

		expr, err := loader.TypeExpr(loaderTestPkg+".Box[__tego_type_ref_0]", map[string]string{
			"__tego_type_ref_0": loaderTestPkg + ".Example",
		})

		require.NoError(t, err)
		require.Len(t, expr.Args, 1)
		assert.Equal(t, "Example", expr.Args[0].Name)
	})

	t.Run("resolves arrays maps and parentheses", func(t *testing.T) {
		loader := NewLoader()

		expr, err := loader.TypeExpr("(map[string][]*[0xC]uint64)", nil)

		require.NoError(t, err)
		require.Equal(t, TypeExprKindMap, expr.Kind)
		require.Equal(t, TypeExprKindPredeclared, expr.Key.Kind)
		assert.Equal(t, "string", expr.Key.Name)
		require.Equal(t, TypeExprKindSlice, expr.Value.Kind)
		require.Equal(t, TypeExprKindPointer, expr.Value.Elem.Kind)
		require.Equal(t, TypeExprKindArray, expr.Value.Elem.Elem.Kind)
		assert.Equal(t, int64(12), expr.Value.Elem.Elem.Length)
		assert.Equal(t, "uint64", expr.Value.Elem.Elem.Elem.Name)
	})

	t.Run("resolves arrays through type arguments", func(t *testing.T) {
		loader := NewLoader()

		expr, err := loader.TypeExpr(loaderTestPkg+".Box[T]", map[string]string{"T": "[1_2]uint"})

		require.NoError(t, err)
		require.Len(t, expr.Args, 1)
		assert.Equal(t, TypeExprKindArray, expr.Args[0].Kind)
		assert.Equal(t, int64(12), expr.Args[0].Length)
		assert.Equal(t, "uint", expr.Args[0].Elem.Name)
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
		_, recursiveErr := loader.TypeExpr(loaderTestPkg+".Set[T]", map[string]string{"T": "U", "U": "T"})

		require.Error(t, missingErr)
		assert.Contains(t, missingErr.Error(), "no type argument")
		require.Error(t, unusedErr)
		assert.Contains(t, unusedErr.Error(), "unused")
		require.Error(t, recursiveErr)
		assert.Contains(t, recursiveErr.Error(), "recursive")
	})

	t.Run("rejects malformed expressions and arity mismatches", func(t *testing.T) {
		loader := NewLoader()

		_, malformedErr := loader.TypeExpr("[]", nil)
		_, arityErr := loader.TypeExpr(loaderTestPkg+".Pair[T]", map[string]string{
			"T": loaderTestPkg + ".Example",
		})

		require.Error(t, malformedErr)
		assert.Contains(t, malformedErr.Error(), "invalid type expression")
		require.Error(t, arityErr)
		assert.Contains(t, arityErr.Error(), "requires 2 type arguments")
	})

	t.Run("rejects invalid arrays maps and unsupported forms", func(t *testing.T) {
		loader := NewLoader()
		tests := []struct {
			name       string
			expr       string
			diagnostic string
		}{
			{name: "negative array", expr: "[-1]uint", diagnostic: "non-negative integer literal"},
			{name: "array expression", expr: "[1 << 2]uint", diagnostic: "non-negative integer literal"},
			{name: "inferred array", expr: "[...]uint", diagnostic: "inferred-length arrays"},
			{name: "overflowing array", expr: "[9223372036854775808]uint", diagnostic: "overflows int64"},
			{name: "non-comparable map key", expr: "map[[]string]uint", diagnostic: "not comparable"},
			{name: "constraint", expr: "comparable", diagnostic: "constraint-only"},
			{name: "channel", expr: "chan uint", diagnostic: "channel types"},
			{name: "send channel", expr: "chan<- uint", diagnostic: "channel types"},
			{name: "function", expr: "func(uint) string", diagnostic: "function types"},
			{name: "struct", expr: "struct{ Value uint }", diagnostic: "anonymous struct types"},
			{name: "interface", expr: "interface{ Value() uint }", diagnostic: "anonymous interface types"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := loader.TypeExpr(tt.expr, nil)

				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.diagnostic)
			})
		}
	})

	t.Run("does not expose normalized reference names", func(t *testing.T) {
		loader := NewLoader()

		_, err := loader.TypeExpr(loaderTestPkg+".Set["+loaderTestPkg+".Example", nil)

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "__tego_type_ref_")
		assert.Contains(t, err.Error(), loaderTestPkg+".Set")
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
