//go:build linux

package document_test

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	"testing"
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
		if got := document.Failure(errors.Join(refusal, test.contextErr)); got != test.want {
			t.Fatalf("local context diagnostic = %q, want %q", got, test.want)
		}
	}
}
