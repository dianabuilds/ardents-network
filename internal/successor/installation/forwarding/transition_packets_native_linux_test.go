//go:build network_signing_native

package forwarding

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// Physical private-channel tests only. No peer/process fixture supplies a
// successful Release, installed transition, original-cut proof or recovery.
func nativeTransitionPair(t *testing.T) (*net.UnixConn, *net.UnixConn) {
	t.Helper()
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Getuid() != 0 || os.Getgid() != 0 {
		t.Fatal("invalid environment: explicit disposable root preparation stand required", err)
	}
	controller, endpoint, err := transitionSocketPair()
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.FileConn(endpoint)
	closeErr := endpoint.Close()
	if err != nil {
		t.Fatal(errors.Join(err, closeErr, controller.Close()))
	}
	sender, ok := connection.(*net.UnixConn)
	if !ok || closeErr != nil {
		t.Fatal(errors.Join(ErrBinding, closeErr, controller.Close(), connection.Close()))
	}
	t.Cleanup(func() {
		for _, channel := range []*net.UnixConn{controller, sender} {
			if err := channel.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		}
	})
	for _, channel := range []*net.UnixConn{controller, sender} {
		if err := channel.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	return controller, sender
}

func TestForwardingTransitionPacketsNativeExactCredentialsAndSnapshotBytes(t *testing.T) {
	controller, sender := nativeTransitionPair(t)
	message := transitionMessage{Schema: "ardents-forwarding-transition-boundary-v1", OriginalContext: append(bytes.Repeat([]byte{'o'}, (64<<10)-1), '\n'), NativeContext: append(bytes.Repeat([]byte{'x'}, (64<<10)-1), '\n'), Binding: append(bytes.Repeat([]byte{'y'}, (32<<10)-1), '\n')}
	if err := writeTransitionMessage(sender, message); err != nil {
		t.Fatal(err)
	}
	actual, err := readTransitionMessage(controller, int32(os.Getpid()))
	if err != nil || !bytes.Equal(actual.OriginalContext, message.OriginalContext) || !bytes.Equal(actual.NativeContext, message.NativeContext) || !bytes.Equal(actual.Binding, message.Binding) {
		t.Fatal("actual full-sized private packet", err)
	}
	// Maximum framing bytes only; these deliberately opaque values supply no
	// valid original intent, native snapshot or recovery permission.
	message.Schema = "ardents-forwarding-transition-boundary-v2"
	message.Operation = "start"
	message.StartIntent = append(bytes.Repeat([]byte{'i'}, (8<<10)-1), '\n')
	if err := writeTransitionMessage(sender, message); err != nil {
		t.Fatal(err)
	}
	actual, err = readTransitionMessage(controller, int32(os.Getpid()))
	if err != nil || !bytes.Equal(actual.StartIntent, message.StartIntent) || !bytes.Equal(actual.OriginalContext, message.OriginalContext) || !bytes.Equal(actual.NativeContext, message.NativeContext) {
		t.Fatal("full intent/snapshot packet preimages lost", err)
	}
	if err := writeTransitionMessage(sender, message); err != nil {
		t.Fatal(err)
	}
	if _, err := readTransitionMessage(controller, int32(os.Getpid()+1)); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign kernel peer accepted", err)
	}
}

func TestForwardingTransitionPacketsNativeCloseEveryRefusedRight(t *testing.T) {
	for _, count := range []int{1, 20} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			controller, sender := nativeTransitionPair(t)
			file, err := os.Open("/dev/null")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			})
			rights := make([]int, count)
			for n := range rights {
				rights[n] = int(file.Fd())
			}
			raw, err := encodeTransitionMessage(transitionMessage{Schema: "ardents-forwarding-transition-boundary-v1"})
			if err != nil {
				t.Fatal(err)
			}
			before := nativeInputFDCount(t)
			if _, _, err := sender.WriteMsgUnix(raw, syscall.UnixRights(rights...), nil); err != nil {
				t.Fatal(err)
			}
			if _, err := readTransitionMessage(controller, int32(os.Getpid())); !errors.Is(err, ErrBinding) {
				t.Fatal("descriptor-bearing packet accepted", err)
			}
			if after := nativeInputFDCount(t); after != before {
				t.Fatal("refused/truncated ancillary data retained delivered descriptors", before, after)
			}
		})
	}
}

func TestForwardingTransitionPacketsNativeDistinguishPeerClosureFromMalformedData(t *testing.T) {
	// Observe the actual public recvmsg API separately. Only finite numeric
	// shape/error categories are logged; no packet or native identity is copied.
	probe, peer := nativeTransitionPair(t)
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	var data [8]byte
	var ancillary [128]byte
	n, cn, flags, _, probeErr := probe.ReadMsgUnix(data[:], ancillary[:])
	t.Logf("actual closed-peer shape: data=%d control=%d flags=%#x error=%T %v", n, cn, flags, probeErr, probeErr)
	controller, sender := nativeTransitionPair(t)
	if _, _, err := sender.WriteMsgUnix([]byte("partial packet"), nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := readTransitionMessage(controller, int32(os.Getpid())); !errors.Is(err, ErrBinding) || errors.Is(err, io.EOF) {
		t.Fatal("malformed bytes became orderly closure", err)
	}
	if err := sender.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readTransitionMessage(controller, int32(os.Getpid())); err != io.EOF {
		t.Fatal("actual orderly peer closure lost", err)
	}
}
