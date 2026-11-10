//go:build installation_native

package systemd

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Closing after a verified frame's headers isolates decoder interruption,
// without supplying manager identity, properties or startup authority.
func TestInstallationNativeReferenceDecoderCloseAtHeaderPadding(t *testing.T) {
	transport, peer := referenceSocketPair(t)
	transport.binary.Store(true)
	frame := referenceReplyFixture()
	if _, err := peer.Write(frame); err != nil {
		t.Fatal(err)
	}
	boundary := 16 + int(binary.LittleEndian.Uint32(frame[12:16]))
	header := make([]byte, boundary)
	if _, err := io.ReadFull(transport, header); err != nil || !bytes.Equal(header, frame[:boundary]) {
		t.Fatal("decoder header bytes changed", err)
	}
	if err := transport.Close(); err != nil {
		t.Fatal(err)
	}
	// The upstream decoder requests this final padding outside its error
	// boundary. It must remain available only from the original verified frame.
	padding := make([]byte, len(frame)-boundary)
	if _, err := io.ReadFull(transport, padding); err != nil || !bytes.Equal(padding, frame[boundary:]) {
		t.Fatal("closed transport interrupted verified header padding", err)
	}
	if n, err := transport.Read(make([]byte, 1)); n != 0 || !errors.Is(err, net.ErrClosed) {
		t.Fatal("decoder acquired another frame after original transport close", n, err)
	}
}

// Coalesced replies exercise actual Unix read-ahead, not manager authority.
// Only a fully checked frame may reach the decoder or drain after owner Close.
func TestInstallationNativeReferenceBinaryReadAhead(t *testing.T) {
	for _, action := range []string{"next-frame", "owner-close", "socket-loss", "invalid-frame", "partial-frame"} {
		t.Run(action, func(t *testing.T) {
			transport, peer := referenceSocketPair(t)
			transport.binary.Store(true)
			first := referenceReplyFixture()
			second := append([]byte(nil), first...)
			switch action {
			case "invalid-frame":
				second[0] = 'x'
			case "partial-frame":
				second = second[:10]
			}
			wire := append(append([]byte(nil), first...), second...)
			if n, err := peer.Write(wire); err != nil || n != len(wire) {
				t.Fatal("original coalesced write", n, err)
			}
			got := make([]byte, len(first))
			if n, err := io.ReadFull(transport, got); err != nil || n != len(first) || !bytes.Equal(got, first) {
				t.Fatal("first checked frame changed", n, err)
			}
			if transport.reader == nil || transport.reader.Buffered() != len(second) {
				t.Fatal("actual Unix read-ahead not exercised")
			}
			var want error
			switch action {
			case "owner-close":
				if err := transport.Close(); err != nil {
					t.Fatal(err)
				}
				want = net.ErrClosed
			case "socket-loss":
				if err := transport.socket.Close(); err != nil {
					t.Fatal(err)
				}
				want = net.ErrClosed
			case "invalid-frame":
				want = ErrObservation
			case "partial-frame":
				if err := peer.Close(); err != nil {
					t.Fatal(err)
				}
				want = io.ErrUnexpectedEOF
			}
			clear(got)
			n, err := transport.Read(got)
			if want == nil {
				if err != nil || n != len(first) || !bytes.Equal(got, first) {
					t.Fatal("second independently checked frame changed", n, err)
				}
			} else if n != 0 || !errors.Is(err, want) || !bytes.Equal(got, make([]byte, len(got))) {
				t.Fatal("unverified read-ahead escaped original refusal", n, err, want)
			}
		})
	}
}

func TestInstallationNativeReferenceReadAheadStartsAfterBegin(t *testing.T) {
	transport, peer := referenceSocketPair(t)
	line := []byte("OK original-guid\r\n")
	frame := referenceReplyFixture()
	wire := append(append([]byte(nil), line...), frame...)
	if n, err := peer.Write(wire); err != nil || n != len(wire) {
		t.Fatal("original AUTH and binary delivery", n, err)
	}
	got := make([]byte, len(line)+len(frame))
	for _, expected := range line {
		if n, err := transport.Read(got); err != nil || n != 1 || got[0] != expected {
			t.Fatal("AUTH read crossed its original byte boundary", n, err)
		}
		if transport.reader != nil || transport.binary.Load() {
			t.Fatal("binary read-ahead began during AUTH")
		}
	}
	begin := []byte("BEGIN\r\n")
	if n, err := transport.Write(begin); err != nil || n != len(begin) {
		t.Fatal("original BEGIN write", n, err)
	}
	if _, err := io.ReadFull(peer, got[:len(begin)]); err != nil || !bytes.Equal(got[:len(begin)], begin) {
		t.Fatal("BEGIN bytes changed", err)
	}
	if n, err := transport.Read(got); err != nil || n != len(frame) || !bytes.Equal(got[:n], frame) {
		t.Fatal("first checked binary frame changed after BEGIN", n, err)
	}
}

