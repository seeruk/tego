package tego

import (
	"testing"

	"github.com/seeruk/tego/tegopb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestPlannerPlanYiraFixture(t *testing.T) {
	descriptorIndex := buildYiraDescriptorIndex(t)
	shapeIndex, err := BuildShapeIndex(descriptorIndex)
	require.NoError(t, err)

	plan, err := NewPlanner().Plan(descriptorIndex, shapeIndex)
	require.NoError(t, err)
	require.Len(t, plan.Files, 1)

	file := plan.Files[0]

	t.Run("plans generated file package", func(t *testing.T) {
		assert.Equal(t, "yirapb/v1/yira.proto", file.ProtoPath)
		assert.Equal(t, "github.com/seeruk/tego/internal/tego/testdata/yira/v1", file.Package.ImportPath)
		assert.Equal(t, "yirav1", file.Package.Name)
		assert.Equal(t, FileOutputPlan{
			Directory:     "github.com/seeruk/tego/internal/tego/testdata/yira/v1",
			Filename:      "yira.tego.go",
			Path:          "github.com/seeruk/tego/internal/tego/testdata/yira/v1/yira.tego.go",
			GeneratorPath: "github.com/seeruk/tego/internal/tego/testdata/yira/v1/yira.tego.go",
		}, file.Output)
		assert.Empty(t, file.Diagnostics)
	})

	t.Run("includes enum plans", func(t *testing.T) {
		require.Len(t, file.Enums, 1)
		assert.Equal(t, protoreflect.FullName("yirapb.v1.TicketStatus"), file.Enums[0].ProtoName)
	})

	t.Run("includes ordinary struct plans", func(t *testing.T) {
		require.NotZero(t, structByProtoName(t, file, "yirapb.v1.Ticket"))
		require.NotZero(t, structByProtoName(t, file, "yirapb.v1.TicketInput"))
		require.NotZero(t, structByProtoName(t, file, "yirapb.v1.Person"))

		assert.False(t, hasStructPlan(file, "yirapb.v1.NullablePerson"))
		assert.False(t, hasStructPlan(file, "yirapb.v1.People"))
		assert.False(t, hasStructPlan(file, "yirapb.v1.TicketsByPeople"))
	})

	t.Run("plans field tags and scalar types", func(t *testing.T) {
		ticket := structByProtoName(t, file, "yirapb.v1.Ticket")
		require.Len(t, ticket.Fields, 11)

		id := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.id")
		assert.Equal(t, "ID", id.Name)
		assert.Equal(t, TypeKindScalar, id.Type.Kind)
		assert.Equal(t, ScalarKindString, id.Type.Scalar)
		require.Len(t, id.Tags, 1)
		assert.Equal(t, StructTagPlan{Key: "json", Value: "id,omitempty"}, id.Tags[0])

		title := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.title")
		require.Len(t, title.Tags, 1)
		assert.Equal(t, StructTagPlan{Key: "json", Value: "title,omitempty"}, title.Tags[0])
	})

	t.Run("plans custom enum struct map and slice types", func(t *testing.T) {
		ticket := structByProtoName(t, file, "yirapb.v1.Ticket")

		description := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.description")
		descriptionCustom := requirePointerElem(t, description.Type, TypeKindCustom)
		assert.Equal(t, GoTypeRef{ImportPath: plannerTestPkg, Name: "Description"}, descriptionCustom.Ref)
		assert.Equal(t, GoSymbolRef{ImportPath: plannerTestPkg, Name: "DescriptionFromProto"}, descriptionCustom.Custom.FromProto)
		assert.Equal(t, GoSymbolRef{ImportPath: plannerTestPkg, Name: "DescriptionToProto"}, descriptionCustom.Custom.ToProto)

		status := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.status")
		assert.Equal(t, TypeKindEnum, status.Type.Kind)
		assert.Equal(t, "TicketStatus", status.Type.Ref.Name)

		assignee := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.assignee")
		assigneeElem := requirePointerElem(t, assignee.Type, TypeKindStruct)
		assert.Equal(t, "Person", assigneeElem.Ref.Name)

		metadata := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.metadata")
		assert.Equal(t, TypeKindMap, metadata.Type.Kind)
		assert.Equal(t, ScalarKindString, metadata.Type.Key.Scalar)
		assert.Equal(t, ScalarKindString, metadata.Type.Value.Scalar)

		watchersByRole := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.watchers_by_role")
		assert.Equal(t, TypeKindStruct, watchersByRole.Type.Value.Kind)
		assert.Equal(t, "Person", watchersByRole.Type.Value.Ref.Name)

		watcherIDs := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.watcher_ids")
		assert.Equal(t, "WatcherIDs", watcherIDs.Name)
		assert.Equal(t, TypeKindSlice, watcherIDs.Type.Kind)
		assert.Equal(t, ScalarKindString, watcherIDs.Type.Elem.Scalar)
	})

	t.Run("plans omittable input fields", func(t *testing.T) {
		input := structByProtoName(t, file, "yirapb.v1.TicketInput")

		title := fieldPlanByProtoName(t, input, "yirapb.v1.TicketInput.title")
		assert.Equal(t, TypeKindOmittable, title.Type.Kind)
		assert.Equal(t, ScalarKindString, title.Type.Elem.Scalar)

		assignee := fieldPlanByProtoName(t, input, "yirapb.v1.TicketInput.assignee")
		assert.Equal(t, TypeKindOmittable, assignee.Type.Kind)
		assert.Equal(t, TypeKindPointer, assignee.Type.Elem.Kind)

		version := fieldPlanByProtoName(t, input, "yirapb.v1.TicketInput.version")
		assert.Equal(t, TypeKindScalar, version.Type.Kind)
	})
}

