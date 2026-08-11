package namedsite

import (
	"bytes"
	"context"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunRoleServesHTTPApplicationOnlyOnOwnedUDS(t *testing.T) {
	t.Parallel()
	nonce := bytes.Repeat([]byte{0x51}, 32)
	ownedDirectory, err := os.MkdirTemp("", "gatec-role-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(ownedDirectory) })
	socketPath := filepath.Join(ownedDirectory, "http.sock")
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- RunRole(ctx, "http-application", RoleConfig{SocketPath: socketPath, NonceHex: hex.EncodeToString(nonce), EvidenceDir: ownedDirectory})
	}()
	var connection net.Conn
	err = nil
	for range 100 {
		connection, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		select {
		case roleErr := <-done:
			t.Fatalf("role stopped before accepting a connection: %v", roleErr)
		case <-time.After(5 * time.Millisecond):
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executeHTTPWorkload(connection, nonce); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := net.Dial("unix", socketPath); err == nil {
		t.Fatal("HTTP Application left an ordinary/reusable listener")
	}
}
