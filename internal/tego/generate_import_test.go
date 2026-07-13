package tego

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
)

func TestNormalizeGeneratedPackagePart(t *testing.T) {
	tests := map[string]string{
		"go-containers":     "containers",
		"containers-go":     "containers",
		"go-containers-go":  "containers",
		"some.package-name": "some_package_name",
		"123-client":        "_123_client",
		"---":               "",
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, expected, normalizeGeneratedPackagePart(input))
		})
	}
}

func TestFallbackGeneratedPackageName(t *testing.T) {
	tests := map[string]string{
		"example.com/acme/widget/v2": "widget",
		"example.com/acme/widget":    "widget",
		"example.com/v2":             "pkg",
		"context":                    "context",
	}

	for importPath, expected := range tests {
		t.Run(importPath, func(t *testing.T) {
			assert.Equal(t, expected, fallbackGeneratedPackageName(importPath))
		})
	}
}

func TestResolveGeneratedImportAliases(t *testing.T) {
	t.Run("prefers real names and deterministically qualifies collisions", func(t *testing.T) {
		imports := generatedImportRefs(
			"github.com/seeruk/go-containers/omittable",
			"example.com/acme/optional/v2",
			"example.com/other/optional/v3",
		)
		packageNames := map[string]string{
			"github.com/seeruk/go-containers/omittable": "omittable",
			"example.com/acme/optional/v2":              "omittable",
			"example.com/other/optional/v3":             "omittable",
		}

		aliases := resolveGeneratedImportAliases(imports, packageNames, nil)

		assert.Equal(t, "omittable", aliases["example.com/acme/optional/v2"])
		assert.Equal(t, "optional_omittable", aliases["example.com/other/optional/v3"])
		assert.Equal(t, "containers_omittable", aliases["github.com/seeruk/go-containers/omittable"])
	})

	t.Run("reserves declarations keywords predeclared names and output packages", func(t *testing.T) {
		imports := generatedImportRefs(
			"github.com/acme/errors",
			"github.com/acme/any",
			"github.com/acme/generated",
			"github.com/acme/type",
		)
		packageNames := map[string]string{
			"github.com/acme/errors":    "errors",
			"github.com/acme/any":       "any",
			"github.com/acme/generated": "generated",
			"github.com/acme/type":      "type",
		}
		reserved := generatedImportReservedNames(mustParseGeneratedFile(t, `
package generated
type errors struct{}
`), "generated")

		aliases := resolveGeneratedImportAliases(imports, packageNames, reserved)

		assert.Equal(t, "acme_errors", aliases["github.com/acme/errors"])
		assert.Equal(t, "acme_any", aliases["github.com/acme/any"])
		assert.Equal(t, "acme_generated", aliases["github.com/acme/generated"])
		assert.Equal(t, "acme_type", aliases["github.com/acme/type"])
	})

	t.Run("uses deterministic package suffixes when path qualification is exhausted", func(t *testing.T) {
		aliases := resolveGeneratedImportAliases(
			generatedImportRefs("errors"),
			map[string]string{"errors": "errors"},
			map[string]bool{"errors": true, "errors_pkg": true},
		)

		assert.Equal(t, "errors_pkg2", aliases["errors"])
	})

	t.Run("uses upstream non-host components from nearest to furthest", func(t *testing.T) {
		const importPath = "github.com/seeruk/go-containers/omittable"
		aliases := resolveGeneratedImportAliases(
			generatedImportRefs(importPath),
			map[string]string{importPath: "omittable"},
			map[string]bool{"omittable": true, "containers_omittable": true},
		)

		assert.Equal(t, "seeruk_containers_omittable", aliases[importPath])
	})

	t.Run("is independent of map insertion order", func(t *testing.T) {
		packageNames := map[string]string{
			"example.com/zeta/types":  "types",
			"example.com/alpha/types": "types",
		}
		forward := generatedImportRefs("example.com/zeta/types", "example.com/alpha/types")
		reverse := generatedImportRefs("example.com/alpha/types", "example.com/zeta/types")

		assert.Equal(
			t,
			resolveGeneratedImportAliases(forward, packageNames, nil),
			resolveGeneratedImportAliases(reverse, packageNames, nil),
		)
	})
}

