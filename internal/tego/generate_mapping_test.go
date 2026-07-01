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
}
