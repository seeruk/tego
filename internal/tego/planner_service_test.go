package tego

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type serviceInlineFixture struct {
	File     *ProtoFile
	Request  *ProtoMessage
	Response *ProtoMessage
	Method   *ProtoMethod
	Service  *ProtoService
}

func TestPlannerPlanServices(t *testing.T) {
	t.Run("maps stream types", func(t *testing.T) {
		tests := map[string]struct {
			client bool
			server bool
			want   ServiceStreamType
		}{
			"unary":  {want: ServiceStreamTypeUnary},
			"client": {client: true, want: ServiceStreamTypeClientStreaming},
			"server": {server: true, want: ServiceStreamTypeServerStreaming},
			"bidi":   {client: true, server: true, want: ServiceStreamTypeBidiStreaming},
		}

		for name, tt := range tests {
			t.Run(name, func(t *testing.T) {
				method := &ProtoMethod{ClientStreaming: tt.client, ServerStreaming: tt.server}

				assert.Equal(t, tt.want, serviceMethodStreamType(method))
			})
		}
	})

	t.Run("plans covered flatten shape method messages", func(t *testing.T) {
		file := protoFileWithOutput("shapes.proto", "github.com/example/shapes;shapes", "")
		labels := plannerMessage("example.v1.Labels", "Labels")
		labels.GoName = "Labels"
		labels.Options.SetFlatten(true)
		value := field("value", protoreflect.StringKind)
		value.FullName = "example.v1.Labels.value"
		value.GoName = "Value"
		value.Parent = labels
		labels.Fields = []*ProtoField{value}
		attachMessagesToFile(file, labels)
		method := plannerMethod("example.v1.LabelService.SetLabels", "SetLabels", labels, labels)

		plan, diagnostics := NewPlanner().planServiceMethod(nil, method, &ShapeIndex{
			Flattens: map[protoreflect.FullName]*ProtoMessage{labels.FullName: labels},
		}, nil)

		require.Empty(t, diagnostics)
		assert.Equal(t, TypePlan{Kind: TypeKindScalar, Scalar: ScalarKindString}, plan.Request.Type)
		assert.Equal(t, MappingValueKindFlatten, plan.Request.FromProto.Kind)
		assert.Equal(t, MappingValueKindFlatten, plan.Response.ToProto.Kind)
	})

	t.Run("passes through non covered foreign method messages", func(t *testing.T) {
		foreignFile := testProtoFile("foreign.proto", false, "")
		foreign := plannerMessage("foreign.v1.Foreign", "Foreign")
		foreign.GoName = "Foreign"
		attachMessagesToFile(foreignFile, foreign)
		method := plannerMethod("example.v1.ForeignService.UseForeign", "UseForeign", foreign, foreign)

		plan, diagnostics := NewPlanner().planServiceMethod(nil, method, &ShapeIndex{}, nil)

		require.Empty(t, diagnostics)
		assert.Equal(t, TypeKindPointer, plan.Request.Type.Kind)
		assert.Equal(t, TypeKindExternal, plan.Request.Type.Elem.Kind)
		assert.Equal(t, "Foreign", plan.Request.Type.Elem.Ref.Name)
		assert.Equal(t, MappingValueKindDirect, plan.Request.FromProto.Kind)
		assert.Equal(t, MappingValueKindDirect, plan.Response.ToProto.Kind)
	})

	t.Run("derives connect package refs from suffix", func(t *testing.T) {
		descriptorIndex := buildYiraDescriptorIndex(t)
		shapeIndex, err := BuildShapeIndex(descriptorIndex)
		require.NoError(t, err)
		tests := map[string]struct {
			suffix string
			want   string
		}{
			"custom suffix": {
				suffix: "connectgo",
				want:   "github.com/seeruk/tego/internal/tego/testdata/proto/yirapbv1/yirapbv1connectgo",
			},
			"same package": {
				want: "github.com/seeruk/tego/internal/tego/testdata/proto/yirapbv1",
			},
		}

		for name, tt := range tests {
			t.Run(name, func(t *testing.T) {
				options := RPCOptions{Connect: true, ConnectPackageSuffix: tt.suffix}
				plan, err := NewPlanner(WithRPCPlanning(options)).Plan(descriptorIndex, shapeIndex)

				require.NoError(t, err)
				service := serviceByProtoName(t, plan.Files[0], "yirapb.v1.TicketService")
				assert.Equal(t, tt.want, service.ConnectRef.ImportPath)
			})
		}
	})

	t.Run("plans default unary inline helpers", func(t *testing.T) {
		ticket := plannerMessage("example.v1.Ticket", "Ticket")
		fixture := newServiceInlineFixture(
			"GetTicket",
			"GetTicketRequest",
			[]*ProtoField{field("ticket_id", protoreflect.StringKind)},
			"GetTicketResponse",
			[]*ProtoField{messageField("ticket", ticket)},
			ticket,
		)

		plan := planInlineServiceFixture(t, fixture, nil)

		require.Empty(t, plan.Diagnostics)
		require.Len(t, plan.RequestInlineHelpers, 1)
		require.Len(t, plan.ResponseInlineHelpers, 1)
		method := inlineFixtureMethodPlan(t, plan, fixture)
		requireInlineRequestFields(t, method, "ticketID")
		requireInlineResponseFields(t, method, "ticket")
	})

	t.Run("honors service inline defaults", func(t *testing.T) {
		fixture := newTicketIDInlineFixture("GetTicket")
		setServiceInlineByDefault(fixture.Service, false)

		plan := planInlineServiceFixture(t, fixture, nil)

		require.Empty(t, plan.Diagnostics)
		require.Empty(t, plan.RequestInlineHelpers)
		require.Empty(t, plan.ResponseInlineHelpers)
		plannedMethod := inlineFixtureMethodPlan(t, plan, fixture)
		assert.Nil(t, plannedMethod.InlineRequest)
		assert.Nil(t, plannedMethod.InlineResponse)
	})

	t.Run("method options override service defaults", func(t *testing.T) {
		fixture := newTicketIDInlineFixture("GetTicket")
		setMethodInline(fixture.Method, nil, nil, new(true))
		setServiceInlineByDefault(fixture.Service, false)

		plan := planInlineServiceFixture(t, fixture, nil)

		require.Empty(t, plan.Diagnostics)
		plannedMethod := inlineFixtureMethodPlan(t, plan, fixture)
		assert.Nil(t, plannedMethod.InlineRequest)
		requireInlineResponseFields(t, plannedMethod, "ticketID")
	})

	t.Run("side-specific method options override inline", func(t *testing.T) {
		fixture := newTicketIDInlineFixture("GetTicket")
		setMethodInline(fixture.Method, new(false), nil, new(true))

		plan := planInlineServiceFixture(t, fixture, nil)

		require.Empty(t, plan.Diagnostics)
		plannedMethod := inlineFixtureMethodPlan(t, plan, fixture)
		assert.Nil(t, plannedMethod.InlineRequest)
		requireInlineResponseFields(t, plannedMethod, "ticketID")
	})

	t.Run("plans server streaming inline request", func(t *testing.T) {
		fixture := newTicketIDInlineFixture("GetTicket")
		fixture.Method.ServerStreaming = true

		plan := planInlineServiceFixture(t, fixture, nil)

		require.Empty(t, plan.Diagnostics)
		methodPlan := inlineFixtureMethodPlan(t, plan, fixture)
		requireInlineRequestFields(t, methodPlan, "ticketID")
		assert.Nil(t, methodPlan.InlineResponse)
	})

	t.Run("plans client streaming inline response", func(t *testing.T) {
		fixture := newImportEventsInlineFixture()
		fixture.Method.ClientStreaming = true

		plan := planInlineServiceFixture(t, fixture, nil)

		require.Empty(t, plan.Diagnostics)
		methodPlan := inlineFixtureMethodPlan(t, plan, fixture)
		assert.Nil(t, methodPlan.InlineRequest)
		requireInlineResponseFields(t, methodPlan, "importedCount")
	})

	t.Run("reports explicit server streaming inline response", func(t *testing.T) {
		fixture := newTicketIDInlineFixture("GetTicket")
		fixture.Method.ServerStreaming = true
		setMethodInline(fixture.Method, nil, nil, new(true))

		plan := planInlineServiceFixture(t, fixture, nil)

		requireFatalDiagnostic(t, plan.Diagnostics, "facade inline response is not supported on server-streaming methods")
	})

	t.Run("reports explicit client streaming inline request", func(t *testing.T) {
		fixture := newImportEventsInlineFixture()
		fixture.Method.ClientStreaming = true
		setMethodInline(fixture.Method, nil, new(true), nil)

		plan := planInlineServiceFixture(t, fixture, nil)

		requireFatalDiagnostic(t, plan.Diagnostics, "facade inline request is not supported on client-streaming methods")
	})

	t.Run("reports explicit bidi inline options", func(t *testing.T) {
		fixture := newServiceInlineFixture(
			"SyncEvents",
			"SyncEventsRequest",
			[]*ProtoField{field("ticket_id", protoreflect.StringKind)},
			"SyncEventsResponse",
			[]*ProtoField{field("ticket_id", protoreflect.StringKind)},
		)
		fixture.Method.ClientStreaming = true
		fixture.Method.ServerStreaming = true
		setMethodInline(fixture.Method, nil, new(true), nil)

		plan := planInlineServiceFixture(t, fixture, nil)

		requireFatalDiagnostic(t, plan.Diagnostics, "facade inline request is not supported on bidi-streaming methods")
	})

	t.Run("skips automatic non struct inline messages", func(t *testing.T) {
		fixture := newTicketIDInlineFixture("GetTicket")
		fixture.Request.Options.SetFlatten(true)

		plan := planInlineServiceFixture(t, fixture, &ShapeIndex{
			Flattens: map[protoreflect.FullName]*ProtoMessage{fixture.Request.FullName: fixture.Request},
		})

		require.Empty(t, plan.Diagnostics)
		methodPlan := inlineFixtureMethodPlan(t, plan, fixture)
		assert.Nil(t, methodPlan.InlineRequest)
		requireInlineResponseFields(t, methodPlan, "ticketID")
	})

	t.Run("reports non struct inline messages", func(t *testing.T) {
		fixture := newTicketIDInlineFixture("GetTicket")
		fixture.Request.Options.SetFlatten(true)
		setMethodInline(fixture.Method, nil, new(true), nil)

		plan := planInlineServiceFixture(t, fixture, &ShapeIndex{
			Flattens: map[protoreflect.FullName]*ProtoMessage{fixture.Request.FullName: fixture.Request},
		})

		requireFatalDiagnostic(t, plan.Diagnostics, "facade inline requires an ordinary generated struct-shaped message")
	})

	t.Run("reports empty response inlining", func(t *testing.T) {
		response := plannerMessage(emptyFullName, "Empty")
		serviceMessage := ServiceMessagePlan{Type: emptyStructType()}

		_, diagnostics := NewPlanner().planServiceInlineHelper(
			response,
			serviceMessage,
			nil,
			"example.v1.TicketService.CloseTicket",
		)

		requireFatalDiagnostic(t, diagnostics, "facade inline requires an ordinary generated struct-shaped message")
	})

	t.Run("reports zero field inline messages", func(t *testing.T) {
		fixture := newServiceInlineFixture(
			"GetTicket",
			"GetTicketRequest",
			nil,
			"GetTicketResponse",
			[]*ProtoField{field("ticket_id", protoreflect.StringKind)},
		)
		setMethodInline(fixture.Method, nil, new(true), nil)

		plan := planInlineServiceFixture(t, fixture, nil)

		requireFatalDiagnostic(t, plan.Diagnostics, "facade inline requires a message with at least one generated field")
	})

	t.Run("reports incompatible inline helper reuse", func(t *testing.T) {
		stringType := TypePlan{Kind: TypeKindScalar, Scalar: ScalarKindString}
		messageType := TypePlan{Kind: TypeKindStruct, Ref: GoTypeRef{ImportPath: "github.com/example/service", Name: "TicketRef"}}
		requestHelper := ServiceInlineHelperPlan{
			ProtoName:      "example.v1.TicketRef",
			Type:           messageType,
			ToInlineName:   "TicketRefToInline",
			FromInlineName: "TicketRefFromInline",
			Fields: []ServiceInlineFieldPlan{{
				Name:      "ticketID",
				FieldName: "TicketID",
				Type:      stringType,
			}},
		}
		responseHelper := requestHelper
		service := ServicePlan{Methods: []ServiceMethodPlan{{
			InlineRequest:  &requestHelper,
			InlineResponse: &responseHelper,
		}}}

		_, _, diagnostics := plannedServiceInlineHelpers("service.proto", []ServicePlan{service})

		requireFatalDiagnostic(t, diagnostics, `planned Go name "TicketRefToInline"`)
	})

	t.Run("reports inline helper name collisions", func(t *testing.T) {
		colliding := plannerMessage("example.v1.Custom", "Custom")
		colliding.Options.SetName("GetTicketRequestToInline")
		colliding.Fields = []*ProtoField{field("value", protoreflect.StringKind)}
		fixture := newServiceInlineFixture(
			"GetTicket",
			"GetTicketRequest",
			[]*ProtoField{field("ticket_id", protoreflect.StringKind)},
			"GetTicketResponse",
			[]*ProtoField{field("ticket_id", protoreflect.StringKind)},
			colliding,
		)
		setMethodInline(fixture.Method, nil, new(true), nil)

		plan := planInlineServiceFixture(t, fixture, nil)

		requireFatalDiagnostic(t, plan.Diagnostics, `planned Go name "GetTicketRequestToInline"`)
	})

	t.Run("reports service name collisions", func(t *testing.T) {
		tests := map[string]struct {
			messageName string
			rpc         RPCOptions
		}{
			"service facade":       {messageName: "TicketService"},
			"unimplemented facade": {messageName: "UnimplementedTicketService"},
			"grpc adapter":         {messageName: "TicketServiceGRPCAdapter"},
			"grpc constructor":     {messageName: "NewTicketServiceGRPCClient"},
			"connect constructor":  {messageName: "NewTicketServiceConnectClient", rpc: RPCOptions{Connect: true}},
		}

		for name, tt := range tests {
			t.Run(name, func(t *testing.T) {
				file := protoFileWithOutput("service.proto", "github.com/example/service;service", "")
				message := plannerMessage("example.v1.Custom", "Custom")
				message.Options.SetName(tt.messageName)
				message.Fields = []*ProtoField{field("value", protoreflect.StringKind)}
				service := plannerService("example.v1.TicketService", "TicketService")
				attachMessagesToFile(file, message)
				attachServicesToFile(file, service)

				planner := NewPlanner()
				if tt.rpc.Enabled() {
					planner = NewPlanner(WithRPCPlanning(tt.rpc))
				}
				plan := planner.planFile(file, &ShapeIndex{})

				requireFatalDiagnostic(t, plan.Diagnostics, `planned Go name "`+tt.messageName+`"`)
			})
		}
	})

	t.Run("skips service planning when rpc is disabled", func(t *testing.T) {
		file := protoFileWithOutput("service.proto", "github.com/example/service;service", "")
		message := plannerMessage("example.v1.TicketService", "TicketService")
		service := plannerService("example.v1.TicketService", "TicketService")
		attachMessagesToFile(file, message)
		attachServicesToFile(file, service)

		plan := NewPlanner(WithRPCPlanning(RPCOptions{})).planFile(file, &ShapeIndex{})

		require.Empty(t, plan.Diagnostics)
		assert.Empty(t, plan.Services)
	})
}

