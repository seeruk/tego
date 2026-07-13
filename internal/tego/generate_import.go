package tego

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"unicode"

	gotypes "go/types"

	"google.golang.org/protobuf/compiler/protogen"
)

// generatedFile is the file-scoped rendering context shared by all generation helpers. It keeps
// source emission and import qualification together without leaking import state globally.
type generatedFile struct {
	output  *protogen.GeneratedFile
	imports *generatedImportGraph
}

// newGeneratedFile wraps a transient protogen file with the import graph for its render pass.
func newGeneratedFile(output *protogen.GeneratedFile, imports *generatedImportGraph) *generatedFile {
	return &generatedFile{output: output, imports: imports}
}

// P appends a line to the underlying protogen output.
func (g *generatedFile) P(values ...any) {
	g.output.P(values...)
}

// qualify qualifies a package member through this file's import graph.
func (g *generatedFile) qualify(importPath, name string) string {
	return g.imports.qualify(g.output, importPath, name)
}

// generatedImportRef records one import as it moves from protogen's discovery output to Tego's
// final explicitly aliased import block.
type generatedImportRef struct {
	// ImportPath is the canonical path used by planned type and symbol references.
	ImportPath string
	// OutputPath preserves any ImportRewriteFunc transformation applied by protogen.
	OutputPath string
	// DraftAlias correlates a recorded path with its import spec in discovery output.
	DraftAlias string
}

// generatedImportGraph owns the import graph for one output file across both render passes.
type generatedImportGraph struct {
	currentImportPath string
	packageNames      map[string]string

	// Discovery state populated by the first pass.
	imports map[string]*generatedImportRef
	// importsByAlias correlates protogen's draft import specs with canonical paths.
	importsByAlias map[string]*generatedImportRef

	// Final-render state populated after resolveDiscovery chooses aliases.
	aliases map[string]string
	// finalImports records which discovered paths were also requested by the final pass.
	finalImports map[string]bool
	finalRender  bool
	// err defers qualification failures because qualify must return only a string.
	err error
}

// newGeneratedImportGraph combines planner-discovered package names with names for dependencies
// that Tego references directly. Conflicts are retained as generation errors.
func newGeneratedImportGraph(currentImportPath string, packageNames map[string]string) *generatedImportGraph {
	names := make(map[string]string, len(knownGeneratedPackageNames)+len(packageNames))
	for importPath, name := range knownGeneratedPackageNames {
		names[importPath] = name
	}
	graph := &generatedImportGraph{
		currentImportPath: currentImportPath,
		packageNames:      names,
		imports:           make(map[string]*generatedImportRef),
		importsByAlias:    make(map[string]*generatedImportRef),
	}
	// Sorting makes the selected error deterministic if several hints conflict.
	paths := make([]string, 0, len(packageNames))
	for importPath := range packageNames {
		paths = append(paths, importPath)
	}
	sort.Strings(paths)
	for _, importPath := range paths {
		name := packageNames[importPath]
		if importPath == "" || name == "" || name == "_" || !token.IsIdentifier(name) {
			continue
		}
		if existing, ok := names[importPath]; ok && existing != name {
			if graph.err == nil {
				graph.err = fmt.Errorf("package %q has conflicting names %q and %q", importPath, existing, name)
			}
			continue
		}
		names[importPath] = name
	}
	return graph
}

// qualify records imports during discovery and emits already-resolved aliases during the final
// pass. Imports must be identical in both passes.
func (g *generatedImportGraph) qualify(output *protogen.GeneratedFile, importPath, name string) string {
	if importPath == "" || importPath == g.currentImportPath {
		return name
	}
	if g.finalRender {
		// Do not call protogen here: doing so would let it choose another alias and import block.
		alias, ok := g.aliases[importPath]
		if !ok {
			if g.err == nil {
				g.err = fmt.Errorf("import %q appeared only during the final render", importPath)
			}
			return fallbackGeneratedPackageName(importPath) + "." + name
		}
		g.finalImports[importPath] = true
		return alias + "." + name
	}
	if ref, ok := g.imports[importPath]; ok {
		return ref.DraftAlias + "." + name
	}

	// The discovery pass deliberately delegates once per path so protogen still applies configured
	// import rewrites. resolveDiscovery reads the resulting output path back from the draft source.
	qualified := output.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: protogen.GoImportPath(importPath),
		GoName:       name,
	})
	draftAlias, _, ok := strings.Cut(qualified, ".")
	if !ok {
		g.err = fmt.Errorf("qualifying %s from import %q did not produce a package selector", name, importPath)
		return qualified
	}
	ref := &generatedImportRef{
		ImportPath: importPath,
		DraftAlias: draftAlias,
	}
	g.imports[importPath] = ref
	g.importsByAlias[draftAlias] = ref
	return qualified
}