// Only physical Unix I/O is supplied, never successful manager or Ref facts.
func referenceSocketPair(t *testing.T) (*referenceTransport, *net.UnixConn) {
	t.Helper()
	parent := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	if parent == "" {
		t.Fatal("selected native root missing")
	}
	dir, err := os.MkdirTemp(parent, "manager-reference-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, "bus"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	client, err := net.DialUnix("unix", nil, listener.Addr().(*net.UnixAddr))
	if err != nil {
		t.Fatal(err)
	}
	peer, err := listener.AcceptUnix()
	if err != nil {
		client.Close()
		t.Fatal(err)
	}
	transport := &referenceTransport{socket: client}
	t.Cleanup(func() { transport.Close(); peer.Close() })
	return transport, peer
}

func TestInstallationNativeReferenceCloseJoinsOriginalRead(t *testing.T) {
	t.Parallel()
	transport, peer := referenceSocketPair(t)
	joined := make(chan error, 1)
	go func() { _, err := transport.Read(make([]byte, 1)); joined <- err }()
	awaitReferenceIO(t, &transport.readMu)
	if err := transport.Close(); err != nil {
		t.Fatal(err)
	}
	if !transport.readMu.TryLock() {
		t.Fatal("Close returned with original read still running")
	}
	transport.readMu.Unlock()
	select {
	case err := <-joined:
		if err == nil {
			t.Fatal("closed reader succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("original read not joined")
	}
	if n, err := transport.Read(make([]byte, 1)); n != 0 || !errors.Is(err, net.ErrClosed) {
		t.Fatal("late read borrowed closed socket", n, err)
	}
	if n, err := transport.Write([]byte("late")); n != 0 || !errors.Is(err, net.ErrClosed) {
		t.Fatal("late write borrowed closed socket", n, err)
	}
	peer.SetReadDeadline(time.Now().Add(time.Second))
	if n, err := peer.Read(make([]byte, 1)); n != 0 || err == nil {
		t.Fatal("original peer did not observe physical close", n, err)
	}
}

func TestInstallationNativeStartupInterruptionRetainsOriginalReadCause(t *testing.T) {
	transport, _ := referenceSocketPair(t)
	var interruptedIO atomic.Bool
	client := &startupBus{transport: transport, interruptedIO: &interruptedIO}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	joined := make(chan error, 1)
	go func() { _, err := transport.Read(make([]byte, 1)); joined <- err }()
	awaitReferenceIO(t, &transport.readMu)
	cancel()
	if err := client.interrupt(); err != nil {
		t.Fatal("actual original transport close failed", err)
	}
	readErr := <-joined
	if !errors.Is(readErr, net.ErrClosed) {
		t.Fatal("original Unix read did not observe its actual interruption", readErr)
	}
	if err := client.callFailure(ctx, readErr); !errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
		t.Fatal("joined original cancellation became failed Startup", err)
	}
	if !transport.readMu.TryLock() {
		t.Fatal("interruption returned before original reader joined")
	}
	transport.readMu.Unlock()
}

// No bytes are supplied by the peer. Once the original Read owns readMu it
// cannot finish until its socket is interrupted; Close must join that borrow.
func awaitReferenceIO(t *testing.T, lock *sync.Mutex) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for lock.TryLock() {
		lock.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("original I/O did not enter its retained borrow")
		}
		runtime.Gosched()
	}
}

func TestInstallationNativeReferenceCloseJoinsOriginalWrite(t *testing.T) {
	t.Parallel()
	transport, _ := referenceSocketPair(t)
	if err := transport.socket.SetWriteBuffer(1024); err != nil {
		t.Fatal(err)
	}
	if err := transport.socket.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	joined := make(chan error, 1)
	go func() { _, err := transport.Write(make([]byte, 65536)); joined <- err }()
	awaitReferenceIO(t, &transport.writeMu)
	if err := transport.Close(); err != nil {
		t.Fatal(err)
	}
	if !transport.writeMu.TryLock() {
		t.Fatal("Close returned with original write still running")
	}
	transport.writeMu.Unlock()
	select {
	case err := <-joined:
		if err == nil {
			t.Fatal("unread original write completed")
		}
	case <-time.After(time.Second):
		t.Fatal("original write not joined")
	}
}

