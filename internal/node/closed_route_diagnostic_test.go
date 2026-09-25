package node

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
)

func TestClosedRouteDiagnosticCauseIsFixed(t *testing.T) {
	for name, cause := range map[string]error{
		"canceled": context.Canceled,
		"deadline": context.DeadlineExceeded,
		"eof":      io.EOF,
		"closed":   net.ErrClosed,
		"other":    errors.New("opaque"),
	} {
		if got := closedRouteDiagnosticCause(cause); got != name {
			t.Fatalf("cause %s = %q", name, got)
		}
	}
}

func TestClosedIssuerInnerTLSFailureReasonDoesNotExposeCause(t *testing.T) {
	if got := closedIssuerInnerTLSFailureReason(io.EOF); got != "issuer-inner-tls-eof" {
		t.Fatalf("reason = %q", got)
	}
	if got := closedIssuerInnerTLSFailureReason(errors.New("private TLS detail")); got != "issuer-inner-tls-other" {
		t.Fatalf("opaque reason = %q", got)
	}
}

func TestClosedForwardingOpenFailureStageIsFixed(t *testing.T) {
	err := closedForwardingOpenFailureAt("carrier", errors.New("opaque"))
	if got := closedForwardingOpenFailureStage(err); got != "carrier" {
		t.Fatalf("stage = %q", got)
	}
}