// resolveDiscovery reads protogen's discovery output, reserves generated package declarations, and
// chooses the complete alias graph before the final pass begins.
func (g *generatedImportGraph) resolveDiscovery(source []byte, outputPackage string) error {
	if g.err != nil {
		return g.err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse generated discovery source for import resolution: %w", err)
	}

	reserved := generatedImportReservedNames(file, outputPackage)
	seen := make(map[string]bool, len(g.imports))
	for _, spec := range file.Imports {
		if spec.Name == nil {
			continue
		}
		ref, ok := g.importsByAlias[spec.Name.Name]
		if !ok {
			continue
		}
		outputPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return fmt.Errorf("decode generated import path %s: %w", spec.Path.Value, err)
		}
		// The emitted path can differ from ImportPath when protogen has an ImportRewriteFunc.
		ref.OutputPath = outputPath
		seen[ref.ImportPath] = true
	}
	for importPath := range g.imports {
		if !seen[importPath] {
			return fmt.Errorf("generated import %q was not emitted during discovery", importPath)
		}
	}
	g.aliases = resolveGeneratedImportAliases(g.imports, g.packageNames, reserved)
	g.finalImports = make(map[string]bool, len(g.imports))
	g.finalRender = true
	return nil
}

// finalize verifies that discovery and final rendering used the same imports, inserts the resolved
// explicit import block, renames colliding local bindings, and formats the completed source.
func (g *generatedImportGraph) finalize(source []byte) ([]byte, error) {
	if g.err != nil {
		return nil, g.err
	}
	for importPath := range g.imports {
		if !g.finalImports[importPath] {
			return nil, fmt.Errorf("import %q appeared only during discovery", importPath)
		}
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse generated final source for import resolution: %w", err)
	}
	packageEnd := fset.Position(file.Name.End()).Offset
	source, err = insertGeneratedImportBlock(source, packageEnd, g.imports, g.aliases)
	if err != nil {
		return nil, err
	}
	fset = token.NewFileSet()
	file, err = parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse generated source with resolved imports: %w", err)
	}

	aliasNames := make(map[string]bool, len(g.imports))
	for _, alias := range g.aliases {
		aliasNames[alias] = true
	}
	renameGeneratedLocalCollisions(file, aliasNames)

	var out bytes.Buffer
	if err := format.Node(&out, fset, file); err != nil {
		return nil, fmt.Errorf("format generated source after import resolution: %w", err)
	}
	return out.Bytes(), nil
}

// insertGeneratedImportBlock adds the sorted, explicitly aliased imports after the package clause.
// It works on source text rather than attaching new AST nodes, because zero-position AST imports
// can cause existing declaration comments to be reassociated during formatting.
func insertGeneratedImportBlock(
	source []byte,
	packageEnd int,
	imports map[string]*generatedImportRef,
	aliases map[string]string,
) ([]byte, error) {
	refs := make([]*generatedImportRef, 0, len(imports))
	outputPaths := make(map[string]string, len(imports))
	importPaths := make([]string, 0, len(imports))
	for importPath := range imports {
		importPaths = append(importPaths, importPath)
	}
	sort.Strings(importPaths)
	for _, importPath := range importPaths {
		ref := imports[importPath]
		if existing, ok := outputPaths[ref.OutputPath]; ok && existing != importPath {
			return nil, fmt.Errorf("imports %q and %q resolve to the same output path %q", existing, importPath, ref.OutputPath)
		}
		outputPaths[ref.OutputPath] = importPath
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].OutputPath == refs[j].OutputPath {
			return refs[i].ImportPath < refs[j].ImportPath
		}
		return refs[i].OutputPath < refs[j].OutputPath
	})
	if len(refs) == 0 {
		return source, nil
	}

	var block strings.Builder
	block.WriteString("\n\nimport (\n")
	for _, ref := range refs {
		block.WriteByte('\t')
		block.WriteString(aliases[ref.ImportPath])
		block.WriteByte(' ')
		block.WriteString(strconv.Quote(ref.OutputPath))
		block.WriteByte('\n')
	}
	block.WriteByte(')')

	output := make([]byte, 0, len(source)+block.Len())
	output = append(output, source[:packageEnd]...)
	output = append(output, block.String()...)
	output = append(output, source[packageEnd:]...)
	return output, nil
}

