package applicationipc_test

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/applicationipc"
)

func TestConnectionPreservesStage4RawBytesBeforeResult(t *testing.T) {
	want := applicationipc.Result{Class: "clean service connection close", AuthenticatedTarget: [32]byte{1}}
	data := []byte("opaque-ASRS\x01\x00\x02{}")
	left, right := net.Pipe()
	resultLeft, resultRight := net.Pipe()
	reader := applicationipc.NewConnection(right, resultRight)
	go func() {
		_, _ = left.Write(data)
		_ = applicationipc.Write(resultLeft, want)
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

func TestConnectionWritesRawBytesForStage4Peer(t *testing.T) {
	left, right := net.Pipe()
	resultLeft, resultRight := net.Pipe()
	writer := applicationipc.NewConnection(left, resultLeft)
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

func TestConnectionFailsClosedWithoutTerminalResult(t *testing.T) {
	left, right := net.Pipe()
	resultLeft, resultRight := net.Pipe()
	reader := applicationipc.NewConnection(right, resultRight)
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
	if got, err := applicationipc.ResultPath("/run/ardents/app.sock"); err != nil || got != "/run/ardents/app.sock.result" {
		t.Fatalf("result path=%q error=%v", got, err)
	}
	if _, err := applicationipc.ResultPath("/run/ardents/app.sock.result"); err == nil {
		t.Fatal("recursive result path was accepted")
	}
}
