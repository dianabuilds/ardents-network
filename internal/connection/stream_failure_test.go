package connection

import (
	"errors"
	"io"
	"testing"
)

func TestStreamFailureReasonRetainsOriginalLossWithoutPrivateError(t *testing.T) {
	err := recoveryFailure(errors.Join(io.ErrUnexpectedEOF, errors.New("private value")), errRecoveryExhausted)
	if got := StreamFailureReason(err); got != "Service Connection recovery bound reached after the original attachment was truncated" {
		t.Fatal(got)
	}
}
