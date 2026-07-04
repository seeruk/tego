package tego

import (
	"testing"

	"github.com/seeruk/tego/tegopb"
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

		require.NotNil(t, intPlan.Cast)
		require.NotNil(t, uintPlan.Cast)
		assert.Equal(t, MappingValueKindScalarCast, intPlan.Kind)
		assert.False(t, intPlan.Cast.ProtoTarget)
		assert.Equal(t, MappingValueKindScalarCast, uintPlan.Kind)
		assert.True(t, uintPlan.Cast.ProtoTarget)
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
		assert.Equal(t, GoSymbolRef{ImportPath: "example.com/tego", Name: "PersonFromProto"}, fromProto.Struct.Ref)
		assert.Equal(t, GoSymbolRef{ImportPath: "example.com/tego", Name: "PersonToProto"}, toProto.Struct.Ref)
	})

	t.Run("plans identical foreign proto pointers as direct mappings", func(t *testing.T) {
		foreign := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: "google.golang.org/protobuf/types/known/structpb", Name: "Struct"},
		})

		fromProto := planner.planMappingValue(foreign, foreign, mappingDirectionFromProto)
		toProto := planner.planMappingValue(foreign, foreign, mappingDirectionToProto)

		assert.Equal(t, MappingValueKindDirect, fromProto.Kind)
		assert.Equal(t, MappingValueKindDirect, toProto.Kind)
	})

	t.Run("plans structpb struct map conversions", func(t *testing.T) {
		protoStruct := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: "google.golang.org/protobuf/types/known/structpb", Name: "Struct"},
		})
		nativeStruct := dynamicStructType()

		fromProto := planner.planMappingValue(protoStruct, nativeStruct, mappingDirectionFromProto)
		toProto := planner.planMappingValue(nativeStruct, protoStruct, mappingDirectionToProto)

		require.NotNil(t, fromProto.Dynamic)
		require.NotNil(t, toProto.Dynamic)
		assert.Equal(t, MappingValueKindDynamic, fromProto.Kind)
		assert.Equal(t, MappingDynamicKindStruct, fromProto.Dynamic.Kind)
		assert.False(t, fromProto.CanError)
		assert.Equal(t, MappingValueKindDynamic, toProto.Kind)
		assert.Equal(t, MappingDynamicKindStruct, toProto.Dynamic.Kind)
		assert.True(t, toProto.CanError)

		_, ok := structpbStructMapMapping(protoStruct, nativeStruct, mappingDirectionToProto)
		assert.False(t, ok)
		_, ok = structpbStructMapMapping(nativeStruct, protoStruct, mappingDirectionFromProto)
		assert.False(t, ok)
	})

	t.Run("plans structpb value conversions", func(t *testing.T) {
		protoValue := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: structpbImportPath, Name: "Value"},
		})
		nativeValue := dynamicValueType()

		fromProto := planner.planMappingValue(protoValue, nativeValue, mappingDirectionFromProto)
		toProto := planner.planMappingValue(nativeValue, protoValue, mappingDirectionToProto)

		require.NotNil(t, fromProto.Dynamic)
		require.NotNil(t, toProto.Dynamic)
		assert.Equal(t, MappingValueKindDynamic, fromProto.Kind)
		assert.Equal(t, MappingDynamicKindValue, fromProto.Dynamic.Kind)
		assert.False(t, fromProto.CanError)
		assert.Equal(t, MappingValueKindDynamic, toProto.Kind)
		assert.Equal(t, MappingDynamicKindValue, toProto.Dynamic.Kind)
		assert.True(t, toProto.CanError)
	})

	t.Run("plans structpb list value conversions", func(t *testing.T) {
		protoList := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: structpbImportPath, Name: "ListValue"},
		})
		nativeList := dynamicListValueType()

		fromProto := planner.planMappingValue(protoList, nativeList, mappingDirectionFromProto)
		toProto := planner.planMappingValue(nativeList, protoList, mappingDirectionToProto)

		require.NotNil(t, fromProto.Dynamic)
		require.NotNil(t, toProto.Dynamic)
		assert.Equal(t, MappingValueKindDynamic, fromProto.Kind)
		assert.Equal(t, MappingDynamicKindListValue, fromProto.Dynamic.Kind)
		assert.False(t, fromProto.CanError)
		assert.Equal(t, MappingValueKindDynamic, toProto.Kind)
		assert.Equal(t, MappingDynamicKindListValue, toProto.Dynamic.Kind)
		assert.True(t, toProto.CanError)
	})

	t.Run("plans wrapper conversions through nullable fields", func(t *testing.T) {
		protoWrapper := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: wrapperspbImportPath, Name: "StringValue"},
		})
		nativeWrapper := pointerType(stringType)
		field := messageField("wrapped_string", &ProtoMessage{FullName: "google.protobuf.StringValue"})

		fromProto := planner.planFieldMappingValue(field, protoWrapper, nativeWrapper, &ShapeIndex{}, mappingDirectionFromProto)
		toProto := planner.planFieldMappingValue(field, nativeWrapper, protoWrapper, &ShapeIndex{}, mappingDirectionToProto)

		require.NotNil(t, fromProto.Elem)
		require.NotNil(t, toProto.Elem)
		assert.Equal(t, MappingValueKindNullable, fromProto.Kind)
		assert.Equal(t, MappingValueKindNullable, toProto.Kind)
		assertWellKnownMapping(t, *fromProto.Elem, MappingWellKnownKindWrapper)
		assertWellKnownMapping(t, *toProto.Elem, MappingWellKnownKindWrapper)
	})

	t.Run("plans timestamp conversions", func(t *testing.T) {
		protoTimestamp := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: timestamppbImportPath, Name: "Timestamp"},
		})
		nativeTimestamp := TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: "time", Name: "Time"}}

		fromProto := planner.planMappingValue(protoTimestamp, nativeTimestamp, mappingDirectionFromProto)
		toProto := planner.planMappingValue(nativeTimestamp, protoTimestamp, mappingDirectionToProto)

		assertWellKnownMapping(t, fromProto, MappingWellKnownKindTimestamp)
		assertWellKnownMapping(t, toProto, MappingWellKnownKindTimestamp)
	})

	t.Run("plans nullable timestamp conversions", func(t *testing.T) {
		protoTimestamp := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: timestamppbImportPath, Name: "Timestamp"},
		})
		nativeTimestamp := TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: "time", Name: "Time"}}
		nullableTimestamp := pointerType(nativeTimestamp)
		field := nullableMessageField("created_at", &ProtoMessage{FullName: timestampFullName})

		fromProto := planner.planFieldMappingValue(field, protoTimestamp, nullableTimestamp, &ShapeIndex{}, mappingDirectionFromProto)
		toProto := planner.planFieldMappingValue(field, nullableTimestamp, protoTimestamp, &ShapeIndex{}, mappingDirectionToProto)

		require.NotNil(t, fromProto.Elem)
		require.NotNil(t, toProto.Elem)
		assert.Equal(t, MappingValueKindNullable, fromProto.Kind)
		assert.Equal(t, MappingValueKindNullable, toProto.Kind)
		assertWellKnownMapping(t, *fromProto.Elem, MappingWellKnownKindTimestamp)
		assertWellKnownMapping(t, *toProto.Elem, MappingWellKnownKindTimestamp)
	})

	t.Run("plans duration conversions", func(t *testing.T) {
		protoDuration := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: durationpbImportPath, Name: "Duration"},
		})
		nativeDuration := TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: "time", Name: "Duration"}}

		fromProto := planner.planMappingValue(protoDuration, nativeDuration, mappingDirectionFromProto)
		toProto := planner.planMappingValue(nativeDuration, protoDuration, mappingDirectionToProto)

		assertWellKnownMapping(t, fromProto, MappingWellKnownKindDuration)
		assertWellKnownMapping(t, toProto, MappingWellKnownKindDuration)
	})

	t.Run("plans nullable duration conversions", func(t *testing.T) {
		protoDuration := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: durationpbImportPath, Name: "Duration"},
		})
		nativeDuration := TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: "time", Name: "Duration"}}
		nullableDuration := pointerType(nativeDuration)
		field := nullableMessageField("ttl", &ProtoMessage{FullName: durationFullName})

		fromProto := planner.planFieldMappingValue(field, protoDuration, nullableDuration, &ShapeIndex{}, mappingDirectionFromProto)
		toProto := planner.planFieldMappingValue(field, nullableDuration, protoDuration, &ShapeIndex{}, mappingDirectionToProto)

		require.NotNil(t, fromProto.Elem)
		require.NotNil(t, toProto.Elem)
		assert.Equal(t, MappingValueKindNullable, fromProto.Kind)
		assert.Equal(t, MappingValueKindNullable, toProto.Kind)
		assertWellKnownMapping(t, *fromProto.Elem, MappingWellKnownKindDuration)
		assertWellKnownMapping(t, *toProto.Elem, MappingWellKnownKindDuration)
	})

	t.Run("returns unsupported when pointer fallback cannot make progress", func(t *testing.T) {
		protoMessage := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: "example.com/external", Name: "Message"},
		})

		plan := planner.planMappingValue(stringType, protoMessage, mappingDirectionToProto)

		assert.Equal(t, MappingValueKindUnsupported, plan.Kind)
	})

	t.Run("plans nullable structpb value conversions", func(t *testing.T) {
		protoValue := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: structpbImportPath, Name: "Value"},
		})
		nativeValue := pointerType(dynamicValueType())
		field := nullableMessageField("dynamic_value", &ProtoMessage{FullName: valueFullName})

		fromProto := planner.planFieldMappingValue(field, protoValue, nativeValue, &ShapeIndex{}, mappingDirectionFromProto)
		toProto := planner.planFieldMappingValue(field, nativeValue, protoValue, &ShapeIndex{}, mappingDirectionToProto)

		require.NotNil(t, fromProto.Elem)
		require.NotNil(t, toProto.Elem)
		assert.Equal(t, MappingValueKindNullable, fromProto.Kind)
		require.NotNil(t, fromProto.Elem.Dynamic)
		assert.Equal(t, MappingValueKindDynamic, fromProto.Elem.Kind)
		assert.Equal(t, MappingDynamicKindValue, fromProto.Elem.Dynamic.Kind)
		assert.Equal(t, MappingValueKindNullable, toProto.Kind)
		require.NotNil(t, toProto.Elem.Dynamic)
		assert.Equal(t, MappingValueKindDynamic, toProto.Elem.Kind)
		assert.Equal(t, MappingDynamicKindValue, toProto.Elem.Dynamic.Kind)
	})

	t.Run("plans omittable structpb value conversions", func(t *testing.T) {
		protoValue := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: structpbImportPath, Name: "Value"},
		})
		nativeValue := dynamicValueType()
		field := messageField("dynamic_value", &ProtoMessage{FullName: valueFullName})
		options := &tegopb.FieldOptions{}
		options.SetOmittable(true)
		field.Options = options

		fromProto := planner.planFieldMappingValue(
			field,
			protoValue,
			TypePlan{Kind: TypeKindOmittable, Elem: &nativeValue},
			&ShapeIndex{},
			mappingDirectionFromProto,
		)

		require.NotNil(t, fromProto.Elem)
		assert.Equal(t, MappingValueKindOmittable, fromProto.Kind)
		require.NotNil(t, fromProto.Elem.Dynamic)
		assert.Equal(t, MappingValueKindDynamic, fromProto.Elem.Kind)
		assert.Equal(t, MappingDynamicKindValue, fromProto.Elem.Dynamic.Kind)
	})

	t.Run("plans emptypb empty struct conversions", func(t *testing.T) {
		protoEmpty := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: emptypbImportPath, Name: "Empty"},
		})
		nativeEmpty := emptyStructType()

		fromProto := planner.planMappingValue(protoEmpty, nativeEmpty, mappingDirectionFromProto)
		toProto := planner.planMappingValue(nativeEmpty, protoEmpty, mappingDirectionToProto)

		assert.Equal(t, MappingValueKindEmptyStruct, fromProto.Kind)
		assert.False(t, fromProto.CanError)
		assert.Equal(t, MappingValueKindEmptyStruct, toProto.Kind)
		assert.False(t, toProto.CanError)

		_, ok := emptypbEmptyStructMapping(protoEmpty, nativeEmpty, mappingDirectionToProto)
		assert.False(t, ok)
		_, ok = emptypbEmptyStructMapping(nativeEmpty, protoEmpty, mappingDirectionFromProto)
		assert.False(t, ok)
	})

	t.Run("plans nullable empty struct conversions", func(t *testing.T) {
		protoEmpty := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: emptypbImportPath, Name: "Empty"},
		})
		nativeEmpty := pointerType(emptyStructType())
		field := nullableMessageField("marker", &ProtoMessage{FullName: emptyFullName})

		fromProto := planner.planFieldMappingValue(field, protoEmpty, nativeEmpty, &ShapeIndex{}, mappingDirectionFromProto)
		toProto := planner.planFieldMappingValue(field, nativeEmpty, protoEmpty, &ShapeIndex{}, mappingDirectionToProto)

		require.NotNil(t, fromProto.Elem)
		require.NotNil(t, toProto.Elem)
		assert.Equal(t, MappingValueKindNullable, fromProto.Kind)
		assert.Equal(t, MappingValueKindEmptyStruct, fromProto.Elem.Kind)
		assert.Equal(t, MappingValueKindNullable, toProto.Kind)
		assert.Equal(t, MappingValueKindEmptyStruct, toProto.Elem.Kind)
	})

	t.Run("plans omittable empty struct conversions", func(t *testing.T) {
		protoEmpty := pointerType(TypePlan{
			Kind: TypeKindExternal,
			Ref:  GoTypeRef{ImportPath: emptypbImportPath, Name: "Empty"},
		})
		nativeEmpty := emptyStructType()
		field := messageField("marker", &ProtoMessage{FullName: emptyFullName})
		options := &tegopb.FieldOptions{}
		options.SetOmittable(true)
		field.Options = options

		fromProto := planner.planFieldMappingValue(
			field,
			protoEmpty,
			TypePlan{Kind: TypeKindOmittable, Elem: &nativeEmpty},
			&ShapeIndex{},
			mappingDirectionFromProto,
		)

		require.NotNil(t, fromProto.Elem)
		assert.Equal(t, MappingValueKindOmittable, fromProto.Kind)
		assert.Equal(t, MappingValueKindEmptyStruct, fromProto.Elem.Kind)
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

	t.Run("plans explicit flatten wrappers", func(t *testing.T) {
		values := fieldWithPlannerGoType("values", plannerGoTypeWithArgs(
			plannerTestPkg+".Set[T]",
			map[string]string{"T": plannerTestPkg + ".CustomString"},
			plannerTestPkg+".CustomStringSetFromProto",
			plannerTestPkg+".CustomStringSetToProto",
			false,
		))
		values.Cardinality = protoreflect.Repeated

		shape := plannerMessage("example.v1.Labels", "Labels")
		shape.GoName = "Labels"
		values.Parent = shape
		shape.Fields = []*ProtoField{values}

		field := messageField("labels", shape)
		field.FullName = "example.v1.Ticket.labels"
		shapeIndex := &ShapeIndex{
			Flattens: map[protoreflect.FullName]*ProtoMessage{shape.FullName: shape},
		}

		target, diagnostics := planner.planFieldType(field, shapeIndex)
		require.Empty(t, diagnostics)
		source := planner.planProtoFieldType(field)

		fromProto := planner.planFieldMappingValue(field, source, target, shapeIndex, mappingDirectionFromProto)
		toProto := planner.planFieldMappingValue(field, target, source, shapeIndex, mappingDirectionToProto)

		require.NotNil(t, fromProto.Elem)
		require.NotNil(t, toProto.Elem)
		assert.Equal(t, MappingValueKindFlatten, fromProto.Kind)
		assert.Equal(t, MappingValueKindCustom, fromProto.Elem.Kind)
		assert.True(t, fromProto.CanError)
		assert.Equal(t, MappingValueKindFlatten, toProto.Kind)
		assert.Equal(t, MappingValueKindCustom, toProto.Elem.Kind)
		assert.True(t, toProto.CanError)
	})
}

func assertWellKnownMapping(t *testing.T, plan MappingValuePlan, kind MappingWellKnownKind) {
	t.Helper()
	require.NotNil(t, plan.WellKnown)
	assert.Equal(t, MappingValueKindWellKnown, plan.Kind)
	assert.Equal(t, kind, plan.WellKnown.Kind)
	assert.False(t, plan.CanError)
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

func plannerScalarField(parent *ProtoMessage, name protoreflect.Name, kind protoreflect.Kind) *ProtoField {
	field := field(name, kind)
	field.FullName = protoreflect.FullName(string(parent.FullName) + "." + string(name))
	field.Parent = parent
	return field
}
