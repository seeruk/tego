package tego

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

const (
	generatedTestPkg = "github.com/seeruk/tego/internal/tego/testdata/generated"
)

func TestGenerate(t *testing.T) {
	t.Run("renders enums structs types comments and tags", func(t *testing.T) {
		plugin := newGeneratorTestPlugin(t)
		plan := Plan{Files: []FilePlan{generatorTestFilePlan()}}

		require.NoError(t, Generate(plugin, plan))
		content := generatedResponseContent(t, plugin)

		_, err := parser.ParseFile(token.NewFileSet(), "generated.tego.go", content, parser.ParseComments)
		require.NoError(t, err)

		assertCommentLinesFit(t, content)
		goldie.New(t, goldie.WithFixtureDir("testdata/golden")).
			Assert(t, "generate_rendered_tego_go", []byte(content))
	})

	t.Run("renders nullable scalar and enum fields through proto presence", func(t *testing.T) {
		plugin := newGeneratorTestPlugin(t)
		plan := Plan{Files: []FilePlan{nullablePresenceTestFilePlan()}}

		require.NoError(t, Generate(plugin, plan))
		content := generatedResponseContent(t, plugin)

		_, err := parser.ParseFile(token.NewFileSet(), "generated.tego.go", content, parser.ParseComments)
		require.NoError(t, err)

		assert.Contains(t, content, "if source.HasName() {")
		assert.Contains(t, content, "name = new(source.GetName())")
		assert.Contains(t, content, "target.Name = name")
		assert.Contains(t, content, "if source.HasStatus() {")
		assert.Contains(t, content, "status = new(UintStatus(source.GetStatus()))")
		assert.Contains(t, content, "target.Status = status")
		assert.Contains(t, content, "if source.Name != nil {")
		assert.Contains(t, content, "target.SetName(*source.Name)")
		assert.Contains(t, content, "if source.Status != nil {")
		assert.Contains(t, content, "target.SetStatus(generatedpb.UintStatus(*source.Status))")
	})

	t.Run("renders external tego struct mapping calls with package qualifiers", func(t *testing.T) {
		plugin := newGeneratorTestPlugin(t)
		plan := Plan{Files: []FilePlan{externalTegoMappingTestFilePlan()}}

		require.NoError(t, Generate(plugin, plan))
		content := generatedResponseContent(t, plugin)

		_, err := parser.ParseFile(token.NewFileSet(), "generated.tego.go", content, parser.ParseComments)
		require.NoError(t, err)

		assert.Contains(t, content, "target.Owner = external.OwnerFromProto(source.GetOwner())")
		assert.Contains(t, content, "target.SetOwner(external.OwnerToProto(source.Owner))")
	})

	t.Run("renders service interfaces and rpc adapters", func(t *testing.T) {
		plugin := newGeneratorTestPlugin(t)
		plan := Plan{Files: []FilePlan{serviceInterfaceTestFilePlan()}}

		require.NoError(t, Generate(plugin, plan))
		content := generatedResponseContent(t, plugin)

		_, err := parser.ParseFile(token.NewFileSet(), "generated.tego.go", content, parser.ParseComments)
		require.NoError(t, err)

		assertCommentLinesFit(t, content)
		goldie.New(t, goldie.WithFixtureDir("testdata/golden")).
			Assert(t, "generate_service_rpc_tego_go", []byte(content))
	})

	t.Run("skips rpc code when rpc generation is disabled", func(t *testing.T) {
		plugin := newGeneratorTestPlugin(t)
		plan := Plan{Files: []FilePlan{serviceInterfaceTestFilePlan()}}

		require.NoError(t, Generate(plugin, plan, WithRPCGeneration(RPCOptions{})))
		content := generatedResponseContent(t, plugin)

		_, err := parser.ParseFile(token.NewFileSet(), "generated.tego.go", content, parser.ParseComments)
		require.NoError(t, err)
		assert.NotContains(t, content, "type TicketService interface")
		assert.NotContains(t, content, "RegisterTicketServiceGRPCServer")
	})

	t.Run("renders grpc-only rpc output", func(t *testing.T) {
		plugin := newGeneratorTestPlugin(t)
		plan := Plan{Files: []FilePlan{serviceInterfaceTestFilePlan()}}

		require.NoError(t, Generate(plugin, plan, WithRPCGeneration(RPCOptions{GRPC: true})))
		content := generatedResponseContent(t, plugin)

		assert.Contains(t, content, "RegisterTicketServiceGRPCServer")
		assert.NotContains(t, content, "NewTicketServiceConnectHandler")
	})

	t.Run("renders connect-only rpc output", func(t *testing.T) {
		plugin := newGeneratorTestPlugin(t)
		plan := Plan{Files: []FilePlan{serviceInterfaceTestFilePlan()}}

		require.NoError(t, Generate(plugin, plan, WithRPCGeneration(RPCOptions{Connect: true})))
		content := generatedResponseContent(t, plugin)

		assert.Contains(t, content, "NewTicketServiceConnectHandler")
		assert.NotContains(t, content, "RegisterTicketServiceGRPCServer")
	})

	t.Run("blocks fatal diagnostics", func(t *testing.T) {
		plugin := newGeneratorTestPlugin(t)
		plan := Plan{Files: []FilePlan{{
			ProtoPath: "broken.proto",
			Diagnostics: []Diagnostic{{
				Level:   DiagnosticLevelFatal,
				Path:    "broken.proto",
				Message: "something broke",
			}},
		}}}

		err := Generate(plugin, plan)

		require.Error(t, err)
		assert.EqualError(t, err, "plan contains fatal diagnostics")
		assert.Empty(t, plugin.Response().GetFile())
	})

	t.Run("returns shape slice element type errors", func(t *testing.T) {
		plugin := newGeneratorTestPlugin(t)
		stringType := TypePlan{Kind: TypeKindScalar, Scalar: ScalarKindString}
		wrapperType := TypePlan{Kind: TypeKindPointer, Elem: &TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Strings"}}}
		plan := Plan{Files: []FilePlan{{
			ProtoPath: "generated.proto",
			Output:    FileOutputPlan{GeneratorPath: generatedTestPkg + "/generated.tego.go"},
			Package:   PackageRef{ImportPath: generatedTestPkg, Name: "generated"},
			Mappings: []MappingPlan{{
				ProtoName: "generated.v1.Person",
				Name:      "Person",
				FromProto: MappingFunctionPlan{
					Name:   "PersonFromProto",
					Source: TypePlan{Kind: TypeKindPointer, Elem: &TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person"}}},
					Target: TypePlan{Kind: TypeKindStruct, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "Person"}},
				},
				ToProto: MappingFunctionPlan{
					Name:         "PersonToProto",
					ReceiverName: "p",
					Source:       TypePlan{Kind: TypeKindStruct, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "Person"}},
					Target:       TypePlan{Kind: TypeKindPointer, Elem: &TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person"}}},
				},
				Fields: []FieldMappingPlan{{
					ProtoName: "generated.v1.Person.values",
					Name:      "Values",
					Proto:     MappingFieldAccessPlan{Getter: "GetValues"},
					FromProto: MappingValuePlan{
						Kind:   MappingValueKindSlice,
						Source: wrapperType,
						Target: TypePlan{Kind: TypeKindSlice, Elem: &stringType},
						Access: MappingAccessPlan{Field: MappingFieldAccessPlan{Getter: "GetValues"}},
						Elem: &MappingValuePlan{
							Kind:   MappingValueKindDirect,
							Source: TypePlan{Kind: TypeKind(999)},
							Target: stringType,
						},
					},
				}},
			}},
		}}}

		err := Generate(plugin, plan)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "shape slice source element")
		assert.NotContains(t, err.Error(), "any")
	})
}

