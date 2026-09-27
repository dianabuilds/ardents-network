package durable

import (
	"fmt"
	"testing"
)

func testLimits() Limits {
	return Limits{EpochBytes: 1 << 20, RecordBytes: 32 << 10, ClosedProfileBytes: 64 << 10}
}

func TestRootLeaseAndGenerationSurviveReopen(t *testing.T) {
	path := t.TempDir()
	root, err := Open(path, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Open(path, testLimits()); err == nil {
		_ = other.Close()
		t.Fatal("second owner acquired the exclusive State root")
	}
	generation := Generation{Name: fmt.Sprintf("%064x", 1), Epoch: []byte("epoch"), Inputs: [][]byte{[]byte("input")}, Activate: true}
	if err := root.CommitState(generation); err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	current, values, err := reopened.LoadState()
	if err != nil || current != generation.Name || len(values) != 1 ||
		string(values[0].Epoch) != "epoch" || len(values[0].Inputs) != 1 ||
		string(values[0].Inputs[0]) != "input" {
		t.Fatalf("recovered generation: current=%q values=%+v err=%v", current, values, err)
	}
	different := generation
	different.Epoch = []byte("other")
	if err := reopened.CommitState(different); err == nil {
		t.Fatal("overwrote an immutable State generation")
	}
}
