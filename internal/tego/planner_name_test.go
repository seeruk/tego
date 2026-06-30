package tego

import (
	"testing"

	"github.com/seeruk/tego/tegopb"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestPlannerNames(t *testing.T) {
	t.Run("plans message enum and field names with initialisms", func(t *testing.T) {
		message := &ProtoMessage{Name: "api_response", Options: &tegopb.MessageOptions{}}
		enum := protoEnum("example.v1.http_status", "HttpStatus")
		apiURL := &ProtoField{Name: "api_url", Options: &tegopb.FieldOptions{}}
		watcherIDs := &ProtoField{Name: "watcher_ids", Options: &tegopb.FieldOptions{}}
		metricQPS := &ProtoField{Name: "metric_qps", Options: &tegopb.FieldOptions{}}

		assert.Equal(t, "APIResponse", plannedMessageName(message))
		assert.Equal(t, "HTTPStatus", plannedEnumName(enum))
		assert.Equal(t, "APIURL", plannedFieldName(apiURL))
		assert.Equal(t, "WatcherIDs", plannedFieldName(watcherIDs))
		assert.Equal(t, "MetricQPS", plannedFieldName(metricQPS))
	})

	t.Run("plans enum constants with trimmed enum prefix", func(t *testing.T) {
		enum := protoEnum(
			"example.v1.TicketStatus", "TicketStatus",
			protoEnumValue("example.v1.TICKET_STATUS_IN_PROGRESS", "TicketStatus_TICKET_STATUS_IN_PROGRESS", 2, nil),
		)

		assert.Equal(t, "TicketStatusInProgress", plannedEnumConstantName(enum.Values[0], plannedEnumName(enum)))
	})

	t.Run("plans enum constants without enum prefix", func(t *testing.T) {
		enum := protoEnum(
			"example.v1.Status", "Status",
			protoEnumValue("example.v1.OPEN", "Open", 1, nil),
		)

		assert.Equal(t, "StatusOpen", plannedEnumConstantName(enum.Values[0], plannedEnumName(enum)))
	})

	t.Run("preserves explicit option names", func(t *testing.T) {
		messageOptions := &tegopb.MessageOptions{}
		messageOptions.SetName("api_response")
		fieldOptions := &tegopb.FieldOptions{}
		fieldOptions.SetName("api_url")
		enumOptions := &tegopb.EnumOptions{}
		enumOptions.SetName("http_status")
		valueOptions := &tegopb.EnumValueOptions{}
		valueOptions.SetName("ticket_status_open")

		message := &ProtoMessage{Name: "api_response", Options: messageOptions}
		field := &ProtoField{Name: "api_url", Options: fieldOptions}
		enum := protoEnum("example.v1.HTTPStatus", "HTTPStatus", protoEnumValue("example.v1.HTTP_STATUS_OPEN", "HTTPStatusOpen", 1, valueOptions))
		enum.Options = enumOptions

		assert.Equal(t, "api_response", plannedMessageName(message))
		assert.Equal(t, "api_url", plannedFieldName(field))
		assert.Equal(t, "http_status", plannedEnumName(enum))
		assert.Equal(t, "ticket_status_open", plannedEnumConstantName(enum.Values[0], plannedEnumName(enum)))
	})

	t.Run("plans lone initialisms as initialisms", func(t *testing.T) {
		a := &ProtoField{Name: protoreflect.Name("id"), Options: &tegopb.FieldOptions{}}
		b := &ProtoField{Name: protoreflect.Name("url"), Options: &tegopb.FieldOptions{}}
		c := &ProtoField{Name: protoreflect.Name("http"), Options: &tegopb.FieldOptions{}}

		assert.Equal(t, "ID", plannedFieldName(a))
		assert.Equal(t, "URL", plannedFieldName(b))
		assert.Equal(t, "HTTP", plannedFieldName(c))
	})
}
