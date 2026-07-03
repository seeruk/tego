package tego

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/seeruk/tego/internal/protogenx"
	"github.com/seeruk/tego/internal/types"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// RunPlugin is the protoc-gen-tego entrypoint.
func RunPlugin(plugin *protogen.Plugin) error {
	return runPlugin(plugin, os.Stderr)
}

func runPlugin(plugin *protogen.Plugin, diagnostics io.Writer) error {
	plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)
	plugin.SupportedEditionsMinimum = descriptorpb.Edition_EDITION_2023
	plugin.SupportedEditionsMaximum = descriptorpb.Edition_EDITION_2024

	rawParams := plugin.Request.GetParameter()
	if protogenx.HasParameterValue(rawParams, "paths", "source_relative") {
		// Using source_relative would generate invalid results, as we're going to generated
		// types with the same name as the types `proto-gen-go` generates.
		// TODO: Could be allowed if types were generated with a prefix or suffix in this case?
		return errors.New("tego does not support 'paths=source_relative'")
	}
	rpcOptions, err := rpcOptionsFromParams(rawParams)
	if err != nil {
		return fmt.Errorf("parse rpc parameter: %w", err)
	}

	di, err := BuildDescriptorIndex(plugin)
	if err != nil {
		return fmt.Errorf("build descriptor index: %w", err)
	}

	si, err := BuildShapeIndex(di)
	if err != nil {
		return fmt.Errorf("build shape index: %w", err)
	}

	opts := []PlannerOption{
		WithModulePath(modulePath(rawParams)),
		WithRPCPlanning(rpcOptions),
		WithTypeLoader(newTypeLoader(rawParams)),
	}

	plan, err := NewPlanner(opts...).Plan(di, si)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	if err := writeDiagnostics(diagnostics, plan); err != nil {
		return fmt.Errorf("write diagnostics: %w", err)
	}

	if err := Generate(plugin, plan, WithRPCGeneration(rpcOptions)); err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	return nil
}

func modulePath(params string) string {
	modulePath, ok := protogenx.ParameterValue(params, "module")
	if !ok {
		return ""
	}
	return modulePath
}

func moduleRoot(params string) string {
	moduleRoot, ok := protogenx.ParameterValue(params, "module_root")
	if !ok {
		return ""
	}
	return moduleRoot
}

func newTypeLoader(params string) *types.Loader {
	return types.NewLoader(types.WithDir(moduleRoot(params)))
}

func writeDiagnostics(w io.Writer, plan Plan) error {
	diagnostics := planDiagnostics(plan)
	if len(diagnostics) == 0 {
		return nil
	}

	_, err := fmt.Fprintf(w, "diagnostics:\n%s\n", formatDiagnostics(diagnostics))
	return err
}

func planDiagnostics(plan Plan) []Diagnostic {
	var diagnostics []Diagnostic
	for _, file := range plan.Files {
		diagnostics = append(diagnostics, file.Diagnostics...)
	}
	return diagnostics
}