func TestGeneratedImportGraphFinalize(t *testing.T) {
	t.Run("package aliases win over generated local bindings", func(t *testing.T) {
		imports := newGeneratedImportGraph(generatedTestPkg, map[string]string{
			"example.com/source/v2": "source",
			"example.com/errors":    "errors",
		})
		content := renderGeneratedImportTest(t, imports, "locals.tego.go", func(g *generatedFile) {
			g.P("package generated")
			g.P()
			g.P("func Convert(source *", g.qualify("example.com/source/v2", "Input"), ") {")
			g.P("errors := ", g.qualify("example.com/errors", "New"), "()")
			g.P("_ = source")
			g.P("_ = errors")
			g.P("_ = func(source int) { _ = source }")
			g.P("}")
			g.P("func Siblings() {")
			g.P("_ = func(source int) { _ = source }")
			g.P("_ = func(source int) { _ = source }")
			g.P("}")
			g.P("func Walk() (errors error) {")
			g.P("for _, errors := range []error{nil} { _ = errors }")
			g.P("return errors")
			g.P("}")
		})

		assert.Contains(t, content, `errors "example.com/errors"`)
		assert.Contains(t, content, `source "example.com/source/v2"`)
		assert.Contains(t, content, "func Convert(source2 *source.Input)")
		assert.Contains(t, content, "errors2 := errors.New()")
		assert.Contains(t, content, "_ = source2")
		assert.Contains(t, content, "_ = errors2")
		assert.Contains(t, content, "_ = func(source3 int) { _ = source3 }")
		assert.Equal(t, 2, strings.Count(content, "_ = func(source2 int) { _ = source2 }"))
		assert.Contains(t, content, "func Walk() (errors2 error)")
		assert.Contains(t, content, "for _, errors3 := range []error{nil}")
		assert.Contains(t, content, "return errors2")
	})

	t.Run("package-level declarations win without changing their API", func(t *testing.T) {
		imports := newGeneratedImportGraph(generatedTestPkg, map[string]string{
			"github.com/acme/errors": "errors",
		})
		content := renderGeneratedImportTest(t, imports, "declarations.tego.go", func(g *generatedFile) {
			g.P("package generated")
			g.P()
			g.P("type errors struct{}")
			g.P("var _ = ", g.qualify("github.com/acme/errors", "New"))
		})

		assert.Contains(t, content, `acme_errors "github.com/acme/errors"`)
		assert.Contains(t, content, "type errors struct{}")
		assert.Contains(t, content, "var _ = acme_errors.New")
	})

	t.Run("rejects conflicting authoritative package names", func(t *testing.T) {
		imports := newGeneratedImportGraph(generatedTestPkg, map[string]string{"context": "ctx"})

		err := imports.resolveDiscovery([]byte("package generated\n"), "generated")

		require.Error(t, err)
		assert.Contains(t, err.Error(), `package "context" has conflicting names "context" and "ctx"`)
	})

	t.Run("rejects imports missing from discovery output", func(t *testing.T) {
		imports := newGeneratedImportGraph(generatedTestPkg, nil)
		imports.imports["example.com/widget/v2"] = &generatedImportRef{
			ImportPath: "example.com/widget/v2",
			DraftAlias: "v2",
		}
		imports.importsByAlias["v2"] = imports.imports["example.com/widget/v2"]

		err := imports.resolveDiscovery([]byte("package generated\n"), "generated")

		require.Error(t, err)
		assert.Contains(t, err.Error(), `generated import "example.com/widget/v2" was not emitted during discovery`)
	})

	t.Run("rejects imports introduced by the final render", func(t *testing.T) {
		imports := newGeneratedImportGraph(generatedTestPkg, nil)
		require.NoError(t, imports.resolveDiscovery([]byte("package generated\n"), "generated"))
		plugin := newGeneratorTestPlugin(t)
		finalOutput := plugin.NewGeneratedFile("final-only.tego.go", protogen.GoImportPath(generatedTestPkg))
		finalRender := newGeneratedFile(finalOutput, imports)
		finalRender.P("package generated")
		finalRender.P("var _ = ", finalRender.qualify("example.com/widget", "Value"))
		finalContent, err := finalOutput.Content()
		require.NoError(t, err)

		_, err = imports.finalize(finalContent)

		require.Error(t, err)
		assert.Contains(t, err.Error(), `import "example.com/widget" appeared only during the final render`)
	})

	t.Run("preserves rewritten import paths from discovery", func(t *testing.T) {
		imports := newGeneratedImportGraph(generatedTestPkg, map[string]string{
			"example.com/widget": "widget",
		})
		ref := &generatedImportRef{ImportPath: "example.com/widget", DraftAlias: "widget"}
		imports.imports[ref.ImportPath] = ref
		imports.importsByAlias[ref.DraftAlias] = ref
		require.NoError(t, imports.resolveDiscovery([]byte(`package generated
import widget "mirror.example/widget"
var _ = widget.Value
`), "generated"))
		imports.finalImports[ref.ImportPath] = true

		content, err := imports.finalize([]byte("package generated\nvar _ = widget.Value\n"))

		require.NoError(t, err)
		assert.Contains(t, string(content), `widget "mirror.example/widget"`)
	})

	t.Run("uses a safe fallback without loading unknown packages", func(t *testing.T) {
		imports := newGeneratedImportGraph(generatedTestPkg, nil)

		content := renderGeneratedImportTest(t, imports, "fallback.tego.go", func(g *generatedFile) {
			g.P("package generated")
			g.P("var _ = ", g.qualify("unknown.example/client/v2", "Value"))
		})

		assert.Contains(t, content, `client "unknown.example/client/v2"`)
		assert.Contains(t, content, "var _ = client.Value")
	})
}

