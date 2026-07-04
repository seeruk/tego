package tego

import (
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestRunPlugin(t *testing.T) {
	req := loadYiraCodeGeneratorRequest(t)
	require.True(t, slices.Contains(req.GetFileToGenerate(), "yirapb/v1/yira.proto"))

	plugin, err := protogen.Options{}.New(req)
	require.NoError(t, err)

	var diagnostics strings.Builder
	require.NoError(t, runPlugin(plugin, &diagnostics))
	assert.Empty(t, diagnostics.String())

	response := plugin.Response()
	require.Empty(t, response.GetError())
	require.Len(t, response.GetFile(), 1)

	file := response.GetFile()[0]
	assert.Equal(t, "yira/v1/yira.tego.go", file.GetName())

	content := file.GetContent()
	_, err = parser.ParseFile(token.NewFileSet(), file.GetName(), content, parser.ParseComments)
	require.NoError(t, err)

	assertCommentLinesFit(t, content)
	goldie.New(t, goldie.WithFixtureDir("testdata/golden")).
		Assert(t, "run_plugin_yira_tego_go", []byte(content))
}

func TestRunPluginModuleRoot(t *testing.T) {
	t.Run("uses module root for type loading", func(t *testing.T) {
		req := loadYiraCodeGeneratorRequest(t)
		req.Parameter = new("module_root=.")

		plugin, err := protogen.Options{}.New(req)
		require.NoError(t, err)

		var diagnostics strings.Builder
		require.NoError(t, runPlugin(plugin, &diagnostics))
		assert.Empty(t, diagnostics.String())
		require.Empty(t, plugin.Response().GetError())
		require.Len(t, plugin.Response().GetFile(), 1)
	})

	t.Run("surfaces package loading failures when go type resolution is needed", func(t *testing.T) {
		req := loadYiraCodeGeneratorRequest(t)
		req.Parameter = new("module_root=testdata/missing-module-root")

		plugin, err := protogen.Options{}.New(req)
		require.NoError(t, err)

		var diagnostics strings.Builder
		err = runPlugin(plugin, &diagnostics)
		require.Error(t, err)
		assert.EqualError(t, err, "generate: plan contains fatal diagnostics")
		assert.Contains(t, diagnostics.String(), "fatal: yirapb.v1.Labels.values: couldn't resolve go_type")
		assert.Contains(t, diagnostics.String(), "load package")
	})
}

func TestRunPluginRPCParameter(t *testing.T) {
	t.Run("skips rpc output when disabled", func(t *testing.T) {
		req := loadYiraCodeGeneratorRequest(t)
		appendPluginParameter(req, "rpc=none")

		plugin, err := protogen.Options{}.New(req)
		require.NoError(t, err)

		var diagnostics strings.Builder
		require.NoError(t, runPlugin(plugin, &diagnostics))
		assert.Empty(t, diagnostics.String())

		content := generatedPluginResponseContent(t, plugin)
		assert.NotContains(t, content, "type TicketService interface")
		assert.NotContains(t, content, "RegisterTicketServiceGRPCServer")
		assert.NotContains(t, content, "NewTicketServiceConnectHandler")
	})

	t.Run("emits connect output", func(t *testing.T) {
		req := loadYiraCodeGeneratorRequest(t)
		appendPluginParameter(req, "rpc=connect")

		plugin, err := protogen.Options{}.New(req)
		require.NoError(t, err)

		var diagnostics strings.Builder
		require.NoError(t, runPlugin(plugin, &diagnostics))
		assert.Empty(t, diagnostics.String())

		content := generatedPluginResponseContent(t, plugin)
		assert.Contains(t, content, "NewTicketServiceConnectHandler")
		assert.NotContains(t, content, "RegisterTicketServiceGRPCServer")
	})
}

func TestWriteDiagnostics(t *testing.T) {
	plan := Plan{Files: []FilePlan{{
		Diagnostics: []Diagnostic{
			{Level: DiagnosticLevelWarning, Path: "warning.proto", Message: "be careful"},
			{Level: DiagnosticLevelFatal, Path: "fatal.proto", Message: "something broke"},
		},
	}}}
	var out strings.Builder

	require.NoError(t, writeDiagnostics(&out, plan))

	assert.Equal(t, "diagnostics:\n"+
		"- warning: warning.proto: be careful\n"+
		"- fatal: fatal.proto: something broke\n", out.String())
}

func TestWriteDiagnosticsSkipsEmptyPlans(t *testing.T) {
	var out strings.Builder

	require.NoError(t, writeDiagnostics(&out, Plan{}))

	assert.Empty(t, out.String())
}

func TestModuleRoot(t *testing.T) {
	t.Run("returns module root parameter", func(t *testing.T) {
		assert.Equal(t, ".", moduleRoot("module_root=."))
	})

	t.Run("unquotes module root parameter", func(t *testing.T) {
		assert.Equal(t, "../project", moduleRoot(`module_root="../project"`))
	})

	t.Run("returns empty when absent", func(t *testing.T) {
		assert.Empty(t, moduleRoot("module=github.com/seeruk/tego"))
	})
}

func appendPluginParameter(req *pluginpb.CodeGeneratorRequest, param string) {
	params := req.GetParameter()
	if params == "" {
		req.Parameter = new(param)
		return
	}

	params += "," + param
	req.Parameter = new(params)
}

func generatedPluginResponseContent(t *testing.T, plugin *protogen.Plugin) string {
	t.Helper()

	response := plugin.Response()
	require.Empty(t, response.GetError())
	require.Len(t, response.GetFile(), 1)
	return response.GetFile()[0].GetContent()
}

func loadYiraCodeGeneratorRequest(t *testing.T) *pluginpb.CodeGeneratorRequest {
	t.Helper()

	input, err := os.ReadFile("testdata/yira.codegenreq.bin")
	require.NoError(t, err)

	var req pluginpb.CodeGeneratorRequest
	require.NoError(t, proto.Unmarshal(input, &req))
	return &req
}
