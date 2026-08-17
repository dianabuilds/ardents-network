//go:build linux && live

package network_test

import (
	"errors"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

func runBlockedPressure(t *testing.T) {
	t.Helper()
	count, err := strconv.Atoi(os.Getenv("ARDENTS_PRESSURE_CONNECTIONS"))
	if err != nil || count < 1 || count > 23 {
		t.Fatalf("invalid partial-handshake count %q", os.Getenv("ARDENTS_PRESSURE_CONNECTIONS"))
	}
	connections := make([]net.Conn, 0, count)
	defer func() {
		for _, connection := range connections {
			_ = connection.Close()
		}
	}()
	prefix := make([]byte, 128)
	prefix[0], prefix[1], prefix[2], prefix[3], prefix[4] = 22, 3, 3, 0x10, 0
	for range count {
		connection, dialErr := net.DialTimeout("tcp4", "203.0.113.8:8480", time.Second)
		if dialErr != nil {
			t.Fatal(dialErr)
		}
		if _, writeErr := connection.Write(prefix); writeErr != nil {
			_ = connection.Close()
			t.Fatal(writeErr)
		}
		connections = append(connections, connection)
		time.Sleep(500 * time.Millisecond)
	}
	writeBlockedSignal(t, "/run/evidence/pressure-ready")
	waitBlockedFile(t, "/run/evidence/pressure-release", 4*time.Minute)
	var closeErr error
	for _, connection := range connections {
		closeErr = errors.Join(closeErr, connection.Close())
	}
	connections = nil
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	writeBlockedSignal(t, "/run/evidence/pressure-closed")
}
