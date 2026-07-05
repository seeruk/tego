package tego

import (
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
)

func TestWithErrorMapperAppliesToTransportOptions(t *testing.T) {
	mapper := func(err error) error {
		return fmt.Errorf("mapped: %w", err)
	}
	err := errors.New("boom")

	assert.EqualError(t, NewGRPCServerOptions(WithErrorMapper(mapper)).ErrorMapper(nil)(err), "mapped: boom")
	assert.EqualError(t, NewGRPCAdapterOptions(WithErrorMapper(mapper)).ErrorMapper(nil)(err), "mapped: boom")
	assert.EqualError(t, NewGRPCClientOptions(WithErrorMapper(mapper)).ErrorMapper(nil)(err), "mapped: boom")
	assert.EqualError(t, NewConnectHandlerOptions(WithErrorMapper(mapper)).ErrorMapper(nil)(err), "mapped: boom")
	assert.EqualError(t, NewConnectAdapterOptions(WithErrorMapper(mapper)).ErrorMapper(nil)(err), "mapped: boom")
	assert.EqualError(t, NewConnectClientOptions(WithErrorMapper(mapper)).ErrorMapper(nil)(err), "mapped: boom")
}

func TestWithConnectHandlerOptions(t *testing.T) {
	handlerOption := connect.WithCompression("test", nil, nil)

	options := NewConnectHandlerOptions(WithConnectHandlerOptions(handlerOption))

	assert.Equal(t, []connect.HandlerOption{handlerOption}, options.ConnectHandlerOptions())
}

func TestTransportOptionsDefaultErrorMapper(t *testing.T) {
	defaultMapper := func(err error) error {
		return fmt.Errorf("default: %w", err)
	}

	err := errors.New("boom")

	assert.EqualError(t, NewGRPCServerOptions().ErrorMapper(defaultMapper)(err), "default: boom")
}
