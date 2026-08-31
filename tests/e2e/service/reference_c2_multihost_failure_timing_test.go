//go:build referencec2 && (h4_3b_multihost || h4_8_a11)

package service_test

import (
	"context"
	"testing"
)

func TestH43AbortProofAfterUserFailureCancelsAndWaits(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	proof := make(chan error)
	finished := make(chan struct{})
	go func() {
		<-ctx.Done()
		proof <- ctx.Err()
		close(finished)
	}()

	h43AbortProofAfterUserFailure(cancel, proof)
	select {
	case <-finished:
	case <-t.Context().Done():
		t.Fatal("cancelled proof did not finish")
	}
}
