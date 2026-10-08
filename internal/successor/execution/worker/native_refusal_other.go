//go:build !linux

package worker

import (
	"context"
	"errors"
	"time"
)

// There is no selected native worker on these platforms. The common real
// launch composition still consumes local admission and joins its refused
// Job, but this adapter cannot create an artifact, activation or cleanup pin.
var errNativeUnavailable = errors.New("ordinary Execution native platform is unavailable")

type Artifact struct{}
type Attachment struct{}
type Cleanup struct{}

func Activate(context.Context, string) (*Activation, error) {
	return nil, errNativeUnavailable
}

func ObserveInstance(context.Context, string, string) (Instance, error) {
	return Instance{}, errNativeUnavailable
}

func NewCleanup(Instance, func(error)) (*Cleanup, error) {
	return nil, errNativeUnavailable
}

func (*Artifact) Verify() error                 { return errNativeUnavailable }
func (*Attachment) SetDeadline(time.Time) error { return errNativeUnavailable }
func (*Attachment) Read([]byte) (int, error)    { return 0, errNativeUnavailable }
func (*Attachment) Close() error                { return errNativeUnavailable }
func (*Cleanup) Close() error                   { return errNativeUnavailable }
