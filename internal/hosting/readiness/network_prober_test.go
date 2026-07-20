package readiness_test

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"ardents/internal/hosting/readiness"
	"github.com/stretchr/testify/require"
)

func TestNetworkProberRequiresGenerationHandshakeForTCPAndUnix(t *testing.T) {
	for _, network := range []string{"tcp", "unix"} {
		t.Run(network, func(t *testing.T) {
			address := "127.0.0.1:0"
			if network == "unix" {
				address = filepath.Join(t.TempDir(), "ready.sock")
			}
			listener, err := net.Listen(network, address)
			require.NoError(t, err)
			t.Cleanup(func() { _ = listener.Close() })
			go echoGenerationChallenge(listener)
			endpoint := "tcp://" + listener.Addr().String()
			if network == "unix" {
				endpoint = "unix://" + address
			}

			got := (readiness.NetworkProber{}).Check(context.Background(), endpoint, 51, 100*time.Millisecond)

			require.True(t, got.Reachable)
			require.Equal(t, readiness.ReasonReady, got.Reason)
		})
	}
}

func echoGenerationChallenge(listener net.Listener) {
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err == nil {
		_, _ = fmt.Fprint(conn, line)
	}
}

func TestNetworkProberRejectsUnsupportedScheme(t *testing.T) {
	got := (readiness.NetworkProber{}).Check(context.Background(), "udp://127.0.0.1:9", 1, time.Second)
	require.False(t, got.Reachable)
	require.Equal(t, readiness.ReasonUnsupportedScheme, got.Reason)
}

func TestNetworkProberRejectsNonLocalTargetsWithoutDialing(t *testing.T) {
	got := (readiness.NetworkProber{}).Check(context.Background(), "http://metadata.example/ready", 1, time.Second)
	require.False(t, got.Reachable)
	require.Equal(t, readiness.ReasonInvalidEndpoint, got.Reason)
}
