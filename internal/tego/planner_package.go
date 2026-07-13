package tego

import (
	"fmt"
	"go/token"
	"path"
	"sort"
)

func (p *Planner) planPackageNames(di *DescriptorIndex) (map[string]string, error) {
	registry := newPackageNameRegistry()

	for _, file := range di.Files {
		if file.Desc != nil {
			if err := registry.add(string(file.Desc.GoImportPath), string(file.Desc.GoPackageName)); err != nil {
				return nil, err
			}
			if p.rpc.Connect && p.rpc.ConnectPackageSuffix != "" {
				connectPath := path.Join(
					string(file.Desc.GoImportPath),
					string(file.Desc.GoPackageName)+p.rpc.ConnectPackageSuffix,
				)
				if err := registry.add(connectPath, string(file.Desc.GoPackageName)+p.rpc.ConnectPackageSuffix); err != nil {
					return nil, err
				}
			}
		}
		if file.Options != nil && file.Options.HasGoPackage() {
			pkg := packageRef(file.Options.GetGoPackage())
			if err := registry.add(pkg.ImportPath, pkg.Name); err != nil {
				return nil, err
			}
		}
	}

	loadedPackageNames := p.typeLoader.PackageNames()
	loadedImportPaths := make([]string, 0, len(loadedPackageNames))
	for importPath := range loadedPackageNames {
		loadedImportPaths = append(loadedImportPaths, importPath)
	}
	sort.Strings(loadedImportPaths)
	for _, importPath := range loadedImportPaths {
		name := loadedPackageNames[importPath]
		if err := registry.add(importPath, name); err != nil {
			return nil, err
		}
	}

	return registry.names, nil
}

type packageNameRegistry struct {
	names map[string]string
}

func newPackageNameRegistry() *packageNameRegistry {
	return &packageNameRegistry{names: make(map[string]string)}
}

func (r *packageNameRegistry) add(importPath, name string) error {
	if importPath == "" || name == "" || name == "_" || !token.IsIdentifier(name) {
		return nil
	}
	if existing, ok := r.names[importPath]; ok && existing != name {
		return fmt.Errorf("package %q has conflicting names %q and %q", importPath, existing, name)
	}
	r.names[importPath] = name
	return nil
}
