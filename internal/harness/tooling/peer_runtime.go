package tooling

import (
	"fmt"
	"net"
	"regexp"
	"time"
)

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func dialSmokePeer(address string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err == nil {
			return connection, nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("connect to tooling peer: %w", lastErr)
}
