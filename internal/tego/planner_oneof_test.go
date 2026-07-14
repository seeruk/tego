package tego

import (
	"testing"

	"github.com/seeruk/tego/tegopb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestPlannerPlanOneofs(t *testing.T) {
	t.Run("plans oneof interface field and variants", func(t *testing.T) {
		message := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		person := plannerMessage("example.v1.Person", "Person")
		status := protoEnum("example.v1.TicketStatus", "TicketStatus")
		oneof := plannerOneof(
			message, "value",
			field("comment", protoreflect.StringKind),
			enumField("status", status),
			messageField("assignee", person),
			fieldWithPlannerGoType("label", plannerGoType(
				plannerTestPkg+".CustomString",
				plannerTestPkg+".CustomStringFromProto",
				plannerTestPkg+".CustomStringToProto",
				false,
			)),
		)

		structPlan, diagnostics, ok := NewPlanner().planStruct(message, &ShapeIndex{})
		require.True(t, ok)
		require.Empty(t, diagnostics)
		require.Len(t, structPlan.Fields, 1)
		assert.Equal(t, "Value", structPlan.Fields[0].Name)
		assert.Equal(t, TypeKindOneof, structPlan.Fields[0].Type.Kind)
		assert.Equal(t, "TicketEventValue", structPlan.Fields[0].Type.Ref.Name)

		oneofPlan, diagnostics := NewPlanner().planOneof(oneof, &ShapeIndex{})
		require.Empty(t, diagnostics)
		assert.Equal(t, "TicketEventValue", oneofPlan.Name)
		assert.Equal(t, "isTicketEventValue", oneofPlan.MarkerMethod)
		require.Len(t, oneofPlan.Variants, 4)
		assert.Equal(t, "TicketEventComment", oneofPlan.Variants[0].Name)
		assert.Equal(t, "Comment", oneofPlan.Variants[0].FieldName)
		assert.Equal(t, ScalarKindString, oneofPlan.Variants[0].Type.Scalar)
		assert.Equal(t, "TicketEventStatus", oneofPlan.Variants[1].Name)
		assert.Equal(t, TypeKindEnum, oneofPlan.Variants[1].Type.Kind)
		assert.Equal(t, "TicketEventAssignee", oneofPlan.Variants[2].Name)
		assert.Equal(t, TypeKindStruct, oneofPlan.Variants[2].Type.Kind)
		assert.Equal(t, "TicketEventLabel", oneofPlan.Variants[3].Name)
		assert.Equal(t, TypeKindCustom, oneofPlan.Variants[3].Type.Kind)
	})

	t.Run("uses casing helpers for multi word names", func(t *testing.T) {
		message := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		oneof := plannerOneof(
			message, "api_response",
			field("http_url", protoreflect.StringKind),
			field("status_comment", protoreflect.StringKind),
		)

		structPlan, diagnostics, ok := NewPlanner().planStruct(message, &ShapeIndex{})
		require.True(t, ok)
		require.Empty(t, diagnostics)
		require.Len(t, structPlan.Fields, 1)
		assert.Equal(t, "APIResponse", structPlan.Fields[0].Name)
		assert.Equal(t, "TicketEventAPIResponse", structPlan.Fields[0].Type.Ref.Name)

		oneofPlan, diagnostics := NewPlanner().planOneof(oneof, &ShapeIndex{})
		require.Empty(t, diagnostics)
		assert.Equal(t, "TicketEventAPIResponse", oneofPlan.Name)
		require.Len(t, oneofPlan.Variants, 2)
		assert.Equal(t, "TicketEventHTTPURL", oneofPlan.Variants[0].Name)
		assert.Equal(t, "HTTPURL", oneofPlan.Variants[0].FieldName)
		assert.Equal(t, "TicketEventStatusComment", oneofPlan.Variants[1].Name)
		assert.Equal(t, "StatusComment", oneofPlan.Variants[1].FieldName)
	})

	t.Run("plans automatic JSON tags for oneof and variant fields", func(t *testing.T) {
		message := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		setMessageFieldsJSONTags(message.Options, tegopb.CasingStyle_CASING_STYLE_SNAKE_CASE)
		watcherIDs := field("watcher_ids", protoreflect.StringKind)
		watcherIDs.Options = &tegopb.FieldOptions{}
		watcherIDs.Options.SetName("subscriber_ids")
		oneof := plannerOneof(
			message, "api_response",
			field("http_url", protoreflect.StringKind),
			watcherIDs,
		)

		structPlan, diagnostics, ok := NewPlanner().planStruct(message, &ShapeIndex{})
		require.True(t, ok)
		require.Empty(t, diagnostics)
		require.Len(t, structPlan.Fields, 1)
		assert.Equal(t, []StructTagPlan{{Key: "json", Value: "api_response"}}, structPlan.Fields[0].Tags)

		oneofPlan, diagnostics := NewPlanner().planOneof(oneof, &ShapeIndex{})
		require.Empty(t, diagnostics)
		require.Len(t, oneofPlan.Variants, 2)
		assert.Equal(t, []StructTagPlan{{Key: "json", Value: "http_url"}}, oneofPlan.Variants[0].Tags)
		assert.Equal(t, []StructTagPlan{{Key: "json", Value: "subscriber_ids"}}, oneofPlan.Variants[1].Tags)
	})

	t.Run("omits oneof variants", func(t *testing.T) {
		message := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		oneof := plannerOneof(
			message, "value",
			field("comment", protoreflect.StringKind),
			omittedPlannerField(field("internal_note", protoreflect.StringKind)),
		)

		oneofPlan, diagnostics := NewPlanner().planOneof(oneof, &ShapeIndex{})

		require.Empty(t, diagnostics)
		require.Len(t, oneofPlan.Variants, 1)
		assert.Equal(t, "TicketEventComment", oneofPlan.Variants[0].Name)
	})

	t.Run("uses enum go type for variants", func(t *testing.T) {
		index := buildYiraDescriptorIndex(t)
		status := requireEnum(t, index, "yirapb.v1.TicketStatus")
		status.Options.SetGoType(customTicketStatusGoType())
		message := plannerMessage("example.v1.Event", "Event")
		oneof := plannerOneof(message, "value", enumField("status", status))

		plan, diagnostics := NewPlanner().planOneof(oneof, &ShapeIndex{})

		require.Empty(t, diagnostics)
		require.Len(t, plan.Variants, 1)
		assert.Equal(t, TypeKindCustom, plan.Variants[0].Type.Kind)
		assert.Equal(t, "CustomTicketStatus", plan.Variants[0].Type.Custom.Ref.Name)
	})

	t.Run("allows all variants to be omitted", func(t *testing.T) {
		message := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		oneof := plannerOneof(
			message, "value",
			omittedPlannerField(field("comment", protoreflect.StringKind)),
		)

		oneofPlan, diagnostics := NewPlanner().planOneof(oneof, &ShapeIndex{})

		require.Empty(t, diagnostics)
		assert.Empty(t, oneofPlan.Variants)
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

		requireFatalDiagnostic(t, plan.Diagnostics, `planned Go name "TicketEventValue"`)
	})

	t.Run("reports oneof variant name collisions", func(t *testing.T) {
		file := protoFileWithOutput("oneof.proto", "github.com/example/oneof;oneof", "")
		event := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		plannerOneof(event, "value", field("status", protoreflect.StringKind))
		eventStatus := plannerMessage("example.v1.TicketEventStatus", "TicketEventStatus")
		attachMessagesToFile(file, event, eventStatus)

		plan := NewPlanner().planFile(file, &ShapeIndex{})

		requireFatalDiagnostic(t, plan.Diagnostics, `planned Go name "TicketEventStatus"`)
	})
}
