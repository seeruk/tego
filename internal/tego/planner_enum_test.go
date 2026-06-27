package tego

import (
	"testing"

	"github.com/seeruk/tego/tegopb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestPlannerPlanEnumYiraFixture(t *testing.T) {
	index := buildYiraDescriptorIndex(t)
	enum := requireEnum(t, index, "yirapb.v1.TicketStatus")

	plan, diagnostics, ok := NewPlanner().planEnum(enum)
	require.True(t, ok)
	require.Empty(t, diagnostics)

	assert.Equal(t, protoreflect.FullName("yirapb.v1.TicketStatus"), plan.ProtoName)
	assert.Equal(t, "TicketStatus", plan.Name)
	assert.Equal(t, "TicketStatus is the current lifecycle state of a ticket.", plan.Comment)
	assert.Equal(t, EnumUnderlyingTypeUint, plan.Underlying)
	require.Len(t, plan.Constants, 4)

	open := enumConstantByProtoName(t, plan, "yirapb.v1.TICKET_STATUS_OPEN")
	assert.Equal(t, "TicketStatusOpen", open.Name)
	assert.Equal(t, "TicketStatusOpen means work can begin.", open.Comment)
	assert.Equal(t, uint(1), open.Value.Uint)

	inProgress := enumConstantByProtoName(t, plan, "yirapb.v1.TICKET_STATUS_IN_PROGRESS")
	assert.Equal(t, "TicketStatusInProgress", inProgress.Name)
	assert.Empty(t, inProgress.Comment)
	assert.Equal(t, uint(2), inProgress.Value.Uint)
}

