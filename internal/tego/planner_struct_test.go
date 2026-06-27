package tego

import (
	"testing"

	"github.com/seeruk/tego/tegopb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const plannerTestPkg = "github.com/seeruk/tego/internal/tego/testdata/plannertest"

func TestPlannerPlanFieldTypes(t *testing.T) {
	t.Run("flattens nested shapes", func(t *testing.T) {
		descriptorIndex := buildYiraDescriptorIndex(t)
		shapeIndex, err := BuildShapeIndex(descriptorIndex)
		require.NoError(t, err)

		message := requireMessage(t, descriptorIndex, "yirapb.v1.NullableNullablePeople")
		plan, diagnostics := NewPlanner().planSingularFieldType(messageField("people", message), shapeIndex)

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindPointer, plan.Kind)
		slice := requirePointerElem(t, plan, TypeKindSlice)
		person := requirePointerElem(t, *slice.Elem, TypeKindStruct)
		assert.Equal(t, "Person", person.Ref.Name)
	})

	t.Run("flattens map shapes", func(t *testing.T) {
		descriptorIndex := buildYiraDescriptorIndex(t)
		shapeIndex, err := BuildShapeIndex(descriptorIndex)
		require.NoError(t, err)

		message := requireMessage(t, descriptorIndex, "yirapb.v1.TicketsByPeople")
		plan, diagnostics := NewPlanner().planSingularFieldType(messageField("tickets_by_people", message), shapeIndex)

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindMap, plan.Kind)
		assert.Equal(t, TypeKindStruct, plan.Key.Kind)
		assert.Equal(t, "Person", plan.Key.Ref.Name)
		assert.Equal(t, TypeKindSlice, plan.Value.Kind)
		assert.Equal(t, TypeKindPointer, plan.Value.Elem.Kind)
	})

	t.Run("maps well known types", func(t *testing.T) {
		planner := NewPlanner()
		shapeIndex := &ShapeIndex{}

		timestamp, diagnostics := planner.planSingularFieldType(messageField("created_at", &ProtoMessage{FullName: timestampFullName}), shapeIndex)
		require.Empty(t, diagnostics)
		assert.Equal(t, TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: "time", Name: "Time"}}, timestamp)

		stringValue, diagnostics := planner.planSingularFieldType(messageField("name", &ProtoMessage{FullName: "google.protobuf.StringValue"}), shapeIndex)
		require.Empty(t, diagnostics)
		elem := requirePointerElem(t, stringValue, TypeKindScalar)
		assert.Equal(t, ScalarKindString, elem.Scalar)
	})
}

func TestPlannerPlanStructComments(t *testing.T) {
	t.Run("uses explicit message option comment", func(t *testing.T) {
		message := messageForCommentTest("Person", "User", "Person comes from protobuf.")
		message.Options.SetComment("User comes from options.")

		plan, diagnostics, ok := NewPlanner().planStruct(message, &ShapeIndex{})

		require.True(t, ok)
		require.Empty(t, diagnostics)
		assert.Equal(t, "User comes from options.", plan.Comment)
	})

	t.Run("rewrites matching message source comment", func(t *testing.T) {
		message := messageForCommentTest("Person", "User", "Person comes from protobuf.")
		message.Options.SetName("Account")

		plan, diagnostics, ok := NewPlanner().planStruct(message, &ShapeIndex{})

		require.True(t, ok)
		require.Empty(t, diagnostics)
		assert.Equal(t, "Account comes from protobuf.", plan.Comment)
	})

	t.Run("ignores non-matching message source comment", func(t *testing.T) {
		message := messageForCommentTest("Person", "Person", "A user account.")

		plan, diagnostics, ok := NewPlanner().planStruct(message, &ShapeIndex{})

		require.True(t, ok)
		require.Empty(t, diagnostics)
		assert.Empty(t, plan.Comment)
	})

	t.Run("rewrites matching field source comment", func(t *testing.T) {
		field := fieldForCommentTest("first_name", "FirstName", "first_name is the given name.")
		field.Options.SetName("GivenName")

		plan, diagnostics, ok := NewPlanner().planField(field, &ShapeIndex{})

		require.True(t, ok)
		require.Empty(t, diagnostics)
		assert.Equal(t, "GivenName is the given name.", plan.Comment)
	})

	t.Run("uses explicit field option comment", func(t *testing.T) {
		field := fieldForCommentTest("first_name", "FirstName", "first_name is the given name.")
		field.Options.SetComment("FirstName comes from options.")

		plan, diagnostics, ok := NewPlanner().planField(field, &ShapeIndex{})

		require.True(t, ok)
		require.Empty(t, diagnostics)
		assert.Equal(t, "FirstName comes from options.", plan.Comment)
	})
}

