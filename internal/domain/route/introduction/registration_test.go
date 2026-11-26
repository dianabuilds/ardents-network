package introduction

import (
	"context"
	"testing"
	"time"
)

// No constructor without a genuine Prefix can acquire a receiving slot.
// Successful REGISTER/WITHDRAW remains genuine both-Carrier command composition.
func TestRegistrationWithoutPrefixCannotAcquireSlot(t *testing.T) {
	caller, cancel := context.WithCancel(t.Context())
	cancel()
	for _, ctx := range []context.Context{nil, caller, t.Context()} {
		r, err := Register(ctx, nil, RegistrationConfig{Revision: 1, Deadline: time.Now().Add(time.Minute).UTC().Truncate(time.Second)})
		if r != nil || err == nil {
			t.Fatal("missing physical Prefix created registration", r, err)
		}
	}
}
