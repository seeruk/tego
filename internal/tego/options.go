package tego

import (
	"errors"
	"fmt"
	"go/token"
	"strings"

	"github.com/seeruk/tego/internal/protogenx"
)

const defaultConnectPackageSuffix = "connect"

// RPCOptions controls which RPC surfaces Tego plans and generates for protobuf services.
type RPCOptions struct {
	Connect              bool
	ConnectPackageSuffix string
	GRPC                 bool
}

func defaultRPCOptions() RPCOptions {
	return RPCOptions{
		Connect:              true,
		ConnectPackageSuffix: defaultConnectPackageSuffix,
		GRPC:                 true,
	}
}

// Enabled reports whether any RPC code should be planned or generated.
func (options RPCOptions) Enabled() bool {
	return options.Connect || options.GRPC
}

func rpcOptionsFromParams(params string) (RPCOptions, error) {
	options := defaultRPCOptions()
	if values, ok := protogenx.ParameterValues(params, "rpc"); ok {
		parsed, err := parseRPCOptions(values)
		if err != nil {
			return RPCOptions{}, err
		}
		options.Connect = parsed.Connect
		options.GRPC = parsed.GRPC
	}

	if suffix, ok := protogenx.ParameterValue(params, "connect_package_suffix"); ok {
		options.ConnectPackageSuffix = suffix
	}

	if err := validateRPCOptions(options); err != nil {
		return RPCOptions{}, err
	}

	return options, nil
}

func parseRPCOptions(values []string) (RPCOptions, error) {
	var options RPCOptions
	seenNone := false
	seenValue := false

	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				return RPCOptions{}, errors.New("rpc parameter contains empty value")
			}

			seenValue = true
			switch value {
			case "none":
				seenNone = true
			case "connect":
				options.Connect = true
			case "grpc":
				options.GRPC = true
			default:
				return RPCOptions{}, fmt.Errorf("unsupported rpc value %q", value)
			}
		}
	}

	if !seenValue {
		return RPCOptions{}, errors.New("rpc parameter contains no values")
	}
	if seenNone && options.Enabled() {
		return RPCOptions{}, errors.New("rpc value \"none\" cannot be combined with other values")
	}
	if seenNone {
		return RPCOptions{}, nil
	}

	return options, nil
}

func validateRPCOptions(options RPCOptions) error {
	if options.ConnectPackageSuffix != "" && !token.IsIdentifier(options.ConnectPackageSuffix) {
		return fmt.Errorf("connect_package_suffix %q is not a valid Go identifier", options.ConnectPackageSuffix)
	}
	return nil
}
