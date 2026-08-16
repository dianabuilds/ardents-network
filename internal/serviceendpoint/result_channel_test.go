package serviceendpoint

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestResultChannelIsOptionalForStage4Peer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.sock")
	listener, err := listenLocal(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	started := time.Now()
	connection, err := acceptOptionalResult(listener)
	if err != nil || connection != nil || time.Since(started) > time.Second {
		t.Fatalf("optional result admission connection=%v error=%v elapsed=%v", connection, err, time.Since(started))
	}
}

func TestResultChannelAcceptsMaintainedTracer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.sock")
	listener, err := listenLocal(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	peer, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	connection, err := acceptOptionalResult(listener)
	if err != nil || connection == nil {
		t.Fatalf("maintained result admission connection=%v error=%v", connection, err)
	}
	defer connection.Close()
	mode, err := os.Stat(path)
	if err != nil || runtime.GOOS != "windows" && mode.Mode().Perm() != 0o600 {
		t.Fatalf("result socket permissions=%v error=%v", mode, err)
	}
}

func TestUnavailableResultCapabilityPreservesLegacyListener(t *testing.T) {
	applicationPath := filepath.Join(os.TempDir(), fmt.Sprintf("as-%d.sock", time.Now().UnixNano()))
	defer os.Remove(applicationPath)
	defer os.Remove(applicationPath + ".result")
	legacy, err := listenLocal(applicationPath, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	blockedPath := applicationPath + ".result"
	if err := os.WriteFile(blockedPath, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, optional := optionalResultListener(applicationPath, time.Second)
	if path != "" || optional != nil {
		t.Fatalf("unavailable optional capability became mandatory: path=%q listener=%v", path, optional)
	}
	peer, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: applicationPath, Net: "unix"})
	if err != nil {
		t.Fatalf("legacy raw listener was lost: %v", err)
	}
	_ = peer.Close()
}
