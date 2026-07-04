package tego

import (
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrUnimplemented(t *testing.T) {
	err := fmt.Errorf("TicketService.GetTicket: %w", ErrUnimplemented)

	assert.ErrorIs(t, err, ErrUnimplemented)
	assert.True(t, errors.Is(err, ErrUnimplemented))
}

func TestGRPCError(t *testing.T) {
	err := fmt.Errorf("TicketService.GetTicket: %w", ErrUnimplemented)

	mapped := GRPCError(err)

	assert.Equal(t, codes.Unimplemented, status.Code(mapped))
	assert.Equal(t, err.Error(), status.Convert(mapped).Message())
}

func TestGRPCErrorPassthrough(t *testing.T) {
	err := errors.New("boom")

	assert.Same(t, err, GRPCError(err))
	assert.NoError(t, GRPCError(nil))
}

func TestConnectError(t *testing.T) {
	err := fmt.Errorf("TicketService.GetTicket: %w", ErrUnimplemented)

	mapped := ConnectError(err)

	assert.Equal(t, connect.CodeUnimplemented, connect.CodeOf(mapped))
	assert.ErrorIs(t, mapped, ErrUnimplemented)
}

func TestConnectErrorPassthrough(t *testing.T) {
	err := errors.New("boom")

	assert.Same(t, err, ConnectError(err))
	assert.NoError(t, ConnectError(nil))
}
