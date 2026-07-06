package tego

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTempIdentifierBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{name: "ID", want: "id"},
		{name: "URL", want: "url"},
		{name: "APIResponse", want: "apiResponse"},
		{name: "WatcherIDs", want: "watcherIDs"},
		{name: "StructData", want: "structData"},
		{name: "map", want: "mapValue"},
		{name: "", want: "tmp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tempIdentifierBase(tt.name))
		})
	}
}

func TestIsAddressableExpr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{name: "field selector", expr: "source.ID", want: true},
		{name: "range key", expr: "membersKey", want: true},
		{name: "range item", expr: "aliasesItem", want: true},
		{name: "dereference", expr: "*source.Name", want: true},
		{name: "parenthesized selector", expr: "(source.ID)", want: true},
		{name: "scalar cast", expr: "int64(source.Version)", want: false},
		{name: "enum conversion", expr: "generatedpb.Status(source.Status)", want: false},
		{name: "function call", expr: "SomeFunc(source.Value)", want: false},
		{name: "bool literal", expr: "true", want: false},
		{name: "invalid expression", expr: "source.", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isAddressableExpr(tt.expr))
		})
	}
}

func TestBuilderPointerValue(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "&source.ID", builderPointerValue(renderedAddressableValue("source.ID")))
	assert.Equal(t, "source.Name", builderPointerValue(renderedAddressableValue("*source.Name")))
	assert.Equal(t, "new(generatedpb.Status(source.Status))", builderPointerValue(renderedNonAddressableValue("generatedpb.Status(source.Status)")))
	assert.Equal(t, "new(structpb.NullValue_NULL_VALUE)", builderPointerValue(renderedNonAddressableValue("structpb.NullValue_NULL_VALUE")))
}

func TestTempNameAllocator(t *testing.T) {
	t.Parallel()

	allocator := newTempNameAllocator("source", "target", "err")

	assert.Equal(t, "metadataValue", allocator.name("metadataValue"))
	assert.Equal(t, "metadataValue2", allocator.name("metadataValue"))
	assert.Equal(t, "metadataValue3", allocator.name("metadataValue"))
	assert.Equal(t, "source2", allocator.name("source"))
	assert.Equal(t, "source3", allocator.name("source"))
	assert.Equal(t, "tmp", allocator.name(""))
	assert.Equal(t, "tmp2", allocator.name(""))
}

func TestMappingRenderContextTempName(t *testing.T) {
	t.Parallel()

	ctx := newMappingRenderContext(nil, false, "")

	assert.NoError(t, ctx.withTempNameHint("Metadata", func() error {
		assert.Equal(t, "metadata", ctx.tempName("items"))
		assert.Equal(t, "metadataKey", ctx.tempPartName("key"))
		assert.Equal(t, "metadataValue", ctx.tempPartName("value"))
		assert.Equal(t, "metadataValue2", ctx.tempPartName("value"))
		return nil
	}))

	assert.Equal(t, "item", ctx.tempName("item"))
	assert.Equal(t, "item2", ctx.tempName("item"))

	assert.NoError(t, ctx.withTempNameHint("WatcherIDs", func() error {
		assert.Equal(t, "watcherIDs", ctx.tempName("items"))
		assert.Equal(t, "watcherIDsItem", ctx.tempPartName("item"))
		return nil
	}))

	assert.NoError(t, ctx.withTempNameHint("Reviewer", func() error {
		assert.Equal(t, "reviewer", ctx.tempName("value"))
		return nil
	}))

	assert.NoError(t, ctx.withTempNameHint("TicketsByStatusValueProto", func() error {
		assert.Equal(t, "ticketsByStatusValueItem", ctx.collectionItemName("ticketsByStatusValue"))
		assert.Equal(t, "ticketsByStatusValueProtoItem", ctx.collectionItemName("source.GetValue()"))
		return nil
	}))
}

func TestMappedCollectionPartName(t *testing.T) {
	t.Parallel()

	tegoStruct := TypePlan{Kind: TypeKindStruct, Ref: GoTypeRef{Name: "Ticket"}}
	protoStruct := pointerType(TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{Name: "Ticket"}})

	tests := []struct {
		name   string
		source string
		plan   MappingValuePlan
		want   string
	}{
		{
			name:   "tego value",
			source: "ticketsItem",
			plan: MappingValuePlan{
				Target: tegoStruct,
			},
			want: "ticketsItemTego",
		},
		{
			name:   "proto value",
			source: "ticketsItem",
			plan: MappingValuePlan{
				Target: protoStruct,
			},
			want: "ticketsItemProto",
		},
		{
			name:   "proto slice",
			source: "ticketsValue",
			plan: MappingValuePlan{
				Target: TypePlan{Kind: TypeKindSlice, Elem: &protoStruct},
			},
			want: "ticketsValueProto",
		},
		{
			name:   "custom to proto",
			source: "labelsItem",
			plan: MappingValuePlan{
				Kind:   MappingValueKindCustom,
				Source: TypePlan{Kind: TypeKindCustom, Custom: CustomGoTypePlan{ToProto: GoSymbolRef{Name: "LabelSetToProto"}}},
			},
			want: "labelsItemProto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, mappedCollectionPartName(tt.source, tt.plan))
		})
	}
}