func TestPlannerPlanFieldDiagnostics(t *testing.T) {
	t.Run("reports unsupported ordinary oneof", func(t *testing.T) {
		field := messageWithOneof(field("value", protoreflect.StringKind)).Fields[0]
		field.FullName = "example.v1.Message.value"

		_, diagnostics, ok := NewPlanner().planField(field, &ShapeIndex{})

		assert.False(t, ok)
		require.Len(t, diagnostics, 1)
		assert.Contains(t, diagnostics[0].Message, "oneof field planning is not currently supported")
	})

	t.Run("reports conflicting json tags", func(t *testing.T) {
		field := field("title", protoreflect.StringKind)
		field.FullName = "example.v1.Message.title"
		options := &tegopb.FieldOptions{}
		options.SetTags([]*tegopb.GoStructTag{{}})
		options.GetTags()[0].SetKey("json")
		options.GetTags()[0].SetValue("title,omitempty")
		jsonTag := &tegopb.GoJsonStructTag{}
		jsonTag.SetValue("title")
		options.SetJsonTag(jsonTag)
		field.Options = options

		_, diagnostics, ok := NewPlanner().planField(field, &ShapeIndex{})

		assert.True(t, ok)
		require.Len(t, diagnostics, 1)
		assert.Contains(t, diagnostics[0].Message, "json_tag conflicts")
	})
}

func TestPlannerGoTypeValidation(t *testing.T) {
	t.Run("accepts value conversions", func(t *testing.T) {
		field := fieldWithPlannerGoType("custom", plannerGoType(
			plannerTestPkg+".CustomString",
			plannerTestPkg+".CustomStringFromProto",
			plannerTestPkg+".CustomStringToProto",
			false,
		))

		plan, diagnostics := NewPlanner().planFieldType(field, &ShapeIndex{})

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindCustom, plan.Kind)
		assert.Equal(t, GoTypeRef{ImportPath: plannerTestPkg, Name: "CustomString"}, plan.Ref)
	})

	t.Run("accepts pointer conversions and method to proto", func(t *testing.T) {
		field := fieldWithPlannerGoType("custom", plannerGoType(
			plannerTestPkg+".CustomString",
			plannerTestPkg+".CustomStringPointerFromProto",
			plannerTestPkg+".CustomString.ToProtoPointerMethod",
			true,
		))

		plan, diagnostics := NewPlanner().planFieldType(field, &ShapeIndex{})

		require.Empty(t, diagnostics)
		elem := requirePointerElem(t, plan, TypeKindCustom)
		assert.Equal(t, GoSymbolRef{ImportPath: plannerTestPkg, Receiver: "CustomString", Name: "ToProtoPointerMethod"}, elem.Custom.ToProto)
	})

	t.Run("reports invalid conversions", func(t *testing.T) {
		tests := []struct {
			name       string
			goType     *tegopb.GoType
			diagnostic string
		}{
			{
				name:       "missing refs",
				goType:     &tegopb.GoType{},
				diagnostic: "go_type ref is required",
			},
			{
				name: "unresolved ref",
				goType: plannerGoType(
					plannerTestPkg+".Missing",
					plannerTestPkg+".CustomStringFromProto",
					plannerTestPkg+".CustomStringToProto",
					false,
				),
				diagnostic: "resolve go_type ref",
			},
			{
				name: "wrong from parameter",
				goType: plannerGoType(
					plannerTestPkg+".CustomString",
					plannerTestPkg+".WrongParameter",
					plannerTestPkg+".CustomStringToProto",
					false,
				),
				diagnostic: "from_proto parameter has wrong type",
			},
			{
				name: "wrong from result",
				goType: plannerGoType(
					plannerTestPkg+".CustomString",
					plannerTestPkg+".WrongReturn",
					plannerTestPkg+".CustomStringToProto",
					false,
				),
				diagnostic: "from_proto result has wrong type",
			},
			{
				name: "wrong error result",
				goType: plannerGoType(
					plannerTestPkg+".CustomString",
					plannerTestPkg+".WrongError",
					plannerTestPkg+".CustomStringToProto",
					false,
				),
				diagnostic: "second result must be error",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				field := fieldWithPlannerGoType("custom", tt.goType)

				_, diagnostics := NewPlanner().planFieldType(field, &ShapeIndex{})

				require.NotEmpty(t, diagnostics)
				assert.True(t, HasFatalDiagnostics(diagnostics))
				assert.Contains(t, diagnostics[0].Message, tt.diagnostic)
			})
		}
	})
}