func hasStructPlan(file FilePlan, name protoreflect.FullName) bool {
	for _, structure := range file.Structs {
		if structure.ProtoName == name {
			return true
		}
	}
	return false
}

func TestPlannerPlanFileOutput(t *testing.T) {
	t.Run("strips module from default output path", func(t *testing.T) {
		file := protoFileWithOutput("yirapb/v1/yira.proto", "github.com/seeruk/tego/internal/tego/testdata/yira/v1;yirav1", "")

		plan := NewPlanner(WithModulePath("github.com/seeruk/tego")).planFile(file, &ShapeIndex{})

		require.Empty(t, plan.Diagnostics)
		assert.Equal(t, FileOutputPlan{
			Directory:     "internal/tego/testdata/yira/v1",
			Filename:      "yira.tego.go",
			Path:          "internal/tego/testdata/yira/v1/yira.tego.go",
			GeneratorPath: "github.com/seeruk/tego/internal/tego/testdata/yira/v1/yira.tego.go",
		}, plan.Output)
	})

	t.Run("splits explicit output path", func(t *testing.T) {
		file := protoFileWithOutput("yirapb/v1/yira.proto", "github.com/seeruk/tego/internal/tego/testdata/yira/v1;yirav1", "custom/yira.model.go")

		plan := NewPlanner().planFile(file, &ShapeIndex{})

		require.Empty(t, plan.Diagnostics)
		assert.Equal(t, FileOutputPlan{
			Directory:     "custom",
			Filename:      "yira.model.go",
			Path:          "custom/yira.model.go",
			GeneratorPath: "custom/yira.model.go",
		}, plan.Output)
	})

	t.Run("prefixes explicit generator path with module", func(t *testing.T) {
		file := protoFileWithOutput("yirapb/v1/yira.proto", "github.com/seeruk/tego/internal/tego/testdata/yira/v1;yirav1", "internal/tego/testdata/yira/v1/yira.model.go")

		plan := NewPlanner(WithModulePath("github.com/seeruk/tego")).planFile(file, &ShapeIndex{})

		require.Empty(t, plan.Diagnostics)
		assert.Equal(t, "internal/tego/testdata/yira/v1", plan.Output.Directory)
		assert.Equal(t, "yira.model.go", plan.Output.Filename)
		assert.Equal(t, "internal/tego/testdata/yira/v1/yira.model.go", plan.Output.Path)
		assert.Equal(t, "github.com/seeruk/tego/internal/tego/testdata/yira/v1/yira.model.go", plan.Output.GeneratorPath)
	})

	t.Run("reports invalid output paths", func(t *testing.T) {
		tests := []struct {
			name       string
			outputPath string
			diagnostic string
		}{
			{name: "absolute", outputPath: "/tmp/yira.go", diagnostic: "must be relative"},
			{name: "parent traversal", outputPath: "internal/../yira.go", diagnostic: "must not contain parent traversal"},
			{name: "empty filename", outputPath: "internal/", diagnostic: "must include a filename"},
			{name: "non go filename", outputPath: "internal/yira.txt", diagnostic: "must end in .go"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				file := protoFileWithOutput("yirapb/v1/yira.proto", "github.com/seeruk/tego/internal/tego/testdata/yira/v1;yirav1", tt.outputPath)

				plan := NewPlanner().planFile(file, &ShapeIndex{})

				require.Len(t, plan.Diagnostics, 1)
				assert.True(t, HasFatalDiagnostics(plan.Diagnostics))
				assert.Contains(t, plan.Diagnostics[0].Message, tt.diagnostic)
			})
		}
	})

	t.Run("reports module mismatch for default output", func(t *testing.T) {
		file := protoFileWithOutput("yirapb/v1/yira.proto", "github.com/example/yira/v1;yirav1", "")

		plan := NewPlanner(WithModulePath("github.com/seeruk/tego")).planFile(file, &ShapeIndex{})

		require.Len(t, plan.Diagnostics, 1)
		assert.True(t, HasFatalDiagnostics(plan.Diagnostics))
		assert.Contains(t, plan.Diagnostics[0].Message, "outside module")
	})

	t.Run("reports module mismatch for explicit output", func(t *testing.T) {
		file := protoFileWithOutput("yirapb/v1/yira.proto", "github.com/example/yira/v1;yirav1", "internal/yira.tego.go")

		plan := NewPlanner(WithModulePath("github.com/seeruk/tego")).planFile(file, &ShapeIndex{})

		require.Len(t, plan.Diagnostics, 1)
		assert.True(t, HasFatalDiagnostics(plan.Diagnostics))
		assert.Contains(t, plan.Diagnostics[0].Message, "outside module")
	})
}

func protoFileWithOutput(protoPath, goPackage, outputPath string) *ProtoFile {
	options := &tegopb.FileOptions{}
	options.SetGoPackage(goPackage)
	if outputPath != "" {
		options.SetOutputPath(outputPath)
	}
	return &ProtoFile{
		Path:     protoPath,
		Generate: true,
		Options:  options,
	}
}
