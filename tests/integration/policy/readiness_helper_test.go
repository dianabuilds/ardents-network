//go:build integration

package policy_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPolicyReadyHelper(t *testing.T) {
	if os.Getenv("ARDENTS_POLICY_READY_HELPER") != "1" {
		return
	}
	generation := os.Getenv("ARDENTS_WORKLOAD_GENERATION")
	server := &http.Server{Addr: os.Getenv("ARDENTS_POLICY_READY_ADDRESS"), Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generation)
		w.WriteHeader(http.StatusNoContent)
	})}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		os.Exit(2)
	}
}

func policyReadyFixture(t *testing.T) (string, string, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	executable, err := os.Executable()
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{"command": executable, "args": []string{"-test.run=TestPolicyReadyHelper"},
		"env": map[string]string{"ARDENTS_POLICY_READY_HELPER": "1", "ARDENTS_POLICY_READY_ADDRESS": fmt.Sprintf("0.0.0.0:%d", port)}})
	require.NoError(t, err)
	addresses, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, address := range addresses {
		ip, _, parseErr := net.ParseCIDR(address.String())
		if parseErr == nil && ip.To4() != nil && ip.IsPrivate() && !ip.IsLoopback() {
			return string(raw), fmt.Sprintf("http://%s:%d/ready", ip.String(), port), fmt.Sprintf("http://127.0.0.1:%d/ready", port)
		}
	}
	t.Fatal("Linux test container has no private IPv4 address")
	return "", "", ""
}
