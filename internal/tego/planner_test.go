package tego

import (
	"strings"
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
		require.Empty(t, file.Diagnostics)
	})

	t.Run("includes enum plans", func(t *testing.T) {
		require.Len(t, file.Enums, 4)
		assert.Equal(t, protoreflect.FullName("yirapb.v1.TicketStatus"), file.Enums[0].ProtoName)

		visibility := enumByProtoName(t, file, "yirapb.v1.Ticket.Visibility")
		assert.Equal(t, "TicketVisibility", visibility.Name)
		public := enumConstantByProtoName(t, visibility, "yirapb.v1.Ticket.VISIBILITY_PUBLIC")
		assert.Equal(t, "TicketVisibilityPublic", public.Name)

		eventKind := enumByProtoName(t, file, "yirapb.v1.TicketEvent.Kind")
		assert.Equal(t, "TicketEventKind", eventKind.Name)
	})

	t.Run("includes ordinary struct plans", func(t *testing.T) {
		require.NotZero(t, structByProtoName(t, file, "yirapb.v1.ListTicketsRequest"))
		require.NotZero(t, structByProtoName(t, file, "yirapb.v1.ListTicketsResponse"))
		require.NotZero(t, structByProtoName(t, file, "yirapb.v1.Ticket"))
		require.NotZero(t, structByProtoName(t, file, "yirapb.v1.TicketDraft"))
		require.NotZero(t, structByProtoName(t, file, "yirapb.v1.TicketPatch"))
		require.NotZero(t, structByProtoName(t, file, "yirapb.v1.TicketEvent"))
		require.NotZero(t, structByProtoName(t, file, "yirapb.v1.Person"))

		assert.False(t, hasStructPlan(file, "yirapb.v1.NullablePerson"))
		assert.False(t, hasStructPlan(file, "yirapb.v1.DueDate"))
		assert.False(t, hasStructPlan(file, "yirapb.v1.Labels"))
		assert.False(t, hasStructPlan(file, "yirapb.v1.PersonList"))
		assert.False(t, hasStructPlan(file, "yirapb.v1.TicketsByStatus"))
	})

	t.Run("includes ordinary mapping plans", func(t *testing.T) {
		require.NotZero(t, mappingByProtoName(t, file, "yirapb.v1.ListTicketsRequest"))
		require.NotZero(t, mappingByProtoName(t, file, "yirapb.v1.ListTicketsResponse"))
		require.NotZero(t, mappingByProtoName(t, file, "yirapb.v1.Ticket"))
		require.NotZero(t, mappingByProtoName(t, file, "yirapb.v1.TicketDraft"))
		require.NotZero(t, mappingByProtoName(t, file, "yirapb.v1.TicketPatch"))
		require.NotZero(t, mappingByProtoName(t, file, "yirapb.v1.TicketEvent"))
		require.NotZero(t, mappingByProtoName(t, file, "yirapb.v1.Person"))

		assert.False(t, hasMappingPlan(file, "yirapb.v1.NullablePerson"))
		assert.False(t, hasMappingPlan(file, "yirapb.v1.DueDate"))
		assert.False(t, hasMappingPlan(file, "yirapb.v1.Labels"))
		assert.False(t, hasMappingPlan(file, "yirapb.v1.PersonList"))
		assert.False(t, hasMappingPlan(file, "yirapb.v1.TicketsByStatus"))
	})

	t.Run("plans mapping function names and errability", func(t *testing.T) {
		ticket := mappingByProtoName(t, file, "yirapb.v1.Ticket")
		assert.Equal(t, "TicketFromProto", ticket.FromProto.Name)
		assert.Equal(t, "TicketToProto", ticket.ToProto.Name)
		assert.Equal(t, "t", ticket.ToProto.ReceiverName)
		assert.True(t, ticket.FromProto.CanError)
		assert.True(t, ticket.ToProto.CanError)

		person := mappingByProtoName(t, file, "yirapb.v1.Person")
		assert.Equal(t, "PersonFromProto", person.FromProto.Name)
		assert.Equal(t, "PersonToProto", person.ToProto.Name)
		assert.Equal(t, "p", person.ToProto.ReceiverName)
		assert.False(t, person.FromProto.CanError)
		assert.False(t, person.ToProto.CanError)

		request := mappingByProtoName(t, file, "yirapb.v1.UpdateTicketRequest")
		assert.Equal(t, "utr", request.ToProto.ReceiverName)
		assert.True(t, request.FromProto.CanError)
		assert.True(t, request.ToProto.CanError)
	})

	t.Run("plans representative field mappings", func(t *testing.T) {
		ticket := mappingByProtoName(t, file, "yirapb.v1.Ticket")

		id := fieldMappingByProtoName(t, ticket, "yirapb.v1.Ticket.id")
		assert.Equal(t, "ID", id.Name)
		assert.Equal(t, "Id", id.Proto.Name)
		assert.Equal(t, MappingValueKindDirect, id.FromProto.Kind)

		dueDate := fieldMappingByProtoName(t, ticket, "yirapb.v1.Ticket.due_date")
		assert.Equal(t, MappingValueKindFlatten, dueDate.FromProto.Kind)
		require.NotNil(t, dueDate.FromProto.Elem)
		assert.Equal(t, MappingValueKindCustom, dueDate.FromProto.Elem.Kind)
		assert.True(t, dueDate.FromProto.CanError)
		assert.True(t, dueDate.ToProto.CanError)

		status := fieldMappingByProtoName(t, ticket, "yirapb.v1.Ticket.status")
		assert.Equal(t, MappingValueKindEnum, status.FromProto.Kind)
		assert.Equal(t, MappingValueKindEnum, status.ToProto.Kind)

		assignee := fieldMappingByProtoName(t, ticket, "yirapb.v1.Ticket.assignee")
		assert.Equal(t, MappingValueKindNullable, assignee.FromProto.Kind)
		assert.Equal(t, MappingValueKindNullable, assignee.ToProto.Kind)

		source := fieldMappingByProtoName(t, ticket, "yirapb.v1.Ticket.source")
		assert.Equal(t, MappingValueKindOneof, source.FromProto.Kind)
		require.NotNil(t, source.FromProto.Oneof)
		require.Len(t, source.FromProto.Oneof.Variants, 2)
		assert.Equal(t, "TicketManualSource", source.FromProto.Oneof.Variants[0].Name)
		assert.Equal(t, "TicketIntegrationSource", source.FromProto.Oneof.Variants[1].Name)

		metadata := fieldMappingByProtoName(t, ticket, "yirapb.v1.Ticket.metadata")
		require.NotNil(t, metadata.FromProto.Dynamic)
		require.NotNil(t, metadata.ToProto.Dynamic)
		assert.Equal(t, MappingValueKindDynamic, metadata.FromProto.Kind)
		assert.Equal(t, MappingDynamicKindStruct, metadata.FromProto.Dynamic.Kind)
		assert.False(t, metadata.FromProto.CanError)
		assert.Equal(t, MappingValueKindDynamic, metadata.ToProto.Kind)
		assert.Equal(t, MappingDynamicKindStruct, metadata.ToProto.Dynamic.Kind)
		assert.True(t, metadata.ToProto.CanError)

		event := mappingByProtoName(t, file, "yirapb.v1.TicketEvent")
		payload := fieldMappingByProtoName(t, event, "yirapb.v1.TicketEvent.payload")
		require.NotNil(t, payload.FromProto.Dynamic)
		assert.Equal(t, MappingDynamicKindValue, payload.FromProto.Dynamic.Kind)
		assert.True(t, payload.ToProto.CanError)

		attachments := fieldMappingByProtoName(t, event, "yirapb.v1.TicketEvent.attachments")
		require.NotNil(t, attachments.FromProto.Dynamic)
		assert.Equal(t, MappingDynamicKindListValue, attachments.FromProto.Dynamic.Kind)
		assert.True(t, attachments.ToProto.CanError)
	})

	t.Run("plans field tags and scalar types", func(t *testing.T) {
		ticket := structByProtoName(t, file, "yirapb.v1.Ticket")
		require.Len(t, ticket.Fields, 15)

		id := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.id")
		assert.Equal(t, "ID", id.Name)
		assert.Equal(t, TypeKindScalar, id.Type.Kind)
		assert.Equal(t, ScalarKindString, id.Type.Scalar)
		require.Len(t, id.Tags, 1)
		assert.Equal(t, StructTagPlan{Key: "json", Value: "id,omitempty"}, id.Tags[0])

		source := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.source")
		assert.Equal(t, "Source", source.Name)
		assert.Equal(t, TypeKindOneof, source.Type.Kind)
		assert.Equal(t, "TicketSource", source.Type.Ref.Name)
	})

	t.Run("plans custom enum struct map and slice types", func(t *testing.T) {
		ticket := structByProtoName(t, file, "yirapb.v1.Ticket")

		dueDate := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.due_date")
		assert.Equal(t, TypeKindCustom, dueDate.Type.Kind)
		assert.Equal(t, GoTypeRef{ImportPath: "github.com/seeruk/tego/internal/tego/testdata/yira/v1/types", Name: "Date"}, dueDate.Type.Ref)
		assert.Equal(t, GoSymbolRef{ImportPath: "github.com/seeruk/tego/internal/tego/testdata/yira/v1/types", Name: "DateFromProto"}, dueDate.Type.Custom.FromProto)
		assert.Equal(t, GoSymbolRef{ImportPath: "github.com/seeruk/tego/internal/tego/testdata/yira/v1/types", Name: "DateToProto"}, dueDate.Type.Custom.ToProto)

		status := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.status")
		assert.Equal(t, TypeKindEnum, status.Type.Kind)
		assert.Equal(t, "github.com/seeruk/tego/internal/tego/testdata/yira/v1", status.Type.Ref.ImportPath)
		assert.Equal(t, "TicketStatus", status.Type.Ref.Name)

		assignee := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.assignee")
		assigneeElem := requirePointerElem(t, assignee.Type, TypeKindStruct)
		assert.Equal(t, "github.com/seeruk/tego/internal/tego/testdata/yira/v1", assigneeElem.Ref.ImportPath)
		assert.Equal(t, "Person", assigneeElem.Ref.Name)

		watchers := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.watchers")
		assert.Equal(t, TypeKindSlice, watchers.Type.Kind)
		assert.Equal(t, TypeKindStruct, watchers.Type.Elem.Kind)
		assert.Equal(t, "Person", watchers.Type.Elem.Ref.Name)

		labels := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.labels")
		assert.Equal(t, TypeKindCustom, labels.Type.Kind)
		assert.Equal(t, GoTypeRef{
			ImportPath: "github.com/seeruk/tego/internal/tego/testdata/yira/v1/types",
			Name:       "Set",
			Args: []GoTypeRef{{
				ImportPath: "github.com/seeruk/tego/internal/tego/testdata/yira/v1/types",
				Name:       "Label",
			}},
		}, labels.Type.Ref)

		visibility := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.visibility")
		assert.Equal(t, TypeKindEnum, visibility.Type.Kind)
		assert.Equal(t, "TicketVisibility", visibility.Type.Ref.Name)

		events := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.events")
		assert.Equal(t, TypeKindSlice, events.Type.Kind)
		assert.Equal(t, TypeKindStruct, events.Type.Elem.Kind)
		assert.Equal(t, "TicketEvent", events.Type.Elem.Ref.Name)

		metadata := fieldPlanByProtoName(t, ticket, "yirapb.v1.Ticket.metadata")
		assert.Equal(t, dynamicStructType(), metadata.Type)

		event := structByProtoName(t, file, "yirapb.v1.TicketEvent")
		payload := fieldPlanByProtoName(t, event, "yirapb.v1.TicketEvent.payload")
		assert.Equal(t, dynamicValueType(), payload.Type)

		attachments := fieldPlanByProtoName(t, event, "yirapb.v1.TicketEvent.attachments")
		assert.Equal(t, dynamicListValueType(), attachments.Type)
	})

	t.Run("plans omittable patch fields", func(t *testing.T) {
		patch := structByProtoName(t, file, "yirapb.v1.TicketPatch")

		title := fieldPlanByProtoName(t, patch, "yirapb.v1.TicketPatch.title")
		assert.Equal(t, TypeKindOmittable, title.Type.Kind)
		assert.Equal(t, ScalarKindString, title.Type.Elem.Scalar)

		assignee := fieldPlanByProtoName(t, patch, "yirapb.v1.TicketPatch.assignee")
		assert.Equal(t, TypeKindOmittable, assignee.Type.Kind)
		assert.Equal(t, TypeKindPointer, assignee.Type.Elem.Kind)

		metadata := fieldPlanByProtoName(t, patch, "yirapb.v1.TicketPatch.metadata")
		assert.Equal(t, TypeKindOmittable, metadata.Type.Kind)
		assert.Equal(t, dynamicStructType(), *metadata.Type.Elem)

		version := fieldPlanByProtoName(t, patch, "yirapb.v1.TicketPatch.version")
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

func hasMappingPlan(file FilePlan, name protoreflect.FullName) bool {
	for _, mapping := range file.Mappings {
		if mapping.ProtoName == name {
			return true
		}
	}
	return false
}

func enumByProtoName(t *testing.T, file FilePlan, name protoreflect.FullName) EnumPlan {
	t.Helper()

	for _, enum := range file.Enums {
		if enum.ProtoName == name {
			return enum
		}
	}

	t.Fatalf("enum %q not found", name)
	return EnumPlan{}
}

func TestPlannerPlanNestedDeclarations(t *testing.T) {
	t.Run("uses explicit nested declaration names", func(t *testing.T) {
		file := protoFileWithOutput("nested.proto", "github.com/example/nested;nested", "")
		parent := plannerMessage("example.v1.Parent", "Parent")
		child := plannerMessage("example.v1.Parent.Child", "Child")
		child.Options.SetName("Inner")
		status := protoEnum("example.v1.Parent.Status", "Status")
		status.Parent = parent
		status.Options.SetName("State")
		child.Parent = parent
		parent.Messages = []*ProtoMessage{child}
		parent.Enums = []*ProtoEnum{status}
		attachMessagesToFile(file, parent)

		plan := NewPlanner().planFile(file, &ShapeIndex{})

		require.Empty(t, plan.Diagnostics)
		assert.Equal(t, "Inner", structByProtoName(t, plan, "example.v1.Parent.Child").Name)
		assert.Equal(t, "State", enumByProtoName(t, plan, "example.v1.Parent.Status").Name)
	})

	t.Run("reports planned name collisions", func(t *testing.T) {
		file := protoFileWithOutput("nested.proto", "github.com/example/nested;nested", "")
		fooBar := plannerMessage("example.v1.FooBar", "FooBar")
		foo := plannerMessage("example.v1.Foo", "Foo")
		bar := plannerMessage("example.v1.Foo.Bar", "Bar")
		bar.Parent = foo
		foo.Messages = []*ProtoMessage{bar}
		attachMessagesToFile(file, fooBar, foo)

		plan := NewPlanner().planFile(file, &ShapeIndex{})

		require.NotEmpty(t, plan.Diagnostics)
		assert.True(t, HasFatalDiagnostics(plan.Diagnostics))
		assert.Contains(t, plan.Diagnostics[len(plan.Diagnostics)-1].Message, `planned Go name "FooBar"`)
	})
}

func TestPlannerPlanOneofDeclarations(t *testing.T) {
	t.Run("collects oneof plans", func(t *testing.T) {
		file := protoFileWithOutput("oneof.proto", "github.com/example/oneof;oneof", "")
		event := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		plannerOneof(event, "value", field("comment", protoreflect.StringKind))
		attachMessagesToFile(file, event)

		plan := NewPlanner().planFile(file, &ShapeIndex{})

		require.Len(t, plan.Oneofs, 1)
		assert.Equal(t, "TicketEventValue", plan.Oneofs[0].Name)
		assert.Equal(t, "TicketEventValue", fieldPlanByProtoName(t, plan.Structs[0], "example.v1.TicketEvent.value").Type.Ref.Name)
		assert.Empty(t, plan.Diagnostics)
	})

	t.Run("reports oneof interface name collisions", func(t *testing.T) {
		file := protoFileWithOutput("oneof.proto", "github.com/example/oneof;oneof", "")
		event := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		plannerOneof(event, "value", field("comment", protoreflect.StringKind))
		eventValue := plannerMessage("example.v1.TicketEventValue", "TicketEventValue")
		attachMessagesToFile(file, event, eventValue)

		plan := NewPlanner().planFile(file, &ShapeIndex{})

		require.NotEmpty(t, plan.Diagnostics)
		assert.True(t, HasFatalDiagnostics(plan.Diagnostics))
		assert.Contains(t, diagnosticsText(plan.Diagnostics), `planned Go name "TicketEventValue"`)
	})

	t.Run("reports oneof variant name collisions", func(t *testing.T) {
		file := protoFileWithOutput("oneof.proto", "github.com/example/oneof;oneof", "")
		event := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		plannerOneof(event, "value", field("status", protoreflect.StringKind))
		eventStatus := plannerMessage("example.v1.TicketEventStatus", "TicketEventStatus")
		attachMessagesToFile(file, event, eventStatus)

		plan := NewPlanner().planFile(file, &ShapeIndex{})

		require.NotEmpty(t, plan.Diagnostics)
		assert.True(t, HasFatalDiagnostics(plan.Diagnostics))
		assert.Contains(t, diagnosticsText(plan.Diagnostics), `planned Go name "TicketEventStatus"`)
	})
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

func attachMessagesToFile(file *ProtoFile, messages ...*ProtoMessage) {
	file.Messages = messages
	for _, message := range messages {
		attachMessageToFile(file, message)
	}
}

func attachMessageToFile(file *ProtoFile, message *ProtoMessage) {
	message.File = file
	for _, enum := range message.Enums {
		enum.File = file
	}
	for _, oneof := range message.Oneofs {
		oneof.File = file
		for _, field := range oneof.Fields {
			field.File = file
		}
	}
	for _, field := range message.Fields {
		field.File = file
	}
	for _, nested := range message.Messages {
		attachMessageToFile(file, nested)
	}
}

func diagnosticsText(diagnostics []Diagnostic) string {
	var out strings.Builder
	for _, diagnostic := range diagnostics {
		out.WriteString(diagnostic.Message)
		out.WriteByte('\n')
	}
	return out.String()
}
