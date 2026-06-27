package tego

import (
	"errors"
	"fmt"

	"github.com/seeruk/tego/internal/protogenx"
	"google.golang.org/protobuf/compiler/protogen"
)

func RunPlugin(plugin *protogen.Plugin) error {
	rawParams := plugin.Request.GetParameter()
	if protogenx.HasParameterValue(rawParams, "paths", "source_relative") {
		// Using source_relative would generate invalid results, as we're going to generated
		// types with the same name as the types `proto-gen-go` generates.
		// TODO: Could be allowed if types were generated with a prefix or suffix in this case?
		return errors.New("tego does not support 'paths=source_relative'")
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
	}

	plan, err := NewPlanner(opts...).Plan(di, si)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	_ = plan

	// TODO: Generation code...
	return nil
}

func modulePath(params string) string {
	modulePath, ok := protogenx.ParameterValue(params, "module")
	if !ok {
		return ""
	}
	return modulePath
}
