//go:build linux && node_process_native

package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// PinnedCommand verifies the original executable supplied by the native runner.
// The root driver runs outside the measured cgroups.
func PinnedCommand(t testing.TB) string {
	t.Helper()
	path := os.Getenv("ARDENTS_NODE_NATIVE_COMMAND")
	want := os.Getenv("ARDENTS_NODE_NATIVE_COMMAND_SHA256")
	if os.Geteuid() != 0 {
		t.Fatal("selected Node native root driver and pinned command unavailable")
	}
	if err := verifyPinnedCommand(path, want); err != nil {
		t.Fatal(err)
	}
	return path
}

func verifyPinnedCommand(path, want string) error {
	if !filepath.IsAbs(path) || len(want) != 64 {
		return errors.New("selected Node native pinned command unavailable")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	if hex.EncodeToString(hash[:]) != want {
		return errors.New("selected native command digest differs")
	}
	return nil
}

// DiagnosticBuffer captures an original child diagnostic during concurrent reads.
type DiagnosticBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (diagnostic *DiagnosticBuffer) Write(raw []byte) (int, error) {
	diagnostic.mu.Lock()
	defer diagnostic.mu.Unlock()
	return diagnostic.buffer.Write(raw)
}
func (diagnostic *DiagnosticBuffer) String() string {
	diagnostic.mu.Lock()
	defer diagnostic.mu.Unlock()
	return diagnostic.buffer.String()
}
