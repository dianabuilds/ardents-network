package daemon

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestUnixHTTPServerCreatesPrivateSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are unavailable on windows")
	}
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	path := filepath.Join(dir, "control.sock")
	server, listener, err := newUnixHTTPServer(path, http.NotFoundHandler())
	if err != nil {
		t.Fatalf("newUnixHTTPServer() error = %v", err)
	}
	if server == nil {
		t.Fatal("server = nil")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode = %o", info.Mode().Perm())
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("socket still exists: %v", err)
	}
}
