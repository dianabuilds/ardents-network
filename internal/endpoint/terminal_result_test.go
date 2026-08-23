package endpoint_test

import (
	"bytes"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
)

func TestResultFrameRequiresAClassifiedBoundedOutcome(t *testing.T) {
	want := endpoint.Result{Class: "clean service connection close", AuthenticatedTarget: [32]byte{1},
		AcceptedBytes: 4096, ReceivedBytes: 4096}
	var frame bytes.Buffer
	if err := endpoint.Write(&frame, want); err != nil {
		t.Fatal(err)
	}
	got, err := endpoint.Read(&frame)
	if err != nil || got != want {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	if _, err := endpoint.Read(bytes.NewReader(nil)); err == nil {
		t.Fatal("clean EOF was treated as semantic success")
	}
}
