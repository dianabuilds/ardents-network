//go:build linux

package route

import (
	"fmt"
	"io"
	"testing"
)

func TestClosedBootstrapFailureDetailPreservesIssuerTLSBoundary(t *testing.T) {
	cause := closedBootstrapFailureAt("issuer-tls", fmt.Errorf("bootstrap: %w", closedRoleOpenFailureAt("tls-handshake", io.EOF)))
	if got := ClosedBootstrapFailureDetail(cause); got != "issuer-tls-tls-handshake-eof" {
		t.Fatalf("detail = %q", got)
	}
}