func newTicketIDInlineFixture(methodName string) serviceInlineFixture {
	return newServiceInlineFixture(
		methodName,
		methodName+"Request",
		[]*ProtoField{field("ticket_id", protoreflect.StringKind)},
		methodName+"Response",
		[]*ProtoField{field("ticket_id", protoreflect.StringKind)},
	)
}

func newImportEventsInlineFixture() serviceInlineFixture {
	return newServiceInlineFixture(
		"ImportEvents",
		"ImportEventsRequest",
		[]*ProtoField{field("ticket_id", protoreflect.StringKind)},
		"ImportEventsResponse",
		[]*ProtoField{field("imported_count", protoreflect.Int32Kind)},
	)
}

func newServiceInlineFixture(
	methodName string,
	requestName string,
	requestFields []*ProtoField,
	responseName string,
	responseFields []*ProtoField,
	extraMessages ...*ProtoMessage,
) serviceInlineFixture {
	file := protoFileWithOutput("service.proto", "github.com/example/service;service", "")
	request := plannerMessage(protoreflect.FullName("example.v1."+requestName), protoreflect.Name(requestName))
	request.Fields = requestFields
	response := plannerMessage(protoreflect.FullName("example.v1."+responseName), protoreflect.Name(responseName))
	response.Fields = responseFields
	method := plannerMethod(
		protoreflect.FullName("example.v1.TicketService."+methodName),
		protoreflect.Name(methodName),
		request,
		response,
	)
	service := plannerService("example.v1.TicketService", "TicketService", method)
	messages := append([]*ProtoMessage{request, response}, extraMessages...)
	attachMessagesToFile(file, messages...)
	attachServicesToFile(file, service)
	return serviceInlineFixture{
		File:     file,
		Request:  request,
		Response: response,
		Method:   method,
		Service:  service,
	}
}

func planInlineServiceFixture(t *testing.T, fixture serviceInlineFixture, si *ShapeIndex) FilePlan {
	t.Helper()

	if si == nil {
		si = &ShapeIndex{}
	}
	return NewPlanner().planFile(fixture.File, si)
}

func inlineFixtureMethodPlan(t *testing.T, plan FilePlan, fixture serviceInlineFixture) ServiceMethodPlan {
	t.Helper()

	require.Len(t, plan.Services, 1)
	return serviceMethodByProtoName(t, plan.Services[0], fixture.Method.FullName)
}

func requireInlineRequestFields(t *testing.T, method ServiceMethodPlan, fields ...string) {
	t.Helper()

	require.NotNil(t, method.InlineRequest)
	assert.Equal(t, fields, inlineFieldNames(method.InlineRequest.Fields))
}

func requireInlineResponseFields(t *testing.T, method ServiceMethodPlan, fields ...string) {
	t.Helper()

	require.NotNil(t, method.InlineResponse)
	assert.Equal(t, fields, inlineFieldNames(method.InlineResponse.Fields))
}
