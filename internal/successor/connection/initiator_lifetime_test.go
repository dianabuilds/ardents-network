package connection

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

// An unusable Operation isolates refusal before any accepting owner check.
// This fixture cannot construct authenticated state or perform Service effects.
func TestInitiatorAttachmentRetirementRetainsOriginalCause(t *testing.T) {
	for _, state := range []struct {
		name             string
		closed, canceled bool
		expected         error
	}{
		{"closed", true, false, net.ErrClosed},
		{"canceled", false, true, context.Canceled},
		{"closed-canceled", true, true, context.Canceled},
	} {
		t.Run(state.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if state.canceled {
				cancel()
			}
			attachment := &initiatorAttachment{ctx: ctx, closed: state.closed,
				binding: &InitiatorBinding{operation: &executionruntime.Operation{}, setup: ctx}}
			if err := attachment.checkLocked(); !errors.Is(err, state.expected) || errors.Is(err, errIntegrity) {
				t.Fatal("ordinary Attachment retirement invented integrity failure", err)
			}
		})
	}
	if err := (&initiatorAttachment{}).checkLocked(); !errors.Is(err, errIntegrity) {
		t.Fatal("missing Attachment owners lost structural refusal", err)
	}
}

// Refusal-only local-state fixtures provide no successful Operation, Source
// or authentication. Original lifetime loss must not invent tampered identity.
func TestInitiatorLocalRetirementRetainsCauseWithoutIntegrityContradiction(t *testing.T) {
	originalFailure := errors.New("original Source work failure")
	for _, state := range []struct {
		name              string
		closed, canceled  bool
		failure, expected error
	}{
		{"closed", true, false, nil, net.ErrClosed},
		{"canceled", false, true, nil, context.Canceled},
		{"closed-canceled", true, true, nil, context.Canceled},
		{"failed", false, false, originalFailure, originalFailure},
		{"failed-canceled", true, true, originalFailure, originalFailure},
		{"integrity", false, false, errIntegrity, errIntegrity},
	} {
		t.Run(state.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if state.canceled {
				cancel()
			}
			source := &InitiatorSource{ctx: ctx, closed: state.closed, failure: state.failure}
			err := source.localLocked()
			if !errors.Is(err, state.expected) || (errors.Is(err, errIntegrity) && state.failure != errIntegrity) {
				t.Fatal("local retirement replaced or invented its original cause", err)
			}
			if state.canceled && !errors.Is(err, context.Canceled) {
				t.Fatal("retained failure erased original cancellation", err)
			}
		})
	}
}

// Context topology is a mechanism fixture, never successful Job, JOIN, TLS or
// replacement authority. It isolates the lifetime used by real authentication.
func TestInitiatorPhysicalRetirementPreservesOriginalLogicalBound(t *testing.T) {
	original, cancelOriginal := context.WithCancel(t.Context())
	defer cancelOriginal()
	end := time.Now().Add(time.Minute)
	logical, cancelLogical, physical, cancelPhysical := initiatorLifetimes(original, end)
	defer cancelLogical()
	defer cancelPhysical()
	for _, lifetime := range []context.Context{logical, physical} {
		if actual, exists := lifetime.Deadline(); !exists || actual != end {
			t.Fatal("original Work Safety bound changed")
		}
	}
	cancelPhysical()
	if !errors.Is(physical.Err(), context.Canceled) || logical.Err() != nil {
		t.Fatal("physical retirement canceled original logical lifetime")
	}
	// A gap cannot supply a new deadline, and original caller loss still ends
	// the logical lifetime even though no physical users remain.
	if actual, _ := logical.Deadline(); actual != end {
		t.Fatal("physical gap renewed original bound")
	}
	cancelOriginal()
	if !errors.Is(logical.Err(), context.Canceled) {
		t.Fatal("original caller loss left logical lifetime accepting")
	}
}

func TestInitiatorLogicalRetirementInterruptsPhysicalUsers(t *testing.T) {
	logical, cancelLogical, physical, cancelPhysical := initiatorLifetimes(t.Context(), time.Now().Add(time.Minute))
	defer cancelPhysical()
	cancelLogical()
	if !errors.Is(logical.Err(), context.Canceled) || !errors.Is(physical.Err(), context.Canceled) {
		t.Fatal("logical retirement left physical lifetime accepting")
	}
}
