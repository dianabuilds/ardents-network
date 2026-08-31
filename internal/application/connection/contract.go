package connection

import (
	"context"
	"errors"
	"io"
)

const (
	CleanClose         = "clean service connection close"
	ServiceUnavailable = "service unavailable"
	LocalFailure       = "local attachment failure"
)

// Outcome is the bounded terminal Connection projection. It contains no
// Target, Route, peer, credential, or Application bytes.
type Outcome struct {
	Class  string
	Reason string
}

// Stream is one authenticated opaque Application byte stream with an exact
// terminal outcome.
type Stream interface {
	io.ReadWriteCloser
	Done() <-chan Outcome
}

// Interface opens one Service Link without accepting Network or Route facts.
type Interface interface {
	Open(context.Context, string) (Stream, error)
}

// InterfaceFunc adapts one open function to Interface.
type InterfaceFunc func(context.Context, string) (Stream, error)

// Open implements Interface.
func (open InterfaceFunc) Open(ctx context.Context, serviceLink string) (Stream, error) {
	return open(ctx, serviceLink)
}

type refusalError struct{ outcome Outcome }

func (failure refusalError) Error() string {
	if failure.outcome.Reason == "" {
		return failure.outcome.Class
	}
	return failure.outcome.Class + ": " + failure.outcome.Reason
}

// Refuse returns a typed refusal for a server-side Interface implementation.
func Refuse(outcome Outcome) error {
	if outcome.Class == "" {
		return errors.New("application Connection refusal is unclassified")
	}
	return refusalError{outcome: outcome}
}

func refusal(cause error) Outcome {
	var classified refusalError
	if errors.As(cause, &classified) {
		return classified.outcome
	}
	return Outcome{Class: ServiceUnavailable, Reason: "Endpoint could not open the selected Service Link"}
}
