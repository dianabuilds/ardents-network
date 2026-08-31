package connection

import (
	"context"
	"errors"
	"io"
	"unicode/utf8"
)

// InterfaceVersion is the stable local protocol family shared by Network and
// replaceable Applications. Existing AAI2 frame bytes remain unchanged.
const InterfaceVersion = "ardents-application-interface-v1"

const (
	maximumOutcomeClassBytes  = 128
	maximumOutcomeReasonBytes = 512
)

const (
	CleanClose           OutcomeClass = "clean service connection close"
	ServiceUnavailable   OutcomeClass = "service unavailable"
	LocalFailure         OutcomeClass = "local attachment failure"
	LocalCancellation    OutcomeClass = "local cancellation"
	LocalTimeout         OutcomeClass = "local timeout or cancellation"
	IndeterminateFailure OutcomeClass = "indeterminate failure"
)

// OutcomeClass is one bounded terminal classification. The shared constants
// cover transport-owned results; an Endpoint implementation may add a
// domain-owned class, but the local transport admits only 1..128 bytes.
type OutcomeClass string

// Outcome is the sole terminal Connection projection. Reason is diagnostic,
// limited to 512 bytes by the local transport, and never carries a Target,
// Route, peer, credential, or Application bytes.
type Outcome struct {
	Class  OutcomeClass
	Reason string
}

// Stream is one authenticated opaque Application byte stream. Read and Write
// may proceed concurrently; the transport splits writes into frames of at
// most 16 KiB. The implementation must close the read direction and publish
// exactly one non-empty Done result when the Service Connection terminates.
// Close cancels only this attachment and must be safe after Done. Callers must
// not interpret EOF without the Done result as semantic success.
type Stream interface {
	io.ReadWriteCloser
	Done() <-chan Outcome
}

// Interface opens one Service Link without accepting Network or Route facts.
// The link is non-empty and at most 512 bytes. The context governs both setup
// and the returned Stream lifetime. An implementation may return Refuse for a
// classified denial; every other error is exposed as ServiceUnavailable by
// the server Adapter. No operation retries or opens an alternate Service Link.
type Interface interface {
	Open(context.Context, string) (Stream, error)
}

type refusalError struct{ outcome Outcome }

func (failure refusalError) Error() string {
	if failure.outcome.Reason == "" {
		return string(failure.outcome.Class)
	}
	return string(failure.outcome.Class) + ": " + failure.outcome.Reason
}

// Refuse returns a typed refusal for a server-side Interface implementation.
func Refuse(outcome Outcome) error {
	if err := validOutcome(outcome); err != nil {
		return err
	}
	return refusalError{outcome: outcome}
}

func validOutcome(outcome Outcome) error {
	if len(outcome.Class) == 0 || len(outcome.Class) > maximumOutcomeClassBytes ||
		len(outcome.Reason) > maximumOutcomeReasonBytes ||
		!utf8.ValidString(string(outcome.Class)) || !utf8.ValidString(outcome.Reason) {
		return errors.New("application Connection terminal outcome is invalid")
	}
	return nil
}

func refusal(cause error) Outcome {
	var classified refusalError
	if errors.As(cause, &classified) {
		return classified.outcome
	}
	return Outcome{Class: ServiceUnavailable, Reason: "Endpoint could not open the selected Service Link"}
}