// generatedImportReservedNames returns names that an import alias may not claim. Package-level
// generated declarations win to keep the public API stable; import aliases only displace locals.
func generatedImportReservedNames(file *ast.File, outputPackage string) map[string]bool {
	reserved := make(map[string]bool)
	for _, name := range gotypes.Universe.Names() {
		reserved[name] = true
	}
	for name := range goKeyword {
		reserved[name] = true
	}
	reserved["init"] = true
	if outputPackage != "" {
		reserved[outputPackage] = true
	}
	if file.Scope != nil {
		for name, object := range file.Scope.Objects {
			// Import objects came from protogen's draft and are replaced, so they are not reservations.
			if object.Kind != ast.Pkg {
				reserved[name] = true
			}
		}
	}
	return reserved
}

// resolveGeneratedImportAliases assigns one deterministic alias per import path. Paths are sorted
// so the lexicographically first path retains an available short package name.
func resolveGeneratedImportAliases(
	imports map[string]*generatedImportRef,
	packageNames map[string]string,
	reserved map[string]bool,
) map[string]string {
	paths := make([]string, 0, len(imports))
	for importPath := range imports {
		paths = append(paths, importPath)
	}
	sort.Strings(paths)

	used := make(map[string]bool, len(reserved)+len(paths))
	for name := range reserved {
		used[name] = true
	}
	aliases := make(map[string]string, len(paths))
	for _, importPath := range paths {
		preferred := packageNames[importPath]
		if preferred == "" || !token.IsIdentifier(preferred) || goKeyword[preferred] {
			preferred = fallbackGeneratedPackageName(importPath)
		}
		alias := firstAvailableGeneratedImportAlias(importPath, preferred, used)
		aliases[importPath] = alias
		used[alias] = true
	}
	return aliases
}

// firstAvailableGeneratedImportAlias qualifies a collision with normalized path components from
// nearest to furthest, then falls back to stable package and numeric suffixes.
func firstAvailableGeneratedImportAlias(importPath, preferred string, used map[string]bool) string {
	if !used[preferred] {
		return preferred
	}

	candidate := preferred
	seenComponents := map[string]bool{preferred: true}
	parts := strings.Split(importPath, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		// The first dotted segment is a host, not useful package context.
		if i == 0 && strings.Contains(parts[i], ".") {
			continue
		}
		component := normalizeGeneratedPackagePart(parts[i])
		if component == "" || isGeneratedVersionPart(component) || seenComponents[component] {
			continue
		}
		seenComponents[component] = true
		candidate = component + "_" + candidate
		if !used[candidate] {
			return candidate
		}
	}

	candidate = preferred + "_pkg"
	if !used[candidate] {
		return candidate
	}
	for suffix := 2; ; suffix++ {
		qualified := candidate + strconv.Itoa(suffix)
		if !used[qualified] {
			return qualified
		}
	}
}

// fallbackGeneratedPackageName derives a safe package name from the last non-version path segment
// when no descriptor, option, loader, or known-dependency name is available.
func fallbackGeneratedPackageName(importPath string) string {
	parts := strings.Split(importPath, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if i == 0 && strings.Contains(parts[i], ".") {
			continue
		}
		part := normalizeGeneratedPackagePart(parts[i])
		if part != "" && !isGeneratedVersionPart(part) {
			return part
		}
	}
	return "pkg"
}

// normalizeGeneratedPackagePart converts an import path component into a lower-case identifier
// fragment and removes conventional leading go- and trailing -go affixes.
func normalizeGeneratedPackagePart(part string) string {
	part = strings.ToLower(strings.TrimSpace(part))
	part = strings.TrimPrefix(part, "go-")
	part = strings.TrimSuffix(part, "-go")

	var normalized strings.Builder
	underscore := false
	for _, r := range part {
		switch {
		case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			if underscore && normalized.Len() > 0 {
				normalized.WriteByte('_')
			}
			underscore = false
			normalized.WriteRune(r)
		default:
			underscore = true
		}
	}
	result := strings.Trim(normalized.String(), "_")
	if result == "" {
		return ""
	}
	for _, first := range result {
		if unicode.IsDigit(first) {
			return "_" + result
		}
		break
	}
	return result
}

