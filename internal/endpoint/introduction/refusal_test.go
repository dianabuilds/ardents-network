//go:build linux

package introduction

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/client"
)

func TestTextIntroductionRefusalCannotHideTerminalFailure(t *testing.T) {
	refusal := NewRefusal(errors.New("invalid capsule"))
	for _, err := range []error{refusal, errors.Join(refusal), errors.Join(refusal, refusal)} {
		if !OnlyRefusal(err) {
			t.Fatal("input refusal lost its scope")
		}
	}
	for _, err := range []error{nil, context.Canceled, errors.Join(refusal, context.Canceled),
		errors.Join(errors.Join(refusal), client.ErrClosedSourceCleanup), errors.Join(refusal, errors.New("acknowledgement failed"))} {
		if OnlyRefusal(err) {
			t.Fatalf("terminal failure treated as input refusal: %v", err)
		}
	}
}
