//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestSourceTransitionNativePacketRefusalReleasesReceivedDescriptors(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid native stand", err)
	}
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	base, err := os.MkdirTemp(os.Getenv("ARDENTS_SOURCE_STOPPED_FUTURE_PARENT"), "transition-packet-")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, "original-descriptor")
	probe, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	// A failing regression still releases its exact received copies; the
	// assertion below is made before that test cleanup and before any GC.
	defer func() {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Error(err)
			return
		}
		for _, entry := range entries {
			target, err := os.Readlink("/proc/self/fd/" + entry.Name())
			if err != nil || target != path {
				continue
			}
			fd, err := strconv.Atoi(entry.Name())
			if err == nil && fd != int(probe.Fd()) {
				if err := syscall.Close(fd); err != nil {
					t.Error(err)
				}
			}
		}
	}()
	channel, endpoint, err := sourceTransitionSocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Close()
	connection, err := net.FileConn(endpoint)
	closed := endpoint.Close()
	if connection != nil {
		defer connection.Close()
	}
	if err != nil || closed != nil {
		t.Fatal(errors.Join(err, closed))
	}
	sender, ok := connection.(*net.UnixConn)
	if !ok {
		t.Fatal("invalid native packet endpoint")
	}
	if err := channel.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	// Loss of the required credential option is a refusal. Kernel SCM_RIGHTS
	// can still deliver real FDs in that refused packet; framing must close
	// them even though credentials are absent. No successful peer is supplied.
	raw, err := channel.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var optionErr error
	if err := raw.Control(func(fd uintptr) {
		optionErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_PASSCRED, 0)
	}); err != nil || optionErr != nil {
		t.Fatal(errors.Join(err, optionErr))
	}
	watched := map[string]bool{path: true}
	before := sourceHistoryDescriptors(t, watched)
	message, _ := json.Marshal(sourceTransitionMessage{})
	message = append(message, '\n')
	for i := 0; i < 8; i++ {
		body := message
		if i%2 != 0 {
			body = bytes.Repeat([]byte{'x'}, 4*sourceTransitionPacketLimit)
		}
		if _, _, err := sender.WriteMsgUnix(body, syscall.UnixRights(int(probe.Fd())), nil); err != nil {
			t.Fatal(err)
		}
		if _, err := readSourceTransitionMessage(channel, int32(os.Getpid())); !errors.Is(err, ErrBinding) {
			t.Fatal("packet without original credentials accepted", err)
		}
		if after := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(before, after) {
			t.Fatalf("refused actual rights retained FDs before GC: before=%v after=%v", before, after)
		}
	}
	t.Log("eight actual SCM_RIGHTS refusals released received copies before GC")
}

func TestSourceTransitionNativePacketClosedGrammar(t *testing.T) {
	if os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("invalid native credential environment")
	}
	canonical, _ := json.Marshal(sourceTransitionMessage{})
	canonical = append(canonical, '\n')
	for _, body := range [][]byte{[]byte("{}\n"), append(bytes.Clone(canonical), '\n'), []byte("null\n"), bytes.Repeat([]byte{'x'}, sourceTransitionPacketLimit+1)} {
		func() {
			channel, endpoint, err := sourceTransitionSocketPair()
			if err != nil {
				t.Fatal(err)
			}
			defer channel.Close()
			connection, err := net.FileConn(endpoint)
			closed := endpoint.Close()
			if connection != nil {
				defer connection.Close()
			}
			if err != nil || closed != nil {
				t.Fatal(errors.Join(err, closed))
			}
			if err := channel.SetDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := connection.Write(body); err != nil {
				t.Fatal(err)
			}
			if _, err := readSourceTransitionMessage(channel, int32(os.Getpid())); !errors.Is(err, ErrBinding) {
				t.Fatal("unknown/oversized/noncanonical frame accepted", err)
			}
		}()
	}
}
