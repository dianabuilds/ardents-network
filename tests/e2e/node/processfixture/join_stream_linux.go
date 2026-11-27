//go:build linux && (node_process_native || join_platform_native)

package processfixture

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
)

// TransferJoinDirection checks exact delivery of more than one credit window.
func TransferJoinDirection(t *testing.T, from, to net.Conn, value byte) {
	t.Helper()
	payload := bytes.Repeat([]byte{value}, 96<<10)
	written := make(chan error, 1)
	go func() {
		n, err := from.Write(payload)
		if err == nil && n != len(payload) {
			err = io.ErrShortWrite
		}
		written <- err
	}()
	got := make([]byte, len(payload))
	_, err := io.ReadFull(to, got)
	if err = errors.Join(err, <-written); err != nil || !bytes.Equal(got, payload) {
		t.Fatal("native JOIN duplex byte oracle", err)
	}
}

// FinishJoinDirection checks that an authenticated half-close yields EOF.
func FinishJoinDirection(t *testing.T, from, to net.Conn) {
	t.Helper()
	writer, ok := from.(interface{ CloseWrite() error })
	if !ok {
		t.Fatal("joined stream cannot end one direction")
	}
	if err := writer.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if n, err := to.Read(one[:]); n != 0 || err != io.EOF {
		t.Fatal("directional EOF", n, err)
	}
}
