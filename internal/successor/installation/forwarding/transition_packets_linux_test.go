package forwarding

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"
)

func TestForwardingTransitionPacketPreservesCommittedBytesAndClosedShape(t *testing.T) {
	original := transitionMessage{Schema: "ardents-forwarding-transition-boundary-v1", Nonce: digest([]byte("private channel nonce")), Sequence: 1, Operation: "replacement", NativeContext: []byte("{\"native\":\"byte fixture\"}\n"), Binding: []byte("{\"binding\":\"byte fixture\"}\n")}
	raw, err := encodeTransitionMessage(original)
	if err != nil {
		t.Fatal(err)
	}
	read, err := decodeTransitionMessage(raw)
	if err != nil || !bytes.Equal(read.NativeContext, original.NativeContext) || !bytes.Equal(read.Binding, original.Binding) {
		t.Fatal("exact snapshot bytes lost", err)
	}
	original.Binding[0] ^= 1
	if bytes.Equal(read.Binding, original.Binding) {
		t.Fatal("caller rebound copied snapshot bytes")
	}
	for _, bad := range [][]byte{bytes.TrimSuffix(raw, []byte{'\n'}), append(bytes.Clone(raw), '\n'), bytes.Replace(raw, []byte("{\"schema\""), []byte("{ \"schema\""), 1), bytes.Replace(raw, []byte("\"sequence\":1,"), []byte("\"sequence\":2,\"sequence\":1,"), 1), bytes.Replace(raw, []byte("\"sequence\":1,"), []byte("\"sequence\":1,\"authority\":true,"), 1)} {
		if _, err := decodeTransitionMessage(bad); err == nil {
			t.Fatal("ambiguous private packet admitted")
		}
	}
	for _, sizes := range [][3]int{{64 << 10, 64 << 10, 32 << 10}, {(64 << 10) + 1, 0, 0}, {0, (64 << 10) + 1, 0}, {0, 0, (32 << 10) + 1}} {
		message := transitionMessage{OriginalContext: bytes.Repeat([]byte{'o'}, sizes[0]), NativeContext: bytes.Repeat([]byte{'x'}, sizes[1]), Binding: bytes.Repeat([]byte{'y'}, sizes[2])}
		packet, err := encodeTransitionMessage(message)
		if sizes[0] <= 64<<10 && sizes[1] <= 64<<10 && sizes[2] <= 32<<10 {
			if err != nil || len(packet) > transitionPacketLimit {
				t.Fatal("declared finite packet does not fit", len(packet), err)
			}
		} else if err == nil {
			t.Fatal("snapshot byte bound exceeded")
		}
	}
	for _, size := range []int{8 << 10, (8 << 10) + 1} {
		message := transitionMessage{OriginalContext: bytes.Repeat([]byte{'o'}, 64<<10), NativeContext: bytes.Repeat([]byte{'n'}, 64<<10), Binding: bytes.Repeat([]byte{'b'}, 32<<10), StartIntent: bytes.Repeat([]byte{'i'}, size)}
		packet, err := encodeTransitionMessage(message)
		if size == 8<<10 {
			if err != nil || len(packet) > transitionPacketLimit {
				t.Fatal("largest context/binding/intent tuple does not fit", len(packet), err)
			}
		} else if err == nil {
			t.Fatal("oversized original intent encoded")
		}
	}
	// Decoding is never from-birth scope, peer, nonce/sequence, journal,
	// original Sync/Close, Release or recovery authority.
}

func TestForwardingTransitionOrderlyEOFNeverHidesIndependentFailure(t *testing.T) {
	for _, flags := range []int{0, syscall.MSG_CMSG_CLOEXEC} {
		if !transitionOrderlyEOF(0, 0, flags, io.EOF, nil) || !transitionOrderlyEOF(0, 0, flags, nil, nil) {
			t.Fatal("exact orderly closure unavailable", flags)
		}
	}
	late := errors.New("late physical failure")
	for _, failure := range []error{context.Canceled, context.DeadlineExceeded, net.ErrClosed, fmt.Errorf("physical read wrapper: %w", io.EOF), errors.Join(io.EOF, late)} {
		if transitionOrderlyEOF(0, 0, 0, failure, nil) || transitionOrderlyEOF(0, 0, syscall.MSG_CMSG_CLOEXEC, failure, nil) {
			t.Fatal("independent native cause disappeared", failure)
		}
	}
	for _, shape := range [][3]int{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {0, 0, syscall.MSG_CMSG_CLOEXEC | syscall.MSG_TRUNC}, {0, 0, syscall.MSG_CMSG_CLOEXEC | syscall.MSG_CTRUNC}} {
		if transitionOrderlyEOF(shape[0], shape[1], shape[2], io.EOF, nil) {
			t.Fatal("data/control/flags supplied orderly closure", shape)
		}
	}
	if transitionOrderlyEOF(0, 0, 0, io.EOF, late) {
		t.Fatal("ancillary failure disappeared")
	}
}