func TestPlannerPlanEnum(t *testing.T) {
	t.Run("omits enum", func(t *testing.T) {
		enum := protoEnum("example.v1.Status", "Status")
		enum.Options = testEnumOptions(func(options *tegopb.EnumOptions) {
			options.SetOmit(true)
		})

		_, diagnostics, ok := NewPlanner().planEnum(enum)
		assert.False(t, ok)
		assert.Empty(t, diagnostics)
	})

	t.Run("omits enum value", func(t *testing.T) {
		enum := protoEnum(
			"example.v1.Status", "Status",
			protoEnumValue("example.v1.STATUS_UNSPECIFIED", "StatusUnspecified", 0, nil),
			protoEnumValue("example.v1.STATUS_OMITTED", "StatusOmitted", 1, testEnumValueOptions(func(options *tegopb.EnumValueOptions) {
				options.SetOmit(true)
			})),
		)

		plan, diagnostics, ok := NewPlanner().planEnum(enum)
		require.True(t, ok)
		require.Empty(t, diagnostics)
		require.Len(t, plan.Constants, 1)
		assert.Equal(t, protoreflect.FullName("example.v1.STATUS_UNSPECIFIED"), plan.Constants[0].ProtoName)
	})

	t.Run("defaults to uint underlying type", func(t *testing.T) {
		enum := protoEnum(
			"example.v1.Status", "Status",
			protoEnumValue("example.v1.STATUS_OPEN", "StatusOpen", 3, nil),
		)

		plan, diagnostics, ok := NewPlanner().planEnum(enum)
		require.True(t, ok)
		require.Empty(t, diagnostics)
		assert.Equal(t, EnumUnderlyingTypeUint, plan.Underlying)
		require.Len(t, plan.Constants, 1)
		assert.Equal(t, uint(3), plan.Constants[0].Value.Uint)
	})

	t.Run("plans int underlying values", func(t *testing.T) {
		enum := protoEnum(
			"example.v1.Status", "Status",
			protoEnumValue("example.v1.STATUS_OPEN", "StatusOpen", 3, nil),
			protoEnumValue("example.v1.STATUS_CLOSED", "StatusClosed", 4, testEnumValueOptions(func(options *tegopb.EnumValueOptions) {
				options.SetInt(42)
			})),
		)
		enum.Options = testEnumOptions(func(options *tegopb.EnumOptions) {
			options.SetUnderlyingType(tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_INT)
		})

		plan, diagnostics, ok := NewPlanner().planEnum(enum)
		require.True(t, ok)
		require.Empty(t, diagnostics)
		assert.Equal(t, EnumUnderlyingTypeInt, plan.Underlying)
		require.Len(t, plan.Constants, 2)
		assert.Equal(t, int(3), plan.Constants[0].Value.Int)
		assert.Equal(t, int(42), plan.Constants[1].Value.Int)
	})

	t.Run("plans string underlying values", func(t *testing.T) {
		enum := protoEnum(
			"example.v1.Status", "Status",
			protoEnumValue("example.v1.STATUS_OPEN", "StatusOpen", 1, nil),
			protoEnumValue("example.v1.STATUS_CLOSED", "StatusClosed", 2, testEnumValueOptions(func(options *tegopb.EnumValueOptions) {
				options.SetString("closed")
			})),
		)
		enum.Options = testEnumOptions(func(options *tegopb.EnumOptions) {
			options.SetUnderlyingType(tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_STRING)
		})

		plan, diagnostics, ok := NewPlanner().planEnum(enum)
		require.True(t, ok)
		require.Empty(t, diagnostics)
		assert.Equal(t, EnumUnderlyingTypeString, plan.Underlying)
		require.Len(t, plan.Constants, 2)
		assert.Equal(t, "StatusOpen", plan.Constants[0].Value.String)
		assert.Equal(t, "closed", plan.Constants[1].Value.String)
	})

	t.Run("reports mismatched explicit value overrides", func(t *testing.T) {
		tests := []struct {
			name       string
			underlying tegopb.EnumUnderlyingType
			options    *tegopb.EnumValueOptions
		}{
			{
				name:       "uint with int",
				underlying: tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_UINT,
				options: testEnumValueOptions(func(options *tegopb.EnumValueOptions) {
					options.SetInt(1)
				}),
			},
			{
				name:       "uint with string",
				underlying: tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_UINT,
				options: testEnumValueOptions(func(options *tegopb.EnumValueOptions) {
					options.SetString("open")
				}),
			},
			{
				name:       "int with uint",
				underlying: tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_INT,
				options: testEnumValueOptions(func(options *tegopb.EnumValueOptions) {
					options.SetUint(1)
				}),
			},
			{
				name:       "int with string",
				underlying: tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_INT,
				options: testEnumValueOptions(func(options *tegopb.EnumValueOptions) {
					options.SetString("open")
				}),
			},
			{
				name:       "string with uint",
				underlying: tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_STRING,
				options: testEnumValueOptions(func(options *tegopb.EnumValueOptions) {
					options.SetUint(1)
				}),
			},
			{
				name:       "string with int",
				underlying: tegopb.EnumUnderlyingType_ENUM_UNDERLYING_TYPE_STRING,
				options: testEnumValueOptions(func(options *tegopb.EnumValueOptions) {
					options.SetInt(1)
				}),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				enum := protoEnum(
					"example.v1.Status", "Status",
					protoEnumValue("example.v1.STATUS_OPEN", "StatusOpen", 1, tt.options),
				)
				enum.Options = testEnumOptions(func(options *tegopb.EnumOptions) {
					options.SetUnderlyingType(tt.underlying)
				})

				_, diagnostics, ok := NewPlanner().planEnum(enum)
				require.True(t, ok)
				require.Len(t, diagnostics, 1)
				assert.True(t, HasFatalDiagnostics(diagnostics))
				assert.Contains(t, diagnostics[0].Message, "enum value override must match")
			})
		}
	})

	t.Run("reports unknown explicit underlying type", func(t *testing.T) {
		enum := protoEnum("example.v1.Status", "Status")
		enum.Options = testEnumOptions(func(options *tegopb.EnumOptions) {
			options.SetUnderlyingType(tegopb.EnumUnderlyingType(99))
		})

		_, diagnostics, ok := NewPlanner().planEnum(enum)
		require.True(t, ok)
		require.Len(t, diagnostics, 1)
		assert.True(t, HasFatalDiagnostics(diagnostics))
		assert.Contains(t, diagnostics[0].Message, "unsupported enum underlying type")
	})
}