func externalTegoMappingTestFilePlan() FilePlan {
	protoOwnerType := pointerType(TypePlan{
		Kind: TypeKindExternal,
		Ref:  GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Owner"},
	})
	ownerType := TypePlan{
		Kind: TypeKindStruct,
		Ref:  GoTypeRef{ImportPath: generatedTestPkg + "/external", Name: "Owner"},
	}
	personType := TypePlan{Kind: TypeKindStruct, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "Person"}}
	protoPersonType := pointerType(TypePlan{
		Kind: TypeKindExternal,
		Ref:  GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person"},
	})

	return FilePlan{
		ProtoPath: "generated.proto",
		Output:    FileOutputPlan{GeneratorPath: generatedTestPkg + "/generated.tego.go"},
		Package:   PackageRef{ImportPath: generatedTestPkg, Name: "generated"},
		Structs: []StructPlan{{
			Name:   "Person",
			Fields: []FieldPlan{{Name: "Owner", Type: ownerType}},
		}},
		Mappings: []MappingPlan{{
			ProtoName: "generated.v1.Person",
			Name:      "Person",
			FromProto: MappingFunctionPlan{
				Name:   "PersonFromProto",
				Source: protoPersonType,
				Target: personType,
			},
			ToProto: MappingFunctionPlan{
				Name:         "PersonToProto",
				ReceiverName: "p",
				Source:       personType,
				Target:       protoPersonType,
			},
			Fields: []FieldMappingPlan{{
				Name:  "Owner",
				Proto: MappingFieldAccessPlan{Name: "Owner", Getter: "GetOwner", Setter: "SetOwner"},
				FromProto: MappingValuePlan{
					Kind:   MappingValueKindStruct,
					Source: protoOwnerType,
					Target: ownerType,
					Struct: &MappingRefPlan{
						Name:   "OwnerFromProto",
						Ref:    GoSymbolRef{ImportPath: generatedTestPkg + "/external", Name: "OwnerFromProto"},
						Source: protoOwnerType,
						Target: ownerType,
					},
				},
				ToProto: MappingValuePlan{
					Kind:   MappingValueKindStruct,
					Source: ownerType,
					Target: protoOwnerType,
					Struct: &MappingRefPlan{
						Name:   "OwnerToProto",
						Ref:    GoSymbolRef{ImportPath: generatedTestPkg + "/external", Name: "OwnerToProto"},
						Source: ownerType,
						Target: protoOwnerType,
					},
				},
			}},
		}},
	}
}

