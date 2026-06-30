package tego

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestPlannerPlanMappingValues(t *testing.T) {
	planner := NewPlanner()
	stringType := scalarType(ScalarKindString)
	int64Type := scalarType(ScalarKindInt64)
	uint64Type := scalarType(ScalarKindUint64)

	t.Run("plans direct scalar assignment", func(t *testing.T) {
		plan := planner.planMappingValue(stringType, stringType, mappingDirectionFromProto)

		assert.Equal(t, MappingValueKindDirect, plan.Kind)
		assert.False(t, plan.CanError)
	})

	t.Run("plans opinionated scalar casts", func(t *testing.T) {
		intPlan := planner.planMappingValue(int64Type, int64Type, mappingDirectionFromProto)
		uintPlan := planner.planMappingValue(uint64Type, uint64Type, mappingDirectionToProto)

		assert.Equal(t, MappingValueKindScalarCast, intPlan.Kind)
		assert.Equal(t, MappingValueKindScalarCast, uintPlan.Kind)
	})

	t.Run("plans enum conversion", func(t *testing.T) {
		source := TypePlan{Kind: TypeKindEnum, Ref: GoTypeRef{ImportPath: "example.com/pb", Name: "Status"}}
		target := TypePlan{Kind: TypeKindEnum, Ref: GoTypeRef{ImportPath: "example.com/tego", Name: "Status"}}

		plan := planner.planMappingValue(source, target, mappingDirectionFromProto)

		require.NotNil(t, plan.Enum)
		assert.Equal(t, MappingValueKindEnum, plan.Kind)
		assert.Equal(t, target, plan.Enum.Target)
	})

	t.Run("plans struct mapper calls", func(t *testing.T) {
		source := pointerType(TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: "example.com/pb", Name: "Person"}})
		target := TypePlan{Kind: TypeKindStruct, Ref: GoTypeRef{ImportPath: "example.com/tego", Name: "Person"}}

		fromProto := planner.planMappingValue(source, target, mappingDirectionFromProto)
		toProto := planner.planMappingValue(target, source, mappingDirectionToProto)

		require.NotNil(t, fromProto.Struct)
		require.NotNil(t, toProto.Struct)
		assert.Equal(t, "PersonFromProto", fromProto.Struct.Name)
		assert.Equal(t, "PersonToProto", toProto.Struct.Name)
	})

	t.Run("plans nullable pointers", func(t *testing.T) {
		target := pointerType(stringType)

		plan := planner.planMappingValue(stringType, target, mappingDirectionFromProto)

		require.NotNil(t, plan.Elem)
		assert.Equal(t, MappingValueKindNullable, plan.Kind)
		assert.Equal(t, MappingValueKindDirect, plan.Elem.Kind)
	})

	t.Run("plans slice element mapping", func(t *testing.T) {
		source := TypePlan{Kind: TypeKindSlice, Elem: &stringType}
		target := TypePlan{Kind: TypeKindSlice, Elem: &stringType}

		plan := planner.planMappingValue(source, target, mappingDirectionFromProto)

		require.NotNil(t, plan.Elem)
		assert.Equal(t, MappingValueKindSlice, plan.Kind)
		assert.Equal(t, MappingValueKindDirect, plan.Elem.Kind)
	})

	t.Run("plans map key and value mapping", func(t *testing.T) {
		source := TypePlan{Kind: TypeKindMap, Key: &stringType, Value: &int64Type}
		target := TypePlan{Kind: TypeKindMap, Key: &stringType, Value: &int64Type}

		plan := planner.planMappingValue(source, target, mappingDirectionFromProto)

		require.NotNil(t, plan.Key)
		require.NotNil(t, plan.Value)
		assert.Equal(t, MappingValueKindMap, plan.Kind)
		assert.Equal(t, MappingValueKindDirect, plan.Key.Kind)
		assert.Equal(t, MappingValueKindScalarCast, plan.Value.Kind)
	})

	t.Run("plans omittable wrapping", func(t *testing.T) {
		field := field("title", protoreflect.StringKind)
		target := TypePlan{Kind: TypeKindOmittable, Elem: &stringType}

		plan := planner.planFieldMappingValue(field, stringType, target, &ShapeIndex{}, mappingDirectionFromProto)

		require.NotNil(t, plan.Elem)
		assert.Equal(t, MappingValueKindOmittable, plan.Kind)
		assert.Equal(t, MappingValueKindDirect, plan.Elem.Kind)
	})
}