func structByProtoName(t *testing.T, file FilePlan, name protoreflect.FullName) StructPlan {
	t.Helper()

	for _, structure := range file.Structs {
		if structure.ProtoName == name {
			return structure
		}
	}

	t.Fatalf("struct %q not found", name)
	return StructPlan{}
}

func fieldPlanByProtoName(t *testing.T, structure StructPlan, name protoreflect.FullName) FieldPlan {
	t.Helper()

	for _, field := range structure.Fields {
		if field.ProtoName == name {
			return field
		}
	}

	t.Fatalf("field %q not found", name)
	return FieldPlan{}
}

func requirePointerElem(t *testing.T, plan TypePlan, kind TypeKind) TypePlan {
	t.Helper()

	require.Equal(t, TypeKindPointer, plan.Kind)
	require.NotNil(t, plan.Elem)
	require.Equal(t, kind, plan.Elem.Kind)
	return *plan.Elem
}

func fieldWithPlannerGoType(name protoreflect.Name, goType *tegopb.GoType) *ProtoField {
	field := field(name, protoreflect.StringKind)
	field.FullName = protoreflect.FullName("example.v1.Message." + name)
	options := &tegopb.FieldOptions{}
	options.SetGoType(goType)
	field.Options = options
	return field
}

func plannerGoType(ref, fromProto, toProto string, asPointer bool) *tegopb.GoType {
	goType := &tegopb.GoType{}
	goType.SetRef(ref)
	goType.SetFromProto(fromProto)
	goType.SetToProto(toProto)
	if asPointer {
		goType.SetAsPointer(true)
	}
	return goType
}

func messageForCommentTest(protoName protoreflect.Name, goName, comment string) *ProtoMessage {
	return &ProtoMessage{
		FullName: protoreflect.FullName("example.v1." + protoName),
		Name:     protoName,
		GoName:   goName,
		Desc: &protogen.Message{
			Comments: protogen.CommentSet{Leading: protogen.Comments(comment)},
		},
		Options: &tegopb.MessageOptions{},
	}
}

func fieldForCommentTest(protoName protoreflect.Name, goName, comment string) *ProtoField {
	return &ProtoField{
		FullName: protoreflect.FullName("example.v1.Person." + protoName),
		Name:     protoName,
		GoName:   goName,
		Kind:     protoreflect.StringKind,
		Desc: &protogen.Field{
			Comments: protogen.CommentSet{Leading: protogen.Comments(comment)},
		},
		Options: &tegopb.FieldOptions{},
	}
}