func serviceInterfaceTestFilePlan() FilePlan {
	typeRef := func(name string) TypePlan {
		return TypePlan{Kind: TypeKindStruct, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: name}}
	}
	protoRef := func(name string) TypePlan {
		return pointerType(TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: name}})
	}
	message := func(name string) ServiceMessagePlan {
		typ := typeRef(name)
		proto := protoRef(name)
		return ServiceMessagePlan{
			Type:      typ,
			ProtoType: proto,
			FromProto: MappingValuePlan{
				Kind:     MappingValueKindStruct,
				Source:   proto,
				Target:   typ,
				CanError: true,
				Struct:   &MappingRefPlan{Name: name + "FromProto", Source: proto, Target: typ},
			},
			ToProto: MappingValuePlan{
				Kind:     MappingValueKindStruct,
				Source:   typ,
				Target:   proto,
				CanError: true,
				Struct:   &MappingRefPlan{Name: name + "ToProto", Source: typ, Target: proto},
			},
		}
	}
	method := func(name string, streamType ServiceStreamType, request, response ServiceMessagePlan) ServiceMethodPlan {
		return ServiceMethodPlan{
			ProtoGoName: name,
			Name:        name,
			StreamType:  streamType,
			Request:     request,
			Response:    response,
		}
	}

	getTicketRequest := message("GetTicketRequest")
	getTicketResponse := message("GetTicketResponse")
	watchTicketEventsRequest := message("WatchTicketEventsRequest")
	ticketEvent := message("TicketEvent")
	importTicketEventsResponse := message("ImportTicketEventsResponse")

	return FilePlan{
		ProtoPath: "generated.proto",
		Output:    FileOutputPlan{GeneratorPath: generatedTestPkg + "/generated.tego.go"},
		Package:   PackageRef{ImportPath: generatedTestPkg, Name: "generated"},
		Services: []ServicePlan{{
			ProtoRef:              GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "TicketService"},
			ConnectRef:            GoTypeRef{ImportPath: generatedTestPkg + "pb/generatedpbconnect", Name: "TicketService"},
			Name:                  "TicketService",
			UnimplementedName:     "UnimplementedTicketService",
			GRPCAdapterName:       "TicketServiceGRPCAdapter",
			GRPCServerName:        "ticketServiceGRPCServer",
			GRPCClientName:        "ticketServiceGRPCClient",
			GRPCRegisterName:      "RegisterTicketServiceGRPCServer",
			GRPCNewServerName:     "NewTicketServiceGRPCServer",
			GRPCNewClientName:     "NewTicketServiceGRPCClient",
			ConnectAdapterName:    "TicketServiceConnectAdapter",
			ConnectHandlerName:    "ticketServiceConnectHandler",
			ConnectClientName:     "ticketServiceConnectClient",
			ConnectNewHandlerName: "NewTicketServiceConnectHandler",
			ConnectNewClientName:  "NewTicketServiceConnectClient",
			Methods: []ServiceMethodPlan{
				method("GetTicket", ServiceStreamTypeUnary, getTicketRequest, getTicketResponse),
				method("WatchTicketEvents", ServiceStreamTypeServerStreaming, watchTicketEventsRequest, ticketEvent),
				method("ImportTicketEvents", ServiceStreamTypeClientStreaming, ticketEvent, importTicketEventsResponse),
				method("SyncTicketEvents", ServiceStreamTypeBidiStreaming, ticketEvent, ticketEvent),
			},
		}},
	}
}

