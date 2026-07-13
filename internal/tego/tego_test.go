package tego

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilePlanIsEmpty(t *testing.T) {
	tests := map[string]struct {
		plan  FilePlan
		empty bool
	}{
		"zero value": {
			empty: true,
		},
		"metadata and diagnostics": {
			plan: FilePlan{
				ProtoPath: "empty.proto",
				Output:    FileOutputPlan{GeneratorPath: "example/empty.tego.go"},
				Package:   PackageRef{ImportPath: "example", Name: "example"},
				Diagnostics: []Diagnostic{{
					Level:   DiagnosticLevelWarning,
					Path:    "empty.proto",
					Message: "warning",
				}},
			},
			empty: true,
		},
		"enums": {
			plan: FilePlan{Enums: []EnumPlan{{}}},
		},
		"oneofs": {
			plan: FilePlan{Oneofs: []OneofPlan{{}}},
		},
		"structs": {
			plan: FilePlan{Structs: []StructPlan{{}}},
		},
		"mappings": {
			plan: FilePlan{Mappings: []MappingPlan{{}}},
		},
		"services": {
			plan: FilePlan{Services: []ServicePlan{{}}},
		},
		"request inline helpers": {
			plan: FilePlan{RequestInlineHelpers: []ServiceInlineHelperPlan{{}}},
		},
		"response inline helpers": {
			plan: FilePlan{ResponseInlineHelpers: []ServiceInlineHelperPlan{{}}},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.empty, tt.plan.IsEmpty())
		})
	}
}
