package testkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"testing"
)

type ReadinessHelper struct {
	TestName   string
	EnabledEnv string
	AddressEnv string
}

func (h ReadinessHelper) Run() {
	if os.Getenv(h.EnabledEnv) != "1" {
		return
	}
	generation := os.Getenv("ARDENTS_WORKLOAD_GENERATION")
	server := &http.Server{Addr: os.Getenv(h.AddressEnv), Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-Ardents-Generation", generation)
		writer.WriteHeader(http.StatusNoContent)
	})}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		os.Exit(2)
	}
}

func (h ReadinessHelper) Fixture(t testing.TB) (string, string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("cross-container readiness listener requires a non-loopback host bind; run the acceptance test on Linux")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate readiness port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release readiness port: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	raw, err := json.Marshal(map[string]any{
		"command": executable,
		"args":    []string{"-test.run=" + h.TestName},
		"env": map[string]string{
			h.EnabledEnv: "1",
			h.AddressEnv: fmt.Sprintf("0.0.0.0:%d", port),
		},
	})
	if err != nil {
		t.Fatalf("encode readiness workload: %v", err)
	}
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatalf("list readiness addresses: %v", err)
	}
	for _, address := range addresses {
		ip, _, parseErr := net.ParseCIDR(address.String())
		if parseErr == nil && ip.To4() != nil && ip.IsPrivate() && !ip.IsLoopback() {
			return string(raw), fmt.Sprintf("http://%s:%d/ready", ip, port), fmt.Sprintf("http://127.0.0.1:%d/ready", port)
		}
	}
	t.Fatal("Linux test container has no private IPv4 address")
	return "", "", ""
}
