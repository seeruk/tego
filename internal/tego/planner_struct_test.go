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
	t.Run("flattens slice shapes", func(t *testing.T) {
		descriptorIndex := buildYiraDescriptorIndex(t)
		shapeIndex, err := BuildShapeIndex(descriptorIndex)
		require.NoError(t, err)

		message := requireMessage(t, descriptorIndex, "yirapb.v1.PersonList")
		plan, diagnostics := NewPlanner().planSingularFieldType(messageField("watchers", message), shapeIndex)

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindSlice, plan.Kind)
		assert.Equal(t, TypeKindStruct, plan.Elem.Kind)
		assert.Equal(t, "Person", plan.Elem.Ref.Name)
	})

	t.Run("flattens map shapes", func(t *testing.T) {
		descriptorIndex := buildYiraDescriptorIndex(t)
		shapeIndex, err := BuildShapeIndex(descriptorIndex)
		require.NoError(t, err)

		message := requireMessage(t, descriptorIndex, "yirapb.v1.TicketsByStatus")
		plan, diagnostics := NewPlanner().planSingularFieldType(messageField("tickets_by_status", message), shapeIndex)

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindMap, plan.Kind)
		assert.Equal(t, TypeKindEnum, plan.Key.Kind)
		assert.Equal(t, "TicketStatus", plan.Key.Ref.Name)
		assert.Equal(t, TypeKindSlice, plan.Value.Kind)
		assert.Equal(t, TypeKindStruct, plan.Value.Elem.Kind)
		assert.Equal(t, "Ticket", plan.Value.Elem.Ref.Name)
	})

	t.Run("flattens indexed map entry messages", func(t *testing.T) {
		parent, entry := plannerMapShape()
		shapeIndex := &ShapeIndex{
			Nullables: map[protoreflect.FullName]*ProtoMessage{},
			Maps:      map[protoreflect.FullName]*ProtoMessage{parent.FullName: parent},
			Slices:    map[protoreflect.FullName]*ProtoMessage{},
		}

		plan, diagnostics := NewPlanner().planSingularFieldType(messageField("entries", entry), shapeIndex)

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindMap, plan.Kind)
		assert.Equal(t, ScalarKindString, plan.Key.Scalar)
		assert.Equal(t, ScalarKindInt64, plan.Value.Scalar)
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

		empty, diagnostics := planner.planSingularFieldType(messageField("marker", &ProtoMessage{FullName: emptyFullName}), shapeIndex)
		require.Empty(t, diagnostics)
		assert.Equal(t, emptyStructType(), empty)

		descriptorIndex := buildYiraDescriptorIndex(t)
		structMessage := requireMessage(t, descriptorIndex, "google.protobuf.Struct")
		structPlan, diagnostics := planner.planSingularFieldType(messageField("data", structMessage), shapeIndex)
		require.Empty(t, diagnostics)
		assert.Equal(t, dynamicStructType(), structPlan)

		valueMessage := requireMessage(t, descriptorIndex, "google.protobuf.Value")
		valuePlan, diagnostics := planner.planSingularFieldType(messageField("value", valueMessage), shapeIndex)
		require.Empty(t, diagnostics)
		assert.Equal(t, dynamicValueType(), valuePlan)

		listValueMessage := requireMessage(t, descriptorIndex, "google.protobuf.ListValue")
		listValuePlan, diagnostics := planner.planSingularFieldType(messageField("list_value", listValueMessage), shapeIndex)
		require.Empty(t, diagnostics)
		assert.Equal(t, dynamicListValueType(), listValuePlan)

		nullValue := requireEnum(t, descriptorIndex, "google.protobuf.NullValue")
		nullField := field("null", protoreflect.EnumKind)
		nullField.Enum = nullValue

		nullPlan := planner.planEnumType(nullField)
		assert.Equal(t, TypeKindExternal, nullPlan.Kind)
		assert.Equal(t, GoTypeRef{
			ImportPath: "google.golang.org/protobuf/types/known/structpb",
			Name:       "NullValue",
		}, nullPlan.Ref)
	})

	t.Run("plans imported types by tego coverage", func(t *testing.T) {
		externalFile := testProtoFile("external.proto", false, "example.com/external/tego;externalv1")
		foreignFile := testProtoFile("foreign.proto", false, "")

		externalMessage := plannerMessage("external.v1.Owner", "Owner")
		externalMessage.File = externalFile
		foreignMessage := plannerMessage("foreign.v1.Owner", "Owner")
		foreignMessage.File = foreignFile
		foreignMessage.Desc = &protogen.Message{GoIdent: protogen.GoIdent{
			GoImportPath: "example.com/foreign/pb",
			GoName:       "Owner",
		}}

		externalEnum := plannerEnum("external.v1.Status", "Status", externalFile)
		foreignEnum := plannerEnum("foreign.v1.Status", "Status", foreignFile)
		foreignEnum.Desc = &protogen.Enum{GoIdent: protogen.GoIdent{
			GoImportPath: "example.com/foreign/pb",
			GoName:       "Status",
		}}

		externalShape := plannerMessage("external.v1.StringList", "StringList")
		externalShape.File = externalFile
		externalShape.Fields = []*ProtoField{repeatedField("values", protoreflect.StringKind)}
		shapeIndex := &ShapeIndex{
			Nullables: map[protoreflect.FullName]*ProtoMessage{},
			Maps:      map[protoreflect.FullName]*ProtoMessage{},
			Slices:    map[protoreflect.FullName]*ProtoMessage{externalShape.FullName: externalShape},
			Flattens:  map[protoreflect.FullName]*ProtoMessage{},
		}

		tests := []struct {
			name string
			plan TypePlan
			want TypePlan
		}{
			{
				name: "external tego message",
				plan: mustPlanFieldType(t, messageField("owner", externalMessage), shapeIndex),
				want: TypePlan{Kind: TypeKindStruct, Ref: GoTypeRef{
					ImportPath: "example.com/external/tego",
					Name:       "Owner",
				}},
			},
			{
				name: "foreign message",
				plan: mustPlanFieldType(t, messageField("owner", foreignMessage), shapeIndex),
				want: pointerType(TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{
					ImportPath: "example.com/foreign/pb",
					Name:       "Owner",
				}}),
			},
			{
				name: "external tego enum",
				plan: NewPlanner().planEnumType(enumField("status", externalEnum)),
				want: TypePlan{Kind: TypeKindEnum, Ref: GoTypeRef{
					ImportPath: "example.com/external/tego",
					Name:       "Status",
				}},
			},
			{
				name: "foreign enum",
				plan: NewPlanner().planEnumType(enumField("status", foreignEnum)),
				want: TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{
					ImportPath: "example.com/foreign/pb",
					Name:       "Status",
				}},
			},
			{
				name: "external tego shape",
				plan: mustPlanFieldType(t, messageField("values", externalShape), shapeIndex),
				want: TypePlan{Kind: TypeKindSlice, Elem: &TypePlan{Kind: TypeKindScalar, Scalar: ScalarKindString}},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.want, tt.plan)
			})
		}
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
	t.Run("skips oneof member fields", func(t *testing.T) {
		field := messageWithOneof(field("value", protoreflect.StringKind)).Fields[0]
		field.FullName = "example.v1.Message.value"

		_, diagnostics, ok := NewPlanner().planField(field, &ShapeIndex{})

		assert.False(t, ok)
		assert.Empty(t, diagnostics)
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

	t.Run("warns when message-level omittable nullable field cannot preserve null", func(t *testing.T) {
		parent := plannerMessage("example.v1.Message", "Message")
		setMessageFieldOptionsOmittable(parent.Options, true)
		field := nullableMessageField("person", plannerMessage("example.v1.Person", "Person"))
		field.FullName = "example.v1.Message.person"
		field.Parent = parent

		_, diagnostics, ok := NewPlanner().planField(field, &ShapeIndex{})

		assert.True(t, ok)
		require.Len(t, diagnostics, 1)
		assert.Equal(t, DiagnosticLevelWarning, diagnostics[0].Level)
		assert.Contains(t, diagnostics[0].Message, "cannot preserve null")
	})

	t.Run("warns when field-level omittable nullable field cannot preserve null", func(t *testing.T) {
		field := nullableMessageField("person", plannerMessage("example.v1.Person", "Person"))
		field.FullName = "example.v1.Message.person"
		field.Options.SetOmittable(true)

		_, diagnostics, ok := NewPlanner().planField(field, &ShapeIndex{})

		assert.True(t, ok)
		require.Len(t, diagnostics, 1)
		assert.Equal(t, DiagnosticLevelWarning, diagnostics[0].Level)
		assert.Contains(t, diagnostics[0].Message, "cannot preserve null")
	})

	t.Run("warns when scalar nullable omittable field cannot preserve null", func(t *testing.T) {
		field := nullableOmittableField("title", protoreflect.StringKind)
		field.FullName = "example.v1.Message.title"

		_, diagnostics, ok := NewPlanner().planField(field, &ShapeIndex{})

		assert.True(t, ok)
		require.Len(t, diagnostics, 1)
		assert.Equal(t, DiagnosticLevelWarning, diagnostics[0].Level)
		assert.Contains(t, diagnostics[0].Message, "cannot preserve null")
	})

	t.Run("does not warn when message-level omittable is disabled on field", func(t *testing.T) {
		parent := plannerMessage("example.v1.Message", "Message")
		setMessageFieldOptionsOmittable(parent.Options, true)
		field := nullableMessageField("person", plannerMessage("example.v1.Person", "Person"))
		field.FullName = "example.v1.Message.person"
		field.Parent = parent
		field.Options.SetOmittable(false)

		_, diagnostics, ok := NewPlanner().planField(field, &ShapeIndex{})

		assert.True(t, ok)
		assert.Empty(t, diagnostics)
	})

	t.Run("does not warn when nullable shape can preserve null", func(t *testing.T) {
		nullable := plannerMessage("example.v1.NullablePerson", "NullablePerson")
		nullable.Fields = []*ProtoField{field("value", protoreflect.StringKind), field("valid", protoreflect.BoolKind)}
		field := nullableMessageField("person", nullable)
		field.FullName = "example.v1.Message.person"
		field.Options.SetOmittable(true)
		shapeIndex := &ShapeIndex{
			Nullables: map[protoreflect.FullName]*ProtoMessage{nullable.FullName: nullable},
		}

		_, diagnostics, ok := NewPlanner().planField(field, shapeIndex)

		assert.True(t, ok)
		assert.Empty(t, diagnostics)
	})

	t.Run("does not warn for non-nullable omittable fields", func(t *testing.T) {
		field := field("title", protoreflect.StringKind)
		field.FullName = "example.v1.Message.title"
		field.Options = &tegopb.FieldOptions{}
		field.Options.SetOmittable(true)

		_, diagnostics, ok := NewPlanner().planField(field, &ShapeIndex{})

		assert.True(t, ok)
		assert.Empty(t, diagnostics)
	})

	t.Run("plans ordinary nested message refs", func(t *testing.T) {
		parent := plannerMessage("example.v1.Parent", "Parent")
		nested := plannerMessage("example.v1.Parent.Nested", "Nested")
		nested.Parent = parent

		plan, diagnostics := NewPlanner().planSingularFieldType(messageField("nested", nested), &ShapeIndex{})

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindStruct, plan.Kind)
		assert.Equal(t, "ParentNested", plan.Ref.Name)
	})

	t.Run("does not bypass malformed map entry messages", func(t *testing.T) {
		parent, entry := plannerMapShape()
		shapeIndex := &ShapeIndex{
			Nullables: map[protoreflect.FullName]*ProtoMessage{},
			Maps:      map[protoreflect.FullName]*ProtoMessage{},
			Slices:    map[protoreflect.FullName]*ProtoMessage{},
		}

		_, diagnostics := NewPlanner().planSingularFieldType(messageField("entries", entry), shapeIndex)

		require.Len(t, diagnostics, 1)
		assert.NotContains(t, shapeIndex.Maps, parent.FullName)
		assert.Contains(t, diagnostics[0].Message, "valid map shape")
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

	t.Run("accepts generic type expressions on repeated fields", func(t *testing.T) {
		field := fieldWithPlannerGoType("custom", plannerGoTypeWithArgs(
			plannerTestPkg+".Set[T]",
			map[string]string{"T": plannerTestPkg + ".CustomString"},
			plannerTestPkg+".CustomStringSetFromProto",
			plannerTestPkg+".CustomStringSetToProto",
			false,
		))
		field.Cardinality = protoreflect.Repeated

		plan, diagnostics := NewPlanner().planFieldType(field, &ShapeIndex{})

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindCustom, plan.Kind)
		assert.Equal(t, GoTypeRef{
			ImportPath: plannerTestPkg,
			Name:       "Set",
			Args:       []GoTypeRef{{ImportPath: plannerTestPkg, Name: "CustomString"}},
		}, plan.Ref)
	})

	t.Run("accepts pointer and slice generic arguments", func(t *testing.T) {
		field := fieldWithPlannerGoType("custom", plannerGoTypeWithArgs(
			plannerTestPkg+".Box[*[]*T]",
			map[string]string{"T": plannerTestPkg + ".CustomString"},
			plannerTestPkg+".CustomStringBoxFromProto",
			plannerTestPkg+".CustomStringBoxToProto",
			false,
		))

		plan, diagnostics := NewPlanner().planFieldType(field, &ShapeIndex{})

		require.Empty(t, diagnostics)
		require.Len(t, plan.Ref.Args, 1)
		arg := plan.Ref.Args[0]
		require.NotNil(t, arg.Pointer)
		require.NotNil(t, arg.Pointer.Slice)
		require.NotNil(t, arg.Pointer.Slice.Pointer)
		assert.Equal(t, "CustomString", arg.Pointer.Slice.Pointer.Name)
	})

	t.Run("accepts as pointer on generic type expressions", func(t *testing.T) {
		field := fieldWithPlannerGoType("custom", plannerGoTypeWithArgs(
			plannerTestPkg+".Set[T]",
			map[string]string{"T": plannerTestPkg + ".CustomString"},
			plannerTestPkg+".CustomStringSetPointerFromProto",
			plannerTestPkg+".CustomStringSetPointerToProto",
			true,
		))

		plan, diagnostics := NewPlanner().planFieldType(field, &ShapeIndex{})

		require.Empty(t, diagnostics)
		elem := requirePointerElem(t, plan, TypeKindCustom)
		assert.Equal(t, "Set", elem.Ref.Name)
		require.Len(t, elem.Ref.Args, 1)
		assert.Equal(t, "CustomString", elem.Ref.Args[0].Name)
	})

	t.Run("uses generic field go type as slice shape element", func(t *testing.T) {
		field := fieldWithPlannerGoType("values", plannerGoTypeWithArgs(
			plannerTestPkg+".Box[*[]*T]",
			map[string]string{"T": plannerTestPkg + ".CustomString"},
			plannerTestPkg+".CustomStringBoxFromProto",
			plannerTestPkg+".CustomStringBoxToProto",
			false,
		))
		field.Cardinality = protoreflect.Repeated
		message := messageWithFields(field)

		plan, diagnostics := NewPlanner().planSliceShape(message, &ShapeIndex{})

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindSlice, plan.Kind)
		require.NotNil(t, plan.Elem)
		assert.Equal(t, TypeKindCustom, plan.Elem.Kind)
		assert.Equal(t, "Box", plan.Elem.Ref.Name)
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
			{
				name: "missing type argument",
				goType: plannerGoTypeWithArgs(
					plannerTestPkg+".Set[T]",
					nil,
					plannerTestPkg+".CustomStringSetFromProto",
					plannerTestPkg+".CustomStringSetToProto",
					false,
				),
				diagnostic: "no type argument",
			},
			{
				name: "unused type argument",
				goType: plannerGoTypeWithArgs(
					plannerTestPkg+".CustomString",
					map[string]string{"T": plannerTestPkg + ".CustomString"},
					plannerTestPkg+".CustomStringFromProto",
					plannerTestPkg+".CustomStringToProto",
					false,
				),
				diagnostic: "unused",
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

func TestPlannerFlattenShapes(t *testing.T) {
	t.Run("uses field go type as the whole flattened value", func(t *testing.T) {
		values := fieldWithPlannerGoType("values", plannerGoTypeWithArgs(
			plannerTestPkg+".Set[T]",
			map[string]string{"T": plannerTestPkg + ".CustomString"},
			plannerTestPkg+".CustomStringSetFromProto",
			plannerTestPkg+".CustomStringSetToProto",
			false,
		))
		values.Cardinality = protoreflect.Repeated
		message := plannerMessage("example.v1.Labels", "Labels")
		message.Fields = []*ProtoField{values}

		plan, diagnostics := NewPlanner().planMessageType(messageField("labels", message), &ShapeIndex{
			Flattens: map[protoreflect.FullName]*ProtoMessage{message.FullName: message},
		})

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindCustom, plan.Kind)
		assert.Equal(t, GoTypeRef{
			ImportPath: plannerTestPkg,
			Name:       "Set",
			Args:       []GoTypeRef{{ImportPath: plannerTestPkg, Name: "CustomString"}},
		}, plan.Ref)
	})

	t.Run("flattens lone fields without go type normally", func(t *testing.T) {
		values := repeatedField("values", protoreflect.StringKind)
		message := plannerMessage("example.v1.Values", "Values")
		message.Fields = []*ProtoField{values}

		plan, diagnostics := NewPlanner().planMessageType(messageField("values", message), &ShapeIndex{
			Flattens: map[protoreflect.FullName]*ProtoMessage{message.FullName: message},
		})

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindSlice, plan.Kind)
		require.NotNil(t, plan.Elem)
		assert.Equal(t, ScalarKindString, plan.Elem.Scalar)
	})
}

func TestFlattenMessageDiagnostics(t *testing.T) {
	t.Run("warns when infer shape is explicitly set", func(t *testing.T) {
		for _, inferShape := range []bool{true, false} {
			message := plannerMessage("example.v1.Labels", "Labels")
			message.Fields = []*ProtoField{repeatedField("values", protoreflect.StringKind)}
			message.Options.SetFlatten(true)
			message.Options.SetInferShape(inferShape)

			diagnostics := flattenMessageDiagnostics(message)

			require.Len(t, diagnostics, 1)
			assert.Equal(t, DiagnosticLevelWarning, diagnostics[0].Level)
			assert.Equal(t, "infer_shape only controls automatic shape detection when flatten is not set", diagnostics[0].Message)
		}
	})

	t.Run("reports invalid flatten declarations", func(t *testing.T) {
		tests := []struct {
			name    string
			message *ProtoMessage
			want    string
		}{
			{
				name:    "no fields",
				message: plannerMessage("example.v1.Empty", "Empty"),
				want:    "exactly one field",
			},
			{
				name: "multiple fields",
				message: func() *ProtoMessage {
					message := plannerMessage("example.v1.Pair", "Pair")
					message.Fields = []*ProtoField{
						field("first", protoreflect.StringKind),
						field("second", protoreflect.StringKind),
					}
					return message
				}(),
				want: "exactly one field",
			},
			{
				name: "oneof",
				message: func() *ProtoMessage {
					message := plannerMessage("example.v1.Choice", "Choice")
					plannerOneof(message, "value", field("name", protoreflect.StringKind))
					return message
				}(),
				want: "must not declare oneofs",
			},
			{
				name: "message go type",
				message: func() *ProtoMessage {
					message := plannerMessage("example.v1.Custom", "Custom")
					message.Fields = []*ProtoField{field("value", protoreflect.StringKind)}
					message.Options.SetGoType(plannerGoType(
						plannerTestPkg+".CustomString",
						plannerTestPkg+".CustomStringFromProto",
						plannerTestPkg+".CustomStringToProto",
						false,
					))
					return message
				}(),
				want: "conflicts with message-level go_type",
			},
			{
				name: "nested enum",
				message: func() *ProtoMessage {
					message := plannerMessage("example.v1.Wrapper", "Wrapper")
					message.Fields = []*ProtoField{field("value", protoreflect.StringKind)}
					message.Enums = []*ProtoEnum{{Name: "Kind"}}
					return message
				}(),
				want: "must not declare nested enums",
			},
			{
				name: "nested message",
				message: func() *ProtoMessage {
					message := plannerMessage("example.v1.Wrapper", "Wrapper")
					message.Fields = []*ProtoField{field("value", protoreflect.StringKind)}
					message.Messages = []*ProtoMessage{{Name: "Metadata"}}
					return message
				}(),
				want: "must not declare nested messages",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.message.Options.SetFlatten(true)

				diagnostics := flattenMessageDiagnostics(tt.message)

				require.NotEmpty(t, diagnostics)
				assert.True(t, HasFatalDiagnostics(diagnostics))
				assert.Contains(t, diagnostics[0].Message, tt.want)
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

func plannerGoTypeWithArgs(ref string, args map[string]string, fromProto, toProto string, asPointer bool) *tegopb.GoType {
	goType := plannerGoType(ref, fromProto, toProto, asPointer)
	typeArgs := make(map[string]*tegopb.GoTypeArg, len(args))
	for name, typ := range args {
		arg := &tegopb.GoTypeArg{}
		arg.SetType(typ)
		typeArgs[name] = arg
	}
	goType.SetTypeArgs(typeArgs)
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

func plannerMapShape() (*ProtoMessage, *ProtoMessage) {
	parent := plannerMessage("example.v1.StringsByName", "StringsByName")
	entry := plannerMessage("example.v1.StringsByName.Map", "Map")
	entry.Parent = parent
	entry.Fields = []*ProtoField{
		field("key", protoreflect.StringKind),
		field("value", protoreflect.Int64Kind),
	}
	for _, field := range entry.Fields {
		field.Parent = entry
	}
	parent.Messages = []*ProtoMessage{entry}
	parent.Fields = []*ProtoField{repeatedMessageField("entries", entry)}
	parent.Fields[0].Parent = parent
	return parent, entry
}

func plannerMessage(fullName protoreflect.FullName, name protoreflect.Name) *ProtoMessage {
	return &ProtoMessage{
		FullName: fullName,
		Name:     name,
		Options:  &tegopb.MessageOptions{},
	}
}

func plannerEnum(fullName protoreflect.FullName, name protoreflect.Name, file *ProtoFile) *ProtoEnum {
	return &ProtoEnum{
		FullName: fullName,
		Name:     name,
		File:     file,
		Options:  &tegopb.EnumOptions{},
	}
}

func testProtoFile(path string, generate bool, goPackage string) *ProtoFile {
	options := &tegopb.FileOptions{}
	if goPackage != "" {
		options.SetGoPackage(goPackage)
	}
	return &ProtoFile{
		Path:     path,
		Generate: generate,
		Options:  options,
	}
}

func mustPlanFieldType(t *testing.T, field *ProtoField, shapeIndex *ShapeIndex) TypePlan {
	t.Helper()

	plan, diagnostics := NewPlanner().planSingularFieldType(field, shapeIndex)

	require.Empty(t, diagnostics)
	return plan
}

func plannerOneof(parent *ProtoMessage, name protoreflect.Name, fields ...*ProtoField) *ProtoOneof {
	oneof := &ProtoOneof{
		FullName: protoreflect.FullName(string(parent.FullName) + "." + string(name)),
		Name:     name,
		GoName:   goName(string(name)),
		Parent:   parent,
		Fields:   fields,
	}
	parent.Oneofs = append(parent.Oneofs, oneof)
	parent.Fields = append(parent.Fields, fields...)
	for _, field := range fields {
		field.FullName = protoreflect.FullName(string(parent.FullName) + "." + string(field.Name))
		field.Parent = parent
		field.Oneof = oneof
	}
	return oneof
}

func enumField(name protoreflect.Name, enum *ProtoEnum) *ProtoField {
	field := field(name, protoreflect.EnumKind)
	field.Enum = enum
	return field
}

func nullableOmittableField(name protoreflect.Name, kind protoreflect.Kind) *ProtoField {
	field := field(name, kind)
	field.Options = &tegopb.FieldOptions{}
	field.Options.SetNullable(true)
	field.Options.SetOmittable(true)
	return field
}

func setMessageFieldOptionsOmittable(options *tegopb.MessageOptions, omittable bool) {
	fields := &tegopb.MessageFieldsOptions{}
	fields.SetOmittable(omittable)
	options.SetFields(fields)
}