func TestPlannerPlanMappingCustomGoTypes(t *testing.T) {
	t.Run("keeps non-erroring conversions non-erroring", func(t *testing.T) {
		field := fieldWithPlannerGoType("custom", plannerGoType(
			plannerTestPkg+".CustomString",
			plannerTestPkg+".CustomStringFromProto",
			plannerTestPkg+".CustomStringToProto",
			false,
		))
		planner := NewPlanner()
		target, diagnostics := planner.planFieldType(field, &ShapeIndex{})
		require.Empty(t, diagnostics)

		fromProto := planner.planMappingValue(planner.planProtoFieldType(field), target, mappingDirectionFromProto)
		toProto := planner.planMappingValue(target, planner.planProtoFieldType(field), mappingDirectionToProto)

		assert.Equal(t, MappingValueKindCustom, fromProto.Kind)
		assert.False(t, fromProto.CanError)
		assert.False(t, toProto.CanError)
	})

	t.Run("marks erroring conversions as erroring", func(t *testing.T) {
		field := fieldWithPlannerGoType("description", plannerGoType(
			plannerTestPkg+".Description",
			plannerTestPkg+".DescriptionFromProto",
			plannerTestPkg+".DescriptionToProto",
			true,
		))
		planner := NewPlanner()
		target, diagnostics := planner.planFieldType(field, &ShapeIndex{})
		require.Empty(t, diagnostics)

		fromProto := planner.planMappingValue(planner.planProtoFieldType(field), target, mappingDirectionFromProto)
		toProto := planner.planMappingValue(target, planner.planProtoFieldType(field), mappingDirectionToProto)

		assert.Equal(t, MappingValueKindCustom, fromProto.Kind)
		assert.True(t, fromProto.CanError)
		assert.True(t, toProto.CanError)
	})

	t.Run("propagates erroring conversions through containers", func(t *testing.T) {
		field := fieldWithPlannerGoType("description", plannerGoType(
			plannerTestPkg+".Description",
			plannerTestPkg+".DescriptionFromProto",
			plannerTestPkg+".DescriptionToProto",
			true,
		))
		planner := NewPlanner()
		custom, diagnostics := planner.planFieldType(field, &ShapeIndex{})
		require.Empty(t, diagnostics)
		source := planner.planProtoFieldType(field)

		sliceSource := TypePlan{Kind: TypeKindSlice, Elem: &source}
		sliceTarget := TypePlan{Kind: TypeKindSlice, Elem: &custom}
		mapSource := TypePlan{Kind: TypeKindMap, Key: &source, Value: &source}
		mapTarget := TypePlan{Kind: TypeKindMap, Key: &custom, Value: &custom}
		omittableTarget := TypePlan{Kind: TypeKindOmittable, Elem: &custom}

		slice := planner.planMappingValue(sliceSource, sliceTarget, mappingDirectionFromProto)
		mapping := planner.planMappingValue(mapSource, mapTarget, mappingDirectionFromProto)
		omittable := planner.planFieldMappingValue(field, source, omittableTarget, &ShapeIndex{}, mappingDirectionFromProto)

		assert.True(t, slice.CanError)
		assert.True(t, mapping.CanError)
		assert.True(t, omittable.CanError)
	})
}

