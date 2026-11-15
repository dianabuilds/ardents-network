package systemd

import (
	"context"
	"errors"
	"testing"
)

// These checks exercise only local admission; they do not impersonate a manager.
func TestFixedUnitControlOriginalAttemptAndIndependentRoles(t *testing.T) {
	for _, purpose := range []fixedUnitPurpose{joinControlPurpose, descriptorControlPurpose, introductionControlPurpose, forwardingControlPurpose, sourceControlPurpose} {
		ctx, cancel := context.WithCancel(t.Context())
		first := &fixedUnitControl{reference: &managerReference{original: ctx}, purpose: purpose}
		other := &fixedUnitControl{reference: &managerReference{original: t.Context()}, purpose: purpose}
		if err := first.admitStart(); err != nil {
			t.Fatal(err)
		}
		if err := first.admitStart(); !errors.Is(err, ErrObservation) {
			t.Fatal("second start accepted", err)
		}
		if err := other.admitStart(); err != nil {
			t.Fatal("independent control inherited attempt", err)
		}
		cancel()
		if err := first.admitStop(context.WithoutCancel(ctx)); err != nil {
			t.Fatal("original cleanup denied", err)
		}
		if err := first.admitStop(t.Context()); !errors.Is(err, ErrObservation) {
			t.Fatal("second cleanup accepted", err)
		}
		if err := first.admitStart(); !errors.Is(err, context.Canceled) {
			t.Fatal("cleanup restored caller", err)
		}
	}
	if fixedUnitPurpose(0).unit() != "" || fixedUnitPurpose(99).unit() != "" {
		t.Fatal("unknown purpose gained a unit")
	}
	if joinControlPurpose.unit() == descriptorControlPurpose.unit() || descriptorControlPurpose.unit() == introductionControlPurpose.unit() || joinControlPurpose.unit() == introductionControlPurpose.unit() {
		t.Fatal("role units collapsed")
	}
	for _, tc := range []struct {
		purpose           fixedUnitPurpose
		control, readonly managerUnitReply
	}{{joinControlPurpose, joinControlReply, joinUnitReply}, {descriptorControlPurpose, descriptorControlReply, descriptorUnitReply}, {introductionControlPurpose, introductionControlReply, introductionUnitReply}} {
		if !tc.purpose.matches(tc.control, true) || tc.purpose.matches(tc.readonly, true) || !tc.purpose.matches(tc.readonly, false) || tc.purpose.matches(nodeSlotsReply, false) {
			t.Fatal("manager reply vocabulary crossed control boundary", tc)
		}
	}
	if joinControlPurpose.matches(descriptorControlReply, true) || descriptorControlPurpose.matches(joinUnitReply, false) || sourceControlPurpose.matches(forwardingUnitReply, true) {
		t.Fatal("foreign unit vocabulary accepted")
	}
}