// isGeneratedVersionPart reports whether a normalized path component is a Go module vN directory.
func isGeneratedVersionPart(part string) bool {
	if len(part) < 2 || part[0] != 'v' {
		return false
	}
	for _, r := range part[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// renameGeneratedLocalCollisions makes function-local bindings yield to resolved package aliases.
// Package-level declarations were already reserved while aliases were selected.
func renameGeneratedLocalCollisions(file *ast.File, aliases map[string]bool) {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		bindings, closures := collectGeneratedFunctionBindings(function.Recv, function.Type, function.Body)
		renameGeneratedBindingCollisions(function, bindings, aliases, nil)
		renameGeneratedClosureCollisions(closures, aliases, generatedBindingNames(bindings))
	}
}

// renameGeneratedClosureCollisions gives each closure its own allocator while reserving visible
// outer names. Sibling closures can therefore reuse the same readable suffix.
func renameGeneratedClosureCollisions(closures []*ast.FuncLit, aliases map[string]bool, outer []string) {
	for _, closure := range closures {
		bindings, nested := collectGeneratedFunctionBindings(nil, closure.Type, closure.Body)
		renameGeneratedBindingCollisions(closure, bindings, aliases, outer)
		visible := append(append([]string(nil), outer...), generatedBindingNames(bindings)...)
		renameGeneratedClosureCollisions(nested, aliases, visible)
	}
}

// renameGeneratedBindingCollisions allocates replacements for bindings that match package aliases
// and updates every identifier linked to the same parser object.
func renameGeneratedBindingCollisions(root ast.Node, bindings []*ast.Object, aliases map[string]bool, outer []string) {
	reserved := make([]string, 0, len(aliases)+len(outer)+len(bindings))
	for alias := range aliases {
		reserved = append(reserved, alias)
	}
	reserved = append(reserved, outer...)
	for _, object := range bindings {
		reserved = append(reserved, object.Name)
	}
	allocator := newTempNameAllocator(reserved...)
	renamed := make(map[*ast.Object]string)
	for _, object := range bindings {
		if aliases[object.Name] {
			renamed[object] = allocator.name(object.Name)
		}
	}
	if len(renamed) == 0 {
		return
	}
	// Object identity avoids renaming struct fields, selectors, or unrelated shadowed declarations.
	ast.Inspect(root, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok || ident.Obj == nil {
			return true
		}
		if name, ok := renamed[ident.Obj]; ok {
			ident.Name = name
			ident.Obj.Name = name
		}
		return true
	})
}

// generatedBindingNames extracts the post-rename names to reserve in nested closure allocators.
func generatedBindingNames(bindings []*ast.Object) []string {
	names := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		names = append(names, binding.Name)
	}
	return names
}

// collectGeneratedFunctionBindings finds receivers, parameters, results, and body declarations in
// one function scope. Nested closures are returned separately so they can use nested allocators.
func collectGeneratedFunctionBindings(
	receiver *ast.FieldList,
	functionType *ast.FuncType,
	body *ast.BlockStmt,
) ([]*ast.Object, []*ast.FuncLit) {
	var objects []*ast.Object
	var closures []*ast.FuncLit
	seen := make(map[*ast.Object]bool)
	addIdent := func(ident *ast.Ident) {
		if ident != nil && ident.Obj != nil && !seen[ident.Obj] {
			seen[ident.Obj] = true
			objects = append(objects, ident.Obj)
		}
	}
	addFields := func(fields *ast.FieldList) {
		if fields == nil {
			return
		}
		for _, field := range fields.List {
			for _, name := range field.Names {
				addIdent(name)
			}
		}
	}
	addFields(receiver)
	addFields(functionType.TypeParams)
	addFields(functionType.Params)
	addFields(functionType.Results)

	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE {
				for _, expr := range node.Lhs {
					if ident, ok := expr.(*ast.Ident); ok {
						addIdent(ident)
					}
				}
			}
		case *ast.RangeStmt:
			if node.Tok == token.DEFINE {
				if ident, ok := node.Key.(*ast.Ident); ok {
					addIdent(ident)
				}
				if ident, ok := node.Value.(*ast.Ident); ok {
					addIdent(ident)
				}
			}
		case *ast.DeclStmt:
			declaration, ok := node.Decl.(*ast.GenDecl)
			if !ok {
				break
			}
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						addIdent(name)
					}
				case *ast.TypeSpec:
					addIdent(spec.Name)
				}
			}
		case *ast.FuncLit:
			closures = append(closures, node)
			return false
		}
		return true
	})
	return objects, closures
}

// knownGeneratedPackageNames supplies authoritative names without loading packages that Tego
// already knows it may reference directly.
var knownGeneratedPackageNames = map[string]string{
	"connectrpc.com/connect": "connect",
	"context":                "context",
	"errors":                 "errors",
	"fmt":                    "fmt",
	"github.com/seeruk/go-containers/omittable":          "omittable",
	"github.com/seeruk/tego":                             "tego",
	"google.golang.org/grpc":                             "grpc",
	"google.golang.org/protobuf/types/known/anypb":       "anypb",
	"google.golang.org/protobuf/types/known/durationpb":  "durationpb",
	"google.golang.org/protobuf/types/known/emptypb":     "emptypb",
	"google.golang.org/protobuf/types/known/structpb":    "structpb",
	"google.golang.org/protobuf/types/known/timestamppb": "timestamppb",
	"google.golang.org/protobuf/types/known/wrapperspb":  "wrapperspb",
	"io":       "io",
	"iter":     "iter",
	"net/http": "http",
	"time":     "time",
}
