package tego

import (
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestRunPluginYiraFixture(t *testing.T) {
	input, err := os.ReadFile("testdata/yira.codegenreq.bin")
	require.NoError(t, err)

	var req pluginpb.CodeGeneratorRequest
	require.NoError(t, proto.Unmarshal(input, &req))
	require.True(t, slices.Contains(req.GetFileToGenerate(), "yirapb/v1/yira.proto"))

	plugin, err := protogen.Options{}.New(&req)
	require.NoError(t, err)

	assert.NoError(t, RunPlugin(plugin))
}