func nullablePresenceTestFilePlan() FilePlan {
	stringType := TypePlan{Kind: TypeKindScalar, Scalar: ScalarKindString}
	statusType := TypePlan{Kind: TypeKindEnum, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "UintStatus"}}
	protoStatusType := TypePlan{Kind: TypeKindEnum, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "UintStatus"}}
	personType := TypePlan{Kind: TypeKindStruct, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "Person"}}
	protoPersonType := TypePlan{
		Kind: TypeKindPointer,
		Elem: &TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person"}},
	}

	return FilePlan{
		ProtoPath: "generated.proto",
		Output:    FileOutputPlan{GeneratorPath: generatedTestPkg + "/generated.tego.go"},
		Package:   PackageRef{ImportPath: generatedTestPkg, Name: "generated"},
		Enums: []EnumPlan{{
			Name:       "UintStatus",
			Underlying: EnumUnderlyingTypeUint,
			Constants:  []EnumConstantPlan{{Name: "UintStatusUnspecified", Value: EnumConstantValue{Uint: 0}}},
		}},
		Structs: []StructPlan{{
			Name: "Person",
			Fields: []FieldPlan{
				{Name: "Name", Type: TypePlan{Kind: TypeKindPointer, Elem: &stringType}},
				{Name: "Status", Type: TypePlan{Kind: TypeKindPointer, Elem: &statusType}},
			},
		}},
		Mappings: []MappingPlan{{
			ProtoName: "generated.v1.Person",
			Name:      "Person",
			FromProto: MappingFunctionPlan{
				Name:   "PersonFromProto",
				Source: protoPersonType,
				Target: personType,
			},
			ToProto: MappingFunctionPlan{
				Name:         "PersonToProto",
				ReceiverName: "p",
				Source:       personType,
				Target:       protoPersonType,
			},
			Fields: []FieldMappingPlan{
				{
					Name:  "Name",
					Proto: MappingFieldAccessPlan{Name: "Name", Getter: "GetName", Setter: "SetName", Has: "HasName"},
					FromProto: MappingValuePlan{
						Kind:   MappingValueKindNullable,
						Source: stringType,
						Target: TypePlan{Kind: TypeKindPointer, Elem: &stringType},
						Access: MappingAccessPlan{Field: MappingFieldAccessPlan{Name: "Name", Getter: "GetName", Has: "HasName"}},
						Elem:   &MappingValuePlan{Kind: MappingValueKindDirect, Source: stringType, Target: stringType},
					},
					ToProto: MappingValuePlan{
						Kind:   MappingValueKindNullable,
						Source: TypePlan{Kind: TypeKindPointer, Elem: &stringType},
						Target: stringType,
						Elem:   &MappingValuePlan{Kind: MappingValueKindDirect, Source: stringType, Target: stringType},
					},
				},
				{
					Name:  "Status",
					Proto: MappingFieldAccessPlan{Name: "Status", Getter: "GetStatus", Setter: "SetStatus", Has: "HasStatus"},
					FromProto: MappingValuePlan{
						Kind:   MappingValueKindNullable,
						Source: protoStatusType,
						Target: TypePlan{Kind: TypeKindPointer, Elem: &statusType},
						Access: MappingAccessPlan{Field: MappingFieldAccessPlan{Name: "Status", Getter: "GetStatus", Has: "HasStatus"}},
						Elem:   &MappingValuePlan{Kind: MappingValueKindEnum, Source: protoStatusType, Target: statusType},
					},
					ToProto: MappingValuePlan{
						Kind:   MappingValueKindNullable,
						Source: TypePlan{Kind: TypeKindPointer, Elem: &statusType},
						Target: protoStatusType,
						Elem:   &MappingValuePlan{Kind: MappingValueKindEnum, Source: statusType, Target: protoStatusType},
					},
				},
			},
		}},
	}
}

