package endpoint_test

import (
	"bytes"
	"testing"

	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/connection"
)

func TestResultFrameRequiresAClassifiedBoundedOutcome(t *testing.T) {
	want := applicationconnection.Result{Class: "clean service connection close", AuthenticatedTarget: [32]byte{1},
		AcceptedBytes: 4096, ReceivedBytes: 4096}
	var frame bytes.Buffer
	if err := applicationconnection.WriteResult(&frame, want); err != nil {
		t.Fatal(err)
	}
	got, err := applicationconnection.ReadResult(&frame)
	if err != nil || got != want {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	if _, err := applicationconnection.ReadResult(bytes.NewReader(nil)); err == nil {
		t.Fatal("clean EOF was treated as semantic success")
	}
}
