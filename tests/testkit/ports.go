package testkit

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func ReserveLoopbackTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { require.NoError(t, listener.Close()) }()

	return listener.Addr().(*net.TCPAddr).Port
}
