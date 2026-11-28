//go:build linux

package localread_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/document/localread"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

func TestDocumentReadLocalContextDiagnosticPrecedesDeliveredRefusal(t *testing.T) {
	refusal := endpointstream.Refuse(endpointstream.Outcome{Class: endpointstream.CapacityUnavailable, Reason: "private fixture detail"})
	for _, test := range []struct {
		contextErr error
		want       string
	}{
		{context.Canceled, "read cancelled"},
		{context.DeadlineExceeded, "read timed out"},
	} {
		if got := localread.Failure(errors.Join(refusal, test.contextErr)); got != test.want {
			t.Fatalf("local context diagnostic = %q, want %q", got, test.want)
		}
	}
}
