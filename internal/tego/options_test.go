package tego

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRPCOptions(t *testing.T) {
	tests := map[string]struct {
		values  []string
		want    RPCOptions
		wantErr string
	}{
		"none": {
			values: []string{"none"},
		},
		"grpc": {
			values: []string{"grpc"},
			want:   RPCOptions{GRPC: true},
		},
		"connect": {
			values: []string{"connect"},
			want:   RPCOptions{Connect: true},
		},
		"grpc and connect": {
			values: []string{"grpc", "connect"},
			want:   RPCOptions{GRPC: true, Connect: true},
		},
		"comma value": {
			values: []string{"grpc,connect"},
			want:   RPCOptions{GRPC: true, Connect: true},
		},
		"repeated values": {
			values: []string{"grpc", "grpc"},
			want:   RPCOptions{GRPC: true},
		},
		"whitespace": {
			values: []string{" grpc ", " connect "},
			want:   RPCOptions{GRPC: true, Connect: true},
		},
		"empty": {
			values:  []string{""},
			wantErr: "rpc parameter contains empty value",
		},
		"unknown": {
			values:  []string{"twirp"},
			wantErr: `unsupported rpc value "twirp"`,
		},
		"none mixed with adapter": {
			values:  []string{"none", "grpc"},
			wantErr: `rpc value "none" cannot be combined with other values`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseRPCOptions(tt.values)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRPCOptionsFromParams(t *testing.T) {
	t.Run("defaults to supported rpc output", func(t *testing.T) {
		options, err := rpcOptionsFromParams("module=github.com/seeruk/tego")

		require.NoError(t, err)
		assert.Equal(t, defaultRPCOptions(), options)
	})

	t.Run("parses rpc none", func(t *testing.T) {
		options, err := rpcOptionsFromParams("module=github.com/seeruk/tego,rpc=none")

		require.NoError(t, err)
		assert.False(t, options.Enabled())
	})

	t.Run("parses comma separated grpc value", func(t *testing.T) {
		options, err := rpcOptionsFromParams("module=github.com/seeruk/tego,rpc=grpc")

		require.NoError(t, err)
		assert.Equal(t, RPCOptions{ConnectPackageSuffix: defaultConnectPackageSuffix, GRPC: true}, options)
	})

	t.Run("parses connect value", func(t *testing.T) {
		options, err := rpcOptionsFromParams("rpc=connect")

		require.NoError(t, err)
		assert.Equal(t, RPCOptions{Connect: true, ConnectPackageSuffix: defaultConnectPackageSuffix}, options)
	})

	t.Run("parses custom connect package suffix", func(t *testing.T) {
		options, err := rpcOptionsFromParams("connect_package_suffix=connectgo")

		require.NoError(t, err)
		assert.Equal(t, "connectgo", options.ConnectPackageSuffix)
	})

	t.Run("parses empty connect package suffix", func(t *testing.T) {
		options, err := rpcOptionsFromParams("connect_package_suffix=")

		require.NoError(t, err)
		assert.Empty(t, options.ConnectPackageSuffix)
	})

	t.Run("rejects invalid connect package suffix", func(t *testing.T) {
		_, err := rpcOptionsFromParams("connect_package_suffix=connect-go")

		require.Error(t, err)
		assert.EqualError(t, err, `connect_package_suffix "connect-go" is not a valid Go identifier`)
	})

	t.Run("rejects unsupported values", func(t *testing.T) {
		_, err := rpcOptionsFromParams("rpc=twirp")

		require.Error(t, err)
		assert.EqualError(t, err, `unsupported rpc value "twirp"`)
	})

	t.Run("rejects empty values", func(t *testing.T) {
		_, err := rpcOptionsFromParams("rpc=grpc,")

		require.Error(t, err)
		assert.EqualError(t, err, "rpc parameter contains empty value")
	})
}
