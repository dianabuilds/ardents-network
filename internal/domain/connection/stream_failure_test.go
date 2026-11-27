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

func TestRecoveryFailureKeepsOriginalAttachmentCause(t *testing.T) {
	original := errors.New("original attachment refused")
	err := recoveryFailure(original, errRecoveryExhausted)
	if !errors.Is(err, original) || !errors.Is(err, errRecoveryExhausted) {
		t.Fatal(err)
	}
}
