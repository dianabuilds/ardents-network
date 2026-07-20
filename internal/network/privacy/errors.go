package privacy

import (
	"errors"
	"fmt"
	"strings"
)

const (
	CodeEnvelopeMalformed          = "privacy.envelope.malformed"
	CodeEnvelopeOversized          = "privacy.envelope.oversized"
	CodeEnvelopeVersionUnsupported = "privacy.envelope.version_unsupported"
	CodeEnvelopeSuiteUnsupported   = "privacy.envelope.suite_unsupported"
	CodeEnvelopeFlagsUnsupported   = "privacy.envelope.flags_unsupported"
	CodeEnvelopeTimeInvalid        = "privacy.envelope.time_invalid"
	CodeEnvelopeExpired            = "privacy.envelope.expired"
	CodeEnvelopeAuthentication     = "privacy.envelope.authentication_failed"
	CodeEnvelopeReplayed           = "privacy.envelope.replayed"
	CodeEnvelopeSignatureInvalid   = "privacy.envelope.signature_invalid"
	CodeEnvelopeSenderUnauthorized = "privacy.envelope.sender_unauthorized"
	CodeReplayCapacityExhausted    = "privacy.replay.capacity_exhausted"
)

type Error struct {
	Code string
	err  error
}

func (e *Error) Error() string {
	if e == nil {
		return "private envelope operation failed"
	}
	return e.Code + ": private envelope operation failed"
}

func (e *Error) Unwrap() error { return e.err }

func (e *Error) FailureCode() string { return e.Code }

func CodeOf(err error) string {
	var coded interface{ FailureCode() string }
	if errors.As(err, &coded) {
		return coded.FailureCode()
	}
	var envelopeErr *Error
	if errors.As(err, &envelopeErr) {
		return envelopeErr.Code
	}
	return ""
}

func IsCapabilityFailure(err error) bool {
	return strings.HasPrefix(CodeOf(err), "privacy.capability.")
}

func envelopeError(code, detail string) error {
	return &Error{Code: code, err: fmt.Errorf("%s", detail)}
}

func CapabilityUnavailable() error {
	return envelopeError(CodeCapabilityMissing, "private channel is unavailable")
}
