//go:build referencec2 && (h4_3b_multihost || h4_8_a11)

package service_test

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestH43TransferRemoteProofReturnsReadFailure(t *testing.T) {
	readFailure := errors.New("remote proof unavailable")
	wrote := false
	err := h43TransferRemoteProof(context.Background(), "remote-proof", "local-proof",
		func(context.Context, string) ([]byte, error) { return nil, readFailure },
		func(string, []byte, os.FileMode) error {
			wrote = true
			return nil
		})
	if !errors.Is(err, readFailure) || wrote {
		t.Fatalf("remote read failure = %v; wrote=%t", err, wrote)
	}
}

func TestH43TransferRemoteProofReturnsWriteFailure(t *testing.T) {
	writeFailure := errors.New("local proof destination unavailable")
	err := h43TransferRemoteProof(context.Background(), "remote-proof", "local-proof",
		func(context.Context, string) ([]byte, error) { return []byte("proof\n"), nil },
		func(string, []byte, os.FileMode) error { return writeFailure })
	if !errors.Is(err, writeFailure) {
		t.Fatalf("local write failure = %v", err)
	}
}