func TestNewGeneratedImportGraphSeedsKnownPackageNames(t *testing.T) {
	imports := newGeneratedImportGraph(generatedTestPkg, nil)

	assert.Equal(t, "context", imports.packageNames["context"])
	assert.Equal(t, "http", imports.packageNames["net/http"])
	assert.Equal(t, "connect", imports.packageNames["connectrpc.com/connect"])
	assert.Equal(t, "grpc", imports.packageNames["google.golang.org/grpc"])
	assert.Equal(t, "tego", imports.packageNames["github.com/seeruk/tego"])
	assert.Equal(t, "omittable", imports.packageNames["github.com/seeruk/go-containers/omittable"])
}

func generatedImportRefs(importPaths ...string) map[string]*generatedImportRef {
	refs := make(map[string]*generatedImportRef, len(importPaths))
	for _, importPath := range importPaths {
		refs[importPath] = &generatedImportRef{ImportPath: importPath}
	}
	return refs
}

func renderGeneratedImportTest(
	t *testing.T,
	imports *generatedImportGraph,
	filename string,
	render func(*generatedFile),
) string {
	t.Helper()

	plugin := newGeneratorTestPlugin(t)

	discoveryOutput := plugin.NewGeneratedFile(filename, generatedTestPkg)
	render(newGeneratedFile(discoveryOutput, imports))

	discoveryContent, err := discoveryOutput.Content()
	require.NoError(t, err)
	require.NoError(t, imports.resolveDiscovery(discoveryContent, "generated"))

	finalOutput := plugin.NewGeneratedFile(filename, generatedTestPkg)
	render(newGeneratedFile(finalOutput, imports))

	finalContent, err := finalOutput.Content()
	require.NoError(t, err)

	content, err := imports.finalize(finalContent)
	require.NoError(t, err)

	return string(content)
}

func mustParseGeneratedFile(t *testing.T, source string) *ast.File {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, parser.ParseComments)
	require.NoError(t, err)

	return file
}
