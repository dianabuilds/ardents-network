package testkit

import (
	"os"
	"sync"
	"testing"

	transport "ardents/internal/network/api"
)

var configureLoopbackTransportOnce sync.Once

func ConfigureLoopbackTransportOnce() {
	configureLoopbackTransportOnce.Do(func() {
		if err := os.Setenv(transport.BindAddressEnv, "127.0.0.1"); err != nil {
			panic(err)
		}
	})
}

func ConfigureLoopbackTransport(t *testing.T) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("set %s: %v", transport.BindAddressEnv, r)
		}
	}()
	ConfigureLoopbackTransportOnce()
}