func TestPlannerPlanEnumComments(t *testing.T) {
	t.Run("uses explicit enum option comment", func(t *testing.T) {
		enum := protoEnum("example.v1.Status", "Status")
		enum.Desc = &protogen.Enum{Comments: protogen.CommentSet{Leading: "Status comes from protobuf."}}
		enum.Options = testEnumOptions(func(options *tegopb.EnumOptions) {
			options.SetComment("Status comes from options.")
		})

		plan, diagnostics, ok := NewPlanner().planEnum(enum)

		require.True(t, ok)
		require.Empty(t, diagnostics)
		assert.Equal(t, "Status comes from options.", plan.Comment)
	})

	t.Run("rewrites matching enum source comment", func(t *testing.T) {
		enum := protoEnum("example.v1.Status", "RenamedStatus")
		enum.Desc = &protogen.Enum{Comments: protogen.CommentSet{Leading: "Status comes from protobuf."}}
		enum.Options = testEnumOptions(func(options *tegopb.EnumOptions) {
			options.SetName("PlannedStatus")
		})

		plan, diagnostics, ok := NewPlanner().planEnum(enum)

		require.True(t, ok)
		require.Empty(t, diagnostics)
		assert.Equal(t, "PlannedStatus comes from protobuf.", plan.Comment)
	})

	t.Run("ignores non-matching enum source comment", func(t *testing.T) {
		enum := protoEnum("example.v1.Status", "Status")
		enum.Desc = &protogen.Enum{Comments: protogen.CommentSet{Leading: "Lifecycle state."}}

		plan, diagnostics, ok := NewPlanner().planEnum(enum)

		require.True(t, ok)
		require.Empty(t, diagnostics)
		assert.Empty(t, plan.Comment)
	})

	t.Run("rewrites matching enum value source comment", func(t *testing.T) {
		value := protoEnumValue("example.v1.STATUS_OPEN", "StatusOpen", 1, nil)
		value.Desc = &protogen.EnumValue{Comments: protogen.CommentSet{Leading: "STATUS_OPEN means work can begin."}}
		enum := protoEnum("example.v1.Status", "Status", value)

		plan, diagnostics, ok := NewPlanner().planEnum(enum)

		require.True(t, ok)
		require.Empty(t, diagnostics)
		open := enumConstantByProtoName(t, plan, "example.v1.STATUS_OPEN")
		assert.Equal(t, "StatusOpen means work can begin.", open.Comment)
	})
}

func enumConstantByProtoName(t *testing.T, enum EnumPlan, name protoreflect.FullName) EnumConstantPlan {
	t.Helper()

	for _, constant := range enum.Constants {
		if constant.ProtoName == name {
			return constant
		}
	}

	t.Fatalf("enum constant %q not found", name)
	return EnumConstantPlan{}
}

func protoEnum(name protoreflect.FullName, goName string, values ...*ProtoEnumValue) *ProtoEnum {
	enum := &ProtoEnum{
		FullName: name,
		Name:     name.Name(),
		GoName:   goName,
		Options:  &tegopb.EnumOptions{},
		Values:   values,
	}
	for _, value := range values {
		value.Parent = enum
	}
	return enum
}

func protoEnumValue(
	name protoreflect.FullName,
	goName string,
	number protoreflect.EnumNumber,
	options *tegopb.EnumValueOptions,
) *ProtoEnumValue {
	if options == nil {
		options = &tegopb.EnumValueOptions{}
	}
	return &ProtoEnumValue{
		FullName: name,
		Name:     name.Name(),
		GoName:   goName,
		Number:   number,
		Options:  options,
	}
}

func testEnumOptions(configure func(*tegopb.EnumOptions)) *tegopb.EnumOptions {
	options := &tegopb.EnumOptions{}
	configure(options)
	return options
}

func testEnumValueOptions(configure func(*tegopb.EnumValueOptions)) *tegopb.EnumValueOptions {
	options := &tegopb.EnumValueOptions{}
	configure(options)
	return options
}
