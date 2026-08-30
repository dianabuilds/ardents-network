package endpoint

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

var applicationPathSequence atomic.Uint64

func TestResultChannelAcceptsDelayedDeclaredContract(t *testing.T) {
	applicationPath := shortApplicationPath(t)
	path, listener, err := listenResult(applicationPath, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	peer := make(chan net.Conn, 1)
	go func() {
		time.Sleep(75 * time.Millisecond)
		connection, dialErr := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
		if dialErr != nil {
			t.Error(dialErr)
			return
		}
		peer <- connection
	}()
	started := time.Now()
	connection, err := acceptResult(listener, time.Now().Add(time.Second))
	if err != nil || connection == nil {
		t.Fatalf("declared result admission connection=%v error=%v", connection, err)
	}
	defer connection.Close()
	defer (<-peer).Close()
	if time.Since(started) < 50*time.Millisecond {
		t.Fatal("result contract was selected by the retired timing window")
	}
	mode, err := os.Stat(path)
	if err != nil || runtime.GOOS != "windows" && mode.Mode().Perm() != 0o600 {
		t.Fatalf("result socket permissions=%v error=%v", mode, err)
	}
}

func TestResultChannelFailureRejectsApplicationContract(t *testing.T) {
	applicationPath := shortApplicationPath(t)
	blockedPath := applicationPath + ".result"
	if err := os.WriteFile(blockedPath, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := listenResult(applicationPath, time.Second); err == nil {
		t.Fatal("missing required result listener was accepted")
	}
}

func TestApplicationHandshakeRejectsUnsupportedContract(t *testing.T) {
	application, peer := net.Pipe()
	defer application.Close()
	defer peer.Close()
	result := make(chan error, 1)
	go func() { result <- acceptApplication(peer, time.Now().Add(time.Second)) }()
	if _, err := application.Write([]byte("ASAP\x01\x02")); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err == nil {
		t.Fatal("unsupported Application contract was accepted")
	}
}

func TestApplicationHandshakeRequiresOneCompleteFrame(t *testing.T) {
	application, peer := net.Pipe()
	defer application.Close()
	defer peer.Close()
	result := make(chan error, 1)
	go func() { result <- acceptApplication(peer, time.Now().Add(50*time.Millisecond)) }()
	if _, err := application.Write(applicationHello[:3]); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err == nil {
		t.Fatal("partial Application contract was accepted")
	}
}

func shortApplicationPath(t *testing.T) string {
	t.Helper()
	path := fmt.Sprintf("%s%casa-%d-%d.sock", os.TempDir(), os.PathSeparator, time.Now().UnixNano(), applicationPathSequence.Add(1))
	t.Cleanup(func() { _ = os.Remove(path); _ = os.Remove(path + ".result") })
	return path
}
