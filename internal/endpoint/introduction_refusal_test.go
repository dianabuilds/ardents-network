//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/client"
)

func TestTextIntroductionRefusalCannotHideTerminalFailure(t *testing.T) {
	refusal := &introductionRefusal{cause: errors.New("invalid capsule")}
	for _, err := range []error{refusal, errors.Join(refusal), errors.Join(refusal, refusal)} {
		if !onlyIntroductionRefusal(err) {
			t.Fatal("input refusal lost its scope")
		}
	}
	for _, err := range []error{nil, context.Canceled, errors.Join(refusal, context.Canceled),
		errors.Join(errors.Join(refusal), client.ErrClosedSourceCleanup), errors.Join(refusal, errors.New("acknowledgement failed"))} {
		if onlyIntroductionRefusal(err) {
			t.Fatalf("terminal failure treated as input refusal: %v", err)
		}
	}
}