func generatorTestFilePlan() FilePlan {
	stringType := TypePlan{Kind: TypeKindScalar, Scalar: ScalarKindString}
	int64Type := TypePlan{Kind: TypeKindScalar, Scalar: ScalarKindInt64}
	personType := TypePlan{Kind: TypeKindStruct, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "Person"}}
	personPointer := TypePlan{Kind: TypeKindPointer, Elem: &personType}
	labelSetType := TypePlan{
		Kind: TypeKindCustom,
		Ref: GoTypeRef{
			ImportPath: plannerTestPkg,
			Name:       "Set",
			Args:       []GoTypeRef{{ImportPath: plannerTestPkg, Name: "CustomString"}},
		},
	}
	return FilePlan{
		ProtoPath: "generated.proto",
		Output: FileOutputPlan{
			GeneratorPath: generatedTestPkg + "/generated.tego.go",
		},
		Package: PackageRef{ImportPath: generatedTestPkg, Name: "generated"},
		Enums: []EnumPlan{
			{
				Name:       "UintStatus",
				Comment:    "UintStatus has enough words to prove comment wrapping keeps the emitted comment line under the configured maximum width.",
				Underlying: EnumUnderlyingTypeUint,
				Constants: []EnumConstantPlan{
					{Name: "UintStatusUnspecified", Value: EnumConstantValue{Uint: 0}},
					{Name: "UintStatusOpen", Comment: "UintStatusOpen is available.", Value: EnumConstantValue{Uint: 1}},
				},
			},
			{
				Name:       "IntStatus",
				Underlying: EnumUnderlyingTypeInt,
				Constants:  []EnumConstantPlan{{Name: "IntStatusNegative", Value: EnumConstantValue{Int: -2}}},
			},
			{
				Name:       "StringStatus",
				Underlying: EnumUnderlyingTypeString,
				Constants:  []EnumConstantPlan{{Name: "StringStatusOpen", Value: EnumConstantValue{String: "open"}}},
			},
		},
		Oneofs: []OneofPlan{
			{
				Name:         "PersonValue",
				Comment:      "PersonValue selects one possible generated value for a person.",
				MarkerMethod: "isPersonValue",
				Variants: []OneofVariantPlan{
					{
						Name:      "PersonName",
						FieldName: "Name",
						Comment:   "PersonName stores a plain scalar value.",
						Type:      stringType,
					},
					{
						Name:      "PersonBestFriend",
						FieldName: "BestFriend",
						Type:      personType,
					},
					{
						Name:      "PersonDescriptionValue",
						FieldName: "Description",
						Type: TypePlan{
							Kind: TypeKindPointer,
							Elem: &TypePlan{Kind: TypeKindCustom, Ref: GoTypeRef{ImportPath: plannerTestPkg, Name: "Description"}},
						},
					},
				},
			},
		},
		Structs: []StructPlan{
			{
				Name:    "Person",
				Comment: "Person stores deliberately verbose generated documentation that should wrap without any full comment line exceeding one hundred characters.",
				Fields: []FieldPlan{
					{
						Name:    "ID",
						Comment: "ID has a comment that is intentionally long enough to wrap while accounting for field indentation and comment syntax.",
						Type:    stringType,
						Tags:    []StructTagPlan{{Key: "json", Value: "id,omitempty"}},
					},
					{Name: "Value", Type: TypePlan{Kind: TypeKindOneof, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "PersonValue"}}},
					{Name: "Raw", Type: TypePlan{Kind: TypeKindScalar, Scalar: ScalarKindBytes}},
					{Name: "Status", Type: TypePlan{Kind: TypeKindEnum, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "UintStatus"}}},
					{Name: "CreatedAt", Type: TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: "time", Name: "Time"}}},
					{Name: "Description", Type: TypePlan{
						Kind: TypeKindPointer,
						Elem: &TypePlan{Kind: TypeKindCustom, Ref: GoTypeRef{ImportPath: plannerTestPkg, Name: "Description"}},
					}},
					{Name: "Labels", Type: labelSetType},
					{Name: "Box", Type: TypePlan{
						Kind: TypeKindCustom,
						Ref: GoTypeRef{
							ImportPath: plannerTestPkg,
							Name:       "Box",
							Args: []GoTypeRef{{
								Pointer: &GoTypeRef{Slice: &GoTypeRef{Pointer: &GoTypeRef{
									ImportPath: plannerTestPkg,
									Name:       "CustomString",
								}}},
							}},
						},
					}},
					{Name: "LabelSets", Type: TypePlan{Kind: TypeKindSlice, Elem: &labelSetType}},
					{Name: "LabelsByName", Type: TypePlan{Kind: TypeKindMap, Key: &stringType, Value: &labelSetType}},
					{Name: "OptionalLabels", Type: TypePlan{Kind: TypeKindOmittable, Elem: &labelSetType}},
					{Name: "Friends", Type: TypePlan{Kind: TypeKindSlice, Elem: &personPointer}},
					{Name: "Metadata", Type: TypePlan{Kind: TypeKindMap, Key: &stringType, Value: &stringType}},
					{Name: "Owners", Type: TypePlan{Kind: TypeKindMap, Key: &stringType, Value: &personType}},
					{Name: "OptionalName", Type: TypePlan{Kind: TypeKindOmittable, Elem: &stringType}},
				},
			},
		},
		Mappings: []MappingPlan{
			{
				ProtoName: "generated.v1.Person",
				Name:      "Person",
				ProtoRef:  GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person"},
				Type:      personType,
				FromProto: MappingFunctionPlan{
					Name:     "PersonFromProto",
					Source:   TypePlan{Kind: TypeKindPointer, Elem: &TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person"}}},
					Target:   personType,
					CanError: true,
				},
				ToProto: MappingFunctionPlan{
					Name:         "PersonToProto",
					ReceiverName: "p",
					Source:       personType,
					Target:       TypePlan{Kind: TypeKindPointer, Elem: &TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person"}}},
					CanError:     true,
				},
				Fields: []FieldMappingPlan{
					{
						Name:  "ID",
						Proto: MappingFieldAccessPlan{Name: "Id", Getter: "GetId", Setter: "SetId", Has: "HasId"},
						FromProto: MappingValuePlan{
							Kind:   MappingValueKindDirect,
							Source: stringType,
							Target: stringType,
						},
						ToProto: MappingValuePlan{
							Kind:   MappingValueKindDirect,
							Source: stringType,
							Target: stringType,
						},
					},
					{
						Name: "Value",
						FromProto: MappingValuePlan{
							Kind:     MappingValueKindOneof,
							Source:   TypePlan{Kind: TypeKindPointer, Elem: &TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person"}}},
							Target:   TypePlan{Kind: TypeKindOneof, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "PersonValue"}},
							CanError: true,
							Oneof:    generatorTestOneofMapping(true, stringType, personType),
						},
						ToProto: MappingValuePlan{
							Kind:     MappingValueKindOneof,
							Source:   TypePlan{Kind: TypeKindOneof, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "PersonValue"}},
							Target:   TypePlan{Kind: TypeKindPointer, Elem: &TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person"}}},
							CanError: true,
							Oneof:    generatorTestOneofMapping(false, stringType, personType),
						},
					},
					{
						Name:  "Version",
						Proto: MappingFieldAccessPlan{Name: "Version", Getter: "GetVersion", Setter: "SetVersion", Has: "HasVersion"},
						FromProto: MappingValuePlan{
							Kind:   MappingValueKindScalarCast,
							Source: int64Type,
							Target: int64Type,
							Cast:   &MappingCastPlan{Source: int64Type, Target: int64Type},
						},
						ToProto: MappingValuePlan{
							Kind:   MappingValueKindScalarCast,
							Source: int64Type,
							Target: int64Type,
							Cast:   &MappingCastPlan{Source: int64Type, Target: int64Type},
						},
					},
					{
						Name:  "Status",
						Proto: MappingFieldAccessPlan{Name: "Status", Getter: "GetStatus", Setter: "SetStatus", Has: "HasStatus"},
						FromProto: MappingValuePlan{
							Kind:   MappingValueKindEnum,
							Source: TypePlan{Kind: TypeKindEnum, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "UintStatus"}},
							Target: TypePlan{Kind: TypeKindEnum, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "UintStatus"}},
						},
						ToProto: MappingValuePlan{
							Kind:   MappingValueKindEnum,
							Source: TypePlan{Kind: TypeKindEnum, Ref: GoTypeRef{ImportPath: generatedTestPkg, Name: "UintStatus"}},
							Target: TypePlan{Kind: TypeKindEnum, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "UintStatus"}},
						},
					},
					{
						Name:  "Description",
						Proto: MappingFieldAccessPlan{Name: "Description", Getter: "GetDescription", Setter: "SetDescription", Has: "HasDescription"},
						FromProto: MappingValuePlan{
							Kind:     MappingValueKindCustom,
							Source:   stringType,
							Target:   TypePlan{Kind: TypeKindPointer, Elem: &TypePlan{Kind: TypeKindCustom, Ref: GoTypeRef{ImportPath: plannerTestPkg, Name: "Description"}}},
							CanError: true,
							Custom: &CustomGoTypePlan{
								FromProto:         GoSymbolRef{ImportPath: plannerTestPkg, Name: "DescriptionFromProto"},
								FromProtoCanError: true,
							},
						},
						ToProto: MappingValuePlan{
							Kind:     MappingValueKindCustom,
							Source:   TypePlan{Kind: TypeKindPointer, Elem: &TypePlan{Kind: TypeKindCustom, Ref: GoTypeRef{ImportPath: plannerTestPkg, Name: "Description"}}},
							Target:   stringType,
							CanError: true,
							Custom: &CustomGoTypePlan{
								ToProto:         GoSymbolRef{ImportPath: plannerTestPkg, Name: "DescriptionToProto"},
								ToProtoCanError: true,
							},
						},
					},
					{
						Name:  "WatcherIDs",
						Proto: MappingFieldAccessPlan{Name: "WatcherIds", Getter: "GetWatcherIds", Setter: "SetWatcherIds", Has: "HasWatcherIds"},
						FromProto: MappingValuePlan{
							Kind:   MappingValueKindSlice,
							Source: TypePlan{Kind: TypeKindSlice, Elem: &stringType},
							Target: TypePlan{Kind: TypeKindSlice, Elem: &stringType},
							Elem:   &MappingValuePlan{Kind: MappingValueKindDirect, Source: stringType, Target: stringType},
						},
						ToProto: MappingValuePlan{
							Kind:   MappingValueKindSlice,
							Source: TypePlan{Kind: TypeKindSlice, Elem: &stringType},
							Target: TypePlan{Kind: TypeKindSlice, Elem: &stringType},
							Elem:   &MappingValuePlan{Kind: MappingValueKindDirect, Source: stringType, Target: stringType},
						},
					},
					{
						Name:  "Metadata",
						Proto: MappingFieldAccessPlan{Name: "Metadata", Getter: "GetMetadata", Setter: "SetMetadata", Has: "HasMetadata"},
						FromProto: MappingValuePlan{
							Kind:   MappingValueKindMap,
							Source: TypePlan{Kind: TypeKindMap, Key: &stringType, Value: &stringType},
							Target: TypePlan{Kind: TypeKindMap, Key: &stringType, Value: &stringType},
							Key:    &MappingValuePlan{Kind: MappingValueKindDirect, Source: stringType, Target: stringType},
							Value:  &MappingValuePlan{Kind: MappingValueKindDirect, Source: stringType, Target: stringType},
						},
						ToProto: MappingValuePlan{
							Kind:   MappingValueKindMap,
							Source: TypePlan{Kind: TypeKindMap, Key: &stringType, Value: &stringType},
							Target: TypePlan{Kind: TypeKindMap, Key: &stringType, Value: &stringType},
							Key:    &MappingValuePlan{Kind: MappingValueKindDirect, Source: stringType, Target: stringType},
							Value:  &MappingValuePlan{Kind: MappingValueKindDirect, Source: stringType, Target: stringType},
						},
					},
					{
						Name:  "OptionalName",
						Proto: MappingFieldAccessPlan{Name: "OptionalName", Getter: "GetOptionalName", Setter: "SetOptionalName", Has: "HasOptionalName"},
						FromProto: MappingValuePlan{
							Kind:   MappingValueKindOmittable,
							Source: stringType,
							Target: TypePlan{Kind: TypeKindOmittable, Elem: &stringType},
							Elem:   &MappingValuePlan{Kind: MappingValueKindDirect, Source: stringType, Target: stringType},
						},
						ToProto: MappingValuePlan{
							Kind:   MappingValueKindOmittable,
							Source: TypePlan{Kind: TypeKindOmittable, Elem: &stringType},
							Target: stringType,
							Elem:   &MappingValuePlan{Kind: MappingValueKindDirect, Source: stringType, Target: stringType},
						},
					},
				},
			},
		},
	}
}