func TestPlannerPlanOneofMappings(t *testing.T) {
	t.Run("plans ordinary oneof mapping from struct fields", func(t *testing.T) {
		message := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		message.GoName = "TicketEvent"
		message.Fields = append(message.Fields, plannerScalarField(message, "id", protoreflect.StringKind))
		oneof := plannerOneof(message, "value", field("comment", protoreflect.StringKind))
		message.Fields = append(message.Fields, plannerScalarField(message, "summary", protoreflect.StringKind))
		structPlan, diagnostics, ok := NewPlanner().planStruct(message, &ShapeIndex{})
		require.True(t, ok)
		require.Empty(t, diagnostics)

		mapping := NewPlanner().planMapping(message, structPlan, &ShapeIndex{})

		require.Len(t, mapping.Fields, 3)
		oneofMapping := mapping.Fields[1]
		require.NotNil(t, oneofMapping.FromProto.Oneof)
		require.NotNil(t, oneofMapping.ToProto.Oneof)
		assert.Equal(t, oneof.FullName, oneofMapping.ProtoName)
		assert.Equal(t, MappingValueKindOneof, oneofMapping.FromProto.Kind)
		assert.Equal(t, "WhichValue", oneofMapping.FromProto.Oneof.Which)
		assert.True(t, oneofMapping.ToProto.CanError)
	})

	t.Run("plans variant metadata and child mappings", func(t *testing.T) {
		message := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		message.GoName = "TicketEvent"
		person := plannerMessage("example.v1.Person", "Person")
		status := protoEnum("example.v1.TicketStatus", "TicketStatus")
		plannerOneof(
			message, "value",
			field("comment", protoreflect.StringKind),
			enumField("status", status),
			messageField("assignee", person),
			fieldWithPlannerGoType("description", plannerGoType(
				plannerTestPkg+".Description",
				plannerTestPkg+".DescriptionFromProto",
				plannerTestPkg+".DescriptionToProto",
				true,
			)),
		)
		structPlan, diagnostics, ok := NewPlanner().planStruct(message, &ShapeIndex{})
		require.True(t, ok)
		require.Empty(t, diagnostics)

		fieldPlan := NewPlanner().planMapping(message, structPlan, &ShapeIndex{}).Fields[0]
		variants := fieldPlan.FromProto.Oneof.Variants

		require.Len(t, variants, 4)
		assert.Equal(t, "TicketEventComment", variants[0].Name)
		assert.Equal(t, "Comment", variants[0].FieldName)
		assert.Equal(t, "TicketEvent_Comment_case", variants[0].Case.Name)
		assert.Equal(t, MappingValueKindDirect, variants[0].Value.Kind)
		assert.Equal(t, MappingValueKindEnum, variants[1].Value.Kind)
		assert.Equal(t, MappingValueKindStruct, variants[2].Value.Kind)
		assert.Equal(t, MappingValueKindCustom, variants[3].Value.Kind)
		assert.True(t, fieldPlan.FromProto.CanError)
		assert.True(t, fieldPlan.ToProto.CanError)
	})

	t.Run("uses casing helpers for multi word names", func(t *testing.T) {
		message := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		plannerOneof(
			message, "api_response",
			field("http_url", protoreflect.StringKind),
		)
		structPlan, diagnostics, ok := NewPlanner().planStruct(message, &ShapeIndex{})
		require.True(t, ok)
		require.Empty(t, diagnostics)

		fieldPlan := NewPlanner().planMapping(message, structPlan, &ShapeIndex{}).Fields[0]

		require.NotNil(t, fieldPlan.FromProto.Oneof)
		require.Len(t, fieldPlan.FromProto.Oneof.Variants, 1)
		assert.Equal(t, "APIResponse", fieldPlan.Name)
		assert.Equal(t, "TicketEventHTTPURL", fieldPlan.FromProto.Oneof.Variants[0].Name)
		assert.Equal(t, "HTTPURL", fieldPlan.FromProto.Oneof.Variants[0].FieldName)
	})

	t.Run("omits oneof variants", func(t *testing.T) {
		message := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		plannerOneof(
			message, "value",
			field("comment", protoreflect.StringKind),
			omittedPlannerField(field("internal_note", protoreflect.StringKind)),
		)
		structPlan, diagnostics, ok := NewPlanner().planStruct(message, &ShapeIndex{})
		require.True(t, ok)
		require.Empty(t, diagnostics)

		fieldPlan := NewPlanner().planMapping(message, structPlan, &ShapeIndex{}).Fields[0]

		require.NotNil(t, fieldPlan.FromProto.Oneof)
		require.Len(t, fieldPlan.FromProto.Oneof.Variants, 1)
		assert.Equal(t, "TicketEventComment", fieldPlan.FromProto.Oneof.Variants[0].Name)
	})

	t.Run("allows all oneof variants to be omitted", func(t *testing.T) {
		message := plannerMessage("example.v1.TicketEvent", "TicketEvent")
		plannerOneof(
			message, "value",
			omittedPlannerField(field("comment", protoreflect.StringKind)),
		)
		structPlan, diagnostics, ok := NewPlanner().planStruct(message, &ShapeIndex{})
		require.True(t, ok)
		require.Empty(t, diagnostics)

		fieldPlan := NewPlanner().planMapping(message, structPlan, &ShapeIndex{}).Fields[0]

		require.NotNil(t, fieldPlan.FromProto.Oneof)
		assert.Empty(t, fieldPlan.FromProto.Oneof.Variants)
		assert.True(t, fieldPlan.ToProto.CanError)
	})

	t.Run("keeps nullable shape oneofs as nullable mappings", func(t *testing.T) {
		descriptorIndex := buildYiraDescriptorIndex(t)
		shapeIndex, err := BuildShapeIndex(descriptorIndex)
		require.NoError(t, err)
		planner := NewPlanner()
		file := planner.planFile(requireFile(t, descriptorIndex, "yirapb/v1/yira.proto"), shapeIndex)
		ticket := mappingByProtoName(t, file, "yirapb.v1.Ticket")

		assignee := fieldMappingByProtoName(t, ticket, "yirapb.v1.Ticket.assignee")

		assert.Equal(t, MappingValueKindNullable, assignee.FromProto.Kind)
		assert.Equal(t, MappingNullableFormOneof, assignee.FromProto.Access.NullableForm)
		assert.Nil(t, assignee.FromProto.Oneof)
	})
}

func mappingByProtoName(t *testing.T, file FilePlan, name protoreflect.FullName) MappingPlan {
	t.Helper()

	for _, mapping := range file.Mappings {
		if mapping.ProtoName == name {
			return mapping
		}
	}

	t.Fatalf("mapping %q not found", name)
	return MappingPlan{}
}

func fieldMappingByProtoName(t *testing.T, mapping MappingPlan, name protoreflect.FullName) FieldMappingPlan {
	t.Helper()

	for _, field := range mapping.Fields {
		if field.ProtoName == name {
			return field
		}
	}

	t.Fatalf("field mapping %q not found", name)
	return FieldMappingPlan{}
}

func plannerScalarField(parent *ProtoMessage, name protoreflect.Name, kind protoreflect.Kind) *ProtoField {
	field := field(name, kind)
	field.FullName = protoreflect.FullName(string(parent.FullName) + "." + string(name))
	field.Parent = parent
	return field
}
