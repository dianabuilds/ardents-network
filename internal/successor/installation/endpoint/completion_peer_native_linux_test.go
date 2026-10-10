//go:build installation_native

package endpoint

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Real kernel Unix credentials, but supplied expected facts and local fixture
// frames are not Release, manager, installed process or Endpoint admission.
func completionPeerFixture(t *testing.T) (*net.UnixConn, *net.UnixConn, generationSelection, [16]byte) {
	t.Helper()
	path := filepath.Join(nativeRequestDirectory(t), "completion.socket")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	if err := listener.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	client, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	server, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	if err := server.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	selected := generationSelection{Schema: "ardents-endpoint-installation-selection-v1", GenerationDigest: strings.Repeat("a", 64), BindingDigest: strings.Repeat("b", 64)}
	return server, client, selected, [16]byte{0: 1, 15: 255}
}

func TestInstallationNativeCompletionReadsExactKernelPeerWithoutAck(t *testing.T) {
	server, client, selected, invocation := completionPeerFixture(t)
	frame := []byte("010000000000000000000000000000ff" + strings.Repeat("a", 64) + strings.Repeat("b", 64))
	if n, err := client.Write(frame); err != nil || n != len(frame) {
		t.Fatal("fixture frame write failed", err)
	}
	if err := readInstallationCompletionPeer(server, uint32(os.Getpid()), uint32(os.Geteuid()), uint32(os.Getegid()), selected, invocation); err != nil {
		t.Fatal(err)
	}
	if err := client.SetReadDeadline(time.Now().Add(20 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	var byteBuffer [1]byte
	n, err := client.Read(byteBuffer[:])
	var timeout net.Error
	if n != 0 || !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatal("peer observation produced unexpected ACK", n, err)
	}
}

func TestInstallationNativeCompletionRefusesForeignPeerAndFrame(t *testing.T) {
	for _, change := range []string{"pid", "uid", "gid", "invocation", "generation", "binding", "truncated"} {
		t.Run(change, func(t *testing.T) {
			server, client, selected, invocation := completionPeerFixture(t)
			pid, uid, gid := uint32(os.Getpid()), uint32(os.Geteuid()), uint32(os.Getegid())
			frame := []byte("010000000000000000000000000000ff" + strings.Repeat("a", 64) + strings.Repeat("b", 64))
			switch change {
			case "pid":
				pid++
			case "uid":
				uid++
			case "gid":
				gid++
			case "invocation":
				frame[0] = '2'
			case "generation":
				frame[32] = 'b'
			case "binding":
				frame[96] = 'a'
			case "truncated":
				frame = frame[:159]
			}
			if n, err := client.Write(frame); err != nil || n != len(frame) {
				t.Fatal(err)
			}
			if err := client.CloseWrite(); err != nil {
				t.Fatal(err)
			}
			if err := readInstallationCompletionPeer(server, pid, uid, gid, selected, invocation); err == nil {
				t.Fatal("foreign/truncated completion admitted")
			}
		})
	}
}
