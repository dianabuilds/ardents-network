package endpoint

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

func TestApplicationConnectionPreservesRawBytesAfterHandshake(t *testing.T) {
	want := Result{Class: "clean service connection close", AuthenticatedTarget: [32]byte{1}}
	data := []byte("opaque-ASRS\x01\x00\x02{}")
	reader, left, resultLeft := openApplicationPair(t)
	go func() {
		_, _ = left.Write(data)
		_ = Write(resultLeft, want)
		_ = left.Close()
		_ = resultLeft.Close()
	}()
	got := make([]byte, len(data))
	_, dataErr := io.ReadFull(reader, got)
	result, resultErr := reader.Result()
	if dataErr != nil || resultErr != nil || !bytes.Equal(got, data) || result != want {
		t.Fatalf("data=%q result=%+v errors=%v/%v", got, result, dataErr, resultErr)
	}
}

func TestApplicationConnectionWritesRawBytesAfterHandshake(t *testing.T) {
	writer, right, resultRight := openApplicationPair(t)
	defer resultRight.Close()
	request := bytes.Repeat([]byte{7}, 512)
	written := make(chan error, 1)
	go func() { _, err := writer.Write(request); written <- err }()
	_ = right.SetReadDeadline(time.Now().Add(time.Second))
	got := make([]byte, len(request))
	if _, err := io.ReadFull(right, got); err != nil || !bytes.Equal(got, request) {
		t.Fatalf("raw request was changed or withheld: bytes=%d error=%v", len(got), err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	_ = right.Close()
}

func TestApplicationConnectionFailsClosedWithoutTerminalResult(t *testing.T) {
	reader, left, resultLeft := openApplicationPair(t)
	go func() { _, _ = left.Write([]byte("opaque-only")); _ = left.Close() }()
	got := make([]byte, len("opaque-only"))
	if _, err := io.ReadFull(reader, got); err != nil || string(got) != "opaque-only" {
		t.Fatalf("data=%q error=%v", got, err)
	}
	_ = resultLeft.Close()
	if _, err := reader.Result(); err == nil {
		t.Fatal("missing terminal frame was accepted")
	}
}

func TestResultPathIsDerivedOnce(t *testing.T) {
	if got, err := ResultPath("/run/ardents/app.sock"); err != nil || got != "/run/ardents/app.sock.result" {
		t.Fatalf("result path=%q error=%v", got, err)
	}
	if _, err := ResultPath("/run/ardents/app.sock.result"); err == nil {
		t.Fatal("recursive result path was accepted")
	}
}

func openApplicationPair(t *testing.T) (*connection, net.Conn, net.Conn) {
	t.Helper()
	application, peer := net.Pipe()
	result, resultPeer := net.Pipe()
	accepted := make(chan error, 1)
	go func() { accepted <- acceptApplication(peer, time.Now().Add(time.Second)) }()
	stream, err := OpenApplication(application, result)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-accepted; err != nil {
		t.Fatal(err)
	}
	return stream, peer, resultPeer
}
