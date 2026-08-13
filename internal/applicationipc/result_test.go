package applicationipc_test

import (
	"bytes"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/applicationipc"
)

func TestResultFrameRequiresAClassifiedBoundedOutcome(t *testing.T) {
	want := applicationipc.Result{Class: "clean service connection close", AuthenticatedTarget: [32]byte{1},
		AcceptedBytes: 4096, ReceivedBytes: 4096}
	var frame bytes.Buffer
	if err := applicationipc.Write(&frame, want); err != nil {
		t.Fatal(err)
	}
	got, err := applicationipc.Read(&frame)
	if err != nil || got != want {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	if _, err := applicationipc.Read(bytes.NewReader(nil)); err == nil {
		t.Fatal("clean EOF was treated as semantic success")
	}
}
