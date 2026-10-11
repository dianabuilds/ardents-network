package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
)

func TestUnstartedWriteRetainsFailureWithoutPeerCategory(t *testing.T) {
	original := fmt.Errorf("original socket: %w", net.ErrClosed)
	marked := MarkUnstartedWrite(original)
	if !IsUnstartedWrite(marked) || !errors.Is(marked, original) || IsPeerRetirementCause(marked) {
		t.Fatalf("write refusal lost failure or minted peer completion: %v", marked)
	}
	for _, invalid := range []error{net.ErrClosed, io.EOF, errors.Join(marked, net.ErrClosed),
		MarkUnstartedWrite(errors.Join(net.ErrClosed, errors.New("other failure"))), MarkUnstartedWrite(io.EOF)} {
		if IsUnstartedWrite(invalid) {
			t.Fatalf("unproven native output acquired refusal provenance: %v", invalid)
		}
	}
}
