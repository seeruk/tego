package tego

import (
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrUnimplemented marks a facade service method that has not been implemented.
var ErrUnimplemented = errors.New("tego: unimplemented")

// GRPCError maps Tego sentinel errors to native gRPC errors.
func GRPCError(err error) error {
	if errors.Is(err, ErrUnimplemented) {
		return status.Error(codes.Unimplemented, err.Error())
	}
	return err
}

// ConnectError maps Tego sentinel errors to native Connect errors.
func ConnectError(err error) error {
	if errors.Is(err, ErrUnimplemented) {
		return connect.NewError(connect.CodeUnimplemented, err)
	}
	return err
}