func generatorTestOneofMapping(fromProto bool, stringType TypePlan, personType TypePlan) *MappingOneofPlan {
	protoPersonType := TypePlan{
		Kind: TypeKindPointer,
		Elem: &TypePlan{Kind: TypeKindExternal, Ref: GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person"}},
	}
	descriptionType := TypePlan{
		Kind: TypeKindPointer,
		Elem: &TypePlan{Kind: TypeKindCustom, Ref: GoTypeRef{ImportPath: plannerTestPkg, Name: "Description"}},
	}

	nameValue := MappingValuePlan{Kind: MappingValueKindDirect, Source: stringType, Target: stringType}
	bestFriendValue := MappingValuePlan{
		Kind:     MappingValueKindStruct,
		Source:   protoPersonType,
		Target:   personType,
		CanError: true,
		Struct:   &MappingRefPlan{Name: "PersonFromProto", Source: protoPersonType, Target: personType},
	}
	descriptionValue := MappingValuePlan{
		Kind:     MappingValueKindCustom,
		Source:   stringType,
		Target:   descriptionType,
		CanError: true,
		Custom: &CustomGoTypePlan{
			FromProto:         GoSymbolRef{ImportPath: plannerTestPkg, Name: "DescriptionFromProto"},
			FromProtoCanError: true,
		},
	}
	if !fromProto {
		bestFriendValue = MappingValuePlan{
			Kind:     MappingValueKindStruct,
			Source:   personType,
			Target:   protoPersonType,
			CanError: true,
			Struct:   &MappingRefPlan{Name: "PersonToProto", Source: personType, Target: protoPersonType},
		}
		descriptionValue = MappingValuePlan{
			Kind:     MappingValueKindCustom,
			Source:   descriptionType,
			Target:   stringType,
			CanError: true,
			Custom: &CustomGoTypePlan{
				ToProto:         GoSymbolRef{ImportPath: plannerTestPkg, Name: "DescriptionToProto"},
				ToProtoCanError: true,
			},
		}
	}

	return &MappingOneofPlan{
		Which: "WhichValue",
		Variants: []MappingOneofVariantPlan{
			{
				ProtoName: "generated.v1.Person.name",
				Name:      "PersonName",
				FieldName: "Name",
				Proto:     MappingFieldAccessPlan{Name: "Name", Getter: "GetName", Setter: "SetName"},
				Case:      GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person_Name_case"},
				Value:     nameValue,
			},
			{
				ProtoName: "generated.v1.Person.best_friend",
				Name:      "PersonBestFriend",
				FieldName: "BestFriend",
				Proto:     MappingFieldAccessPlan{Name: "BestFriend", Getter: "GetBestFriend", Setter: "SetBestFriend"},
				Case:      GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person_BestFriend_case"},
				Value:     bestFriendValue,
			},
			{
				ProtoName: "generated.v1.Person.description_value",
				Name:      "PersonDescriptionValue",
				FieldName: "Description",
				Proto:     MappingFieldAccessPlan{Name: "DescriptionValue", Getter: "GetDescriptionValue", Setter: "SetDescriptionValue"},
				Case:      GoTypeRef{ImportPath: generatedTestPkg + "pb", Name: "Person_DescriptionValue_case"},
				Value:     descriptionValue,
			},
		},
	}
}

func newGeneratorTestPlugin(t *testing.T) *protogen.Plugin {
	t.Helper()

	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"generated.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{{
			Name:    new("generated.proto"),
			Package: new("generated.v1"),
			Syntax:  new("proto3"),
			Options: &descriptorpb.FileOptions{
				GoPackage: new(generatedTestPkg + ";generated"),
			},
		}},
	}

	plugin, err := protogen.Options{}.New(req)
	require.NoError(t, err)
	return plugin
}

func generatedResponseContent(t *testing.T, plugin *protogen.Plugin) string {
	t.Helper()

	response := plugin.Response()
	require.Empty(t, response.GetError())
	require.Len(t, response.GetFile(), 1)
	return response.GetFile()[0].GetContent()
}

func assertCommentLinesFit(t *testing.T, content string) {
	t.Helper()

	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "//") {
			assert.LessOrEqual(t, len(line), maxGeneratedLineWidth, line)
		}
	}
}
