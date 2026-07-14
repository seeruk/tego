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

func TestCasingStyleName(t *testing.T) {
	tests := map[string]struct {
		style tegopb.CasingStyle
		want  string
	}{
		"camel case":           {tegopb.CasingStyle_CASING_STYLE_CAMEL_CASE, "httpApiUrlWatcherIdsVideo1080P"},
		"kebab case":           {tegopb.CasingStyle_CASING_STYLE_KEBAB_CASE, "http-api-url-watcher-ids-video-1080p"},
		"snake case":           {tegopb.CasingStyle_CASING_STYLE_SNAKE_CASE, "http_api_url_watcher_ids_video_1080p"},
		"screaming snake case": {tegopb.CasingStyle_CASING_STYLE_SCREAMING_SNAKE_CASE, "HTTP_API_URL_WATCHER_IDS_VIDEO_1080P"},
		"pascal case":          {tegopb.CasingStyle_CASING_STYLE_PASCAL_CASE, "HttpApiUrlWatcherIdsVideo1080P"},
		"lower Go case":        {tegopb.CasingStyle_CASING_STYLE_LOWER_GO_CASE, "httpAPIURLWatcherIDsVideo1080P"},
		"upper Go case":        {tegopb.CasingStyle_CASING_STYLE_UPPER_GO_CASE, "HTTPAPIURLWatcherIDsVideo1080P"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, casingStyleName("http_api_url_watcher_ids_video_1080p", tt.style))
		})
	}

	assert.Empty(t, casingStyleName("", tegopb.CasingStyle_CASING_STYLE_CAMEL_CASE))
}