func TestInstallationNativeReferenceRejectsInnerLengthBeforeExposure(t *testing.T) {
	t.Parallel()
	transport, peer := referenceSocketPair(t)
	transport.binary.Store(true)
	frame := referenceReplyFixture()
	// The outer envelope remains small and complete. Its sender scalar alone
	// declares a length that cannot fit; no upstream byte may be exposed.
	binary.LittleEndian.PutUint32(frame[28:32], 0xffffffff)
	if _, err := peer.Write(frame); err != nil {
		t.Fatal(err)
	}
	if err := transport.socket.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if n, err := transport.Read(make([]byte, 1)); n != 0 || !errors.Is(err, ErrObservation) {
		t.Fatal("unchecked inner length exposed to client decoder", n, err)
	}
}

func TestInstallationNativeReferenceExposesValidatedFrame(t *testing.T) {
	t.Parallel()
	transport, peer := referenceSocketPair(t)
	transport.binary.Store(true)
	frame := referenceReplyFixture()
	if _, err := peer.Write(frame); err != nil {
		t.Fatal(err)
	}
	if err := transport.socket.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	// Small original reads exercise the retained remainder, not another frame.
	var got []byte
	for len(got) < len(frame) {
		part := make([]byte, 3)
		n, err := transport.Read(part)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, part[:n]...)
	}
	if !bytes.Equal(got, frame) {
		t.Fatal("validated frame bytes changed")
	}
}

func TestInstallationNativeReferenceRejectsOversizedFrameBeforePayload(t *testing.T) {
	t.Parallel()
	transport, peer := referenceSocketPair(t)
	transport.binary.Store(true)
	header := make([]byte, 16)
	header[0] = 'l'
	header[1] = 2
	header[3] = 1
	binary.LittleEndian.PutUint32(header[4:8], 1<<30)
	if _, err := peer.Write(header); err != nil {
		t.Fatal(err)
	}
	transport.socket.SetReadDeadline(time.Now().Add(time.Second))
	if n, err := transport.Read(make([]byte, 1)); n != 0 || !errors.Is(err, ErrObservation) {
		t.Fatal("declared payload read before envelope refusal", n, err)
	}
}

// Independent little-endian empty method reply: reply serial and broker sender.
func referenceReplyFixture() []byte {
	header := []byte{5, 1, 'u', 0, 1, 0, 0, 0, 7, 1, 's', 0, 20, 0, 0, 0}
	header = append(header, []byte("org.freedesktop.DBus")...)
	header = append(header, 0)
	frame := make([]byte, 16)
	frame[0] = 'l'
	frame[1] = 2
	frame[3] = 1
	binary.LittleEndian.PutUint32(frame[8:12], 1)
	binary.LittleEndian.PutUint32(frame[12:16], uint32(len(header)))
	frame = append(frame, header...)
	for len(frame)%8 != 0 {
		frame = append(frame, 0)
	}
	return frame
}

func TestInstallationNativeReferenceClosedScalarGrammar(t *testing.T) {
	frame := referenceReplyFixture()
	if err := verifyReferenceFrame(frame, ""); err != nil {
		t.Fatal("independent empty broker reply refused", err)
	}
	for _, mutate := range []func([]byte){
		func(b []byte) { b[1] = 1 },
		func(b []byte) { b[3] = 2 },
		func(b []byte) { b[16] = 9; binary.LittleEndian.PutUint32(b[20:24], 1) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[28:32], 0xffffffff) },
		func(b []byte) { b[26] = 'a' },
		func(b []byte) { b[32] = 'x' },
	} {
		bad := append([]byte(nil), frame...)
		mutate(bad)
		if err := verifyReferenceFrame(bad, ""); !errors.Is(err, ErrObservation) {
			t.Fatal("foreign/unbounded reply admitted", err)
		}
	}
}

func TestInstallationNativeReferenceRefusesMissingOrCancelledCaller(t *testing.T) {
	if ref, err := RetainEndpoint(nil); ref != nil || !errors.Is(err, ErrInput) {
		t.Fatal("missing caller acquired reference", ref, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if ref, err := RetainEndpoint(ctx); ref != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled caller acquired reference", ref, err)
	}
	var ref *EndpointReference
	if err := ref.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ref.Observe(t.Context()); !errors.Is(err, ErrInput) {
		t.Fatal("missing original reference fabricated observation", err)
	}
}
