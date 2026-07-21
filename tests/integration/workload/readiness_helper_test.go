//go:build integration

package workload_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkloadIntegrationReadyHelper(t *testing.T) {
	if os.Getenv("ARDENTS_WORKLOAD_INTEGRATION_HELPER") != "1" {
		return
	}
	generation := os.Getenv("ARDENTS_WORKLOAD_GENERATION")
	server := &http.Server{Addr: os.Getenv("ARDENTS_WORKLOAD_INTEGRATION_ADDRESS"), Handler: generationHandler(func() string { return generation })}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		os.Exit(2)
	}
}

func workloadReadyFixture(t *testing.T) (string, string, string) {
	t.Helper()
	port := reserveWorkloadPort(t)
	executable, err := os.Executable()
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{"command": executable, "args": []string{"-test.run=TestWorkloadIntegrationReadyHelper"},
		"env": map[string]string{"ARDENTS_WORKLOAD_INTEGRATION_HELPER": "1", "ARDENTS_WORKLOAD_INTEGRATION_ADDRESS": fmt.Sprintf("0.0.0.0:%d", port)}})
	require.NoError(t, err)
	return string(raw), workloadAdvertisedEndpoint(t, port), fmt.Sprintf("http://127.0.0.1:%d/ready", port)
}

func startGenerationReadyServer(t *testing.T, generation func() int64) (string, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	server := &http.Server{Handler: generationHandler(func() string { return strconv.FormatInt(generation(), 10) })}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	return workloadAdvertisedEndpoint(t, port), fmt.Sprintf("http://127.0.0.1:%d/ready", port)
}

func generationHandler(generation func() string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generation())
		w.WriteHeader(http.StatusNoContent)
	})
}

func reserveWorkloadPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { require.NoError(t, listener.Close()) }()
	return listener.Addr().(*net.TCPAddr).Port
}

//goland:noinspection ALL
func workloadAdvertisedEndpoint(t *testing.T, port int) string {
	t.Helper()
	addresses, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, address := range addresses {
		ip, _, parseErr := net.ParseCIDR(address.String())
		if parseErr == nil && ip.To4() != nil && ip.IsPrivate() && !ip.IsLoopback() {
			return fmt.Sprintf("http://%s:%d/ready", ip.String(), port)
		}
	}
	t.Fatal("Linux test container has no private IPv4 address")
	return ""
}
