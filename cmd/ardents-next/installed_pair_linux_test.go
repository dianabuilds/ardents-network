//go:build linux && document_worker_installed

package main

import (
	"bytes"

	"encoding/binary"

	"encoding/json"
	"io"

	"net"

	"testing"
)

// This is a local test-controller exchange, not an Ardents protocol or authority.
// It carries public signed inputs, actual permission requests/responses and
// explicit fixture commands. No private key, live owner or accepting flag is sent.
type installedPairMessage struct {
	Operation  string
	Role       string
	Nonce      string
	Network    *installedStartupNetwork
	Body       []byte
	Link       string
	Failure    string
	Original   [3]string
	Principals [3]string
}

func installedPairSend(t *testing.T, conn net.Conn, value installedPairMessage) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 4<<20 {
		t.Fatal("invalid pair fixture message", err)
	}
	packet := binary.BigEndian.AppendUint32(nil, uint32(len(raw)))
	packet = append(packet, raw...)
	if _, err := io.Copy(conn, bytes.NewReader(packet)); err != nil {
		t.Fatal("pair fixture output", err)
	}
}

func installedPairReceive(t *testing.T, conn net.Conn) installedPairMessage {
	t.Helper()
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		t.Fatal("pair fixture input", err)
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > 4<<20 {
		t.Fatal("pair fixture message bound")
	}
	raw := make([]byte, size)
	if _, err := io.ReadFull(conn, raw); err != nil {
		t.Fatal(err)
	}
	var value installedPairMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatal("pair fixture trailing input", err)
	}
	return value
}
