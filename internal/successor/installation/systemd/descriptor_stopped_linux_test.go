package systemd

import (
	"context"
	"errors"
	"testing"
)

func TestDescriptorStoppedEmptyCustodyRefusesEveryOperation(t *testing.T) {
	for _, h := range []*DescriptorStopped{nil, {}, {original: &descriptorStopped{}}} {
		if _, err := h.CheckAbsent(); !errors.Is(err, ErrInput) {
			t.Fatal(err)
		}
		if err := h.CheckOriginal(); !errors.Is(err, ErrInput) {
			t.Fatal(err)
		}
		if err := h.Reload(); !errors.Is(err, ErrInput) {
			t.Fatal(err)
		}
		if _, _, err := h.ObserveLoaded(); !errors.Is(err, ErrInput) {
			t.Fatal(err)
		}
		if _, err := h.ManagerIdentity(); !errors.Is(err, ErrInput) {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, caller := range []context.Context{nil, ctx} {
		if h, err := OpenDescriptorStopped(caller); h != nil || err == nil || caller == nil && !errors.Is(err, ErrInput) {
			_ = h.Close()
			t.Fatal("unavailable caller acquired manager custody", h, err)
		}
	}
	if (*DescriptorStopped)(nil).Close() != nil || new(DescriptorStopped).Close() != nil {
		t.Fatal("empty completion failed")
	}
}

func TestDescriptorStoppedReplyVocabularyRefusesOtherUnitsAndAllJobs(t *testing.T) {
	for _, object := range []string{"/", "/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eservice", "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice", "/org/freedesktop/systemd1/unit/ardents_2dissuer_2eservice", "/org/freedesktop/systemd1/unit/ardents_2djoin_2eservice", "/org/freedesktop/systemd1/unit/ardents_2dintroduction_2eservice", "/org/freedesktop/systemd1/job/1", "/org/freedesktop/systemd1/job/4294967295", "/org/freedesktop/systemd1/unit/ardents_2ddescriptor_2eservice"} {
		frame := sourceObjectReplyVector(":1.4", object)
		for _, sourceJobs := range []bool{false, true} {
			for _, forwardingJobs := range []bool{false, true} {
				err := verifyClosedManagerFrame(frame, ":1.4", true, sourceJobs, forwardingJobs, descriptorUnitReply)
				wanted := object == "/org/freedesktop/systemd1/unit/ardents_2ddescriptor_2eservice"
				if (err == nil) != wanted {
					t.Fatal("fixed stopped Descriptor vocabulary changed", object, err)
				}
			}
		}
		if verifyClosedManagerFrame(frame, ":1.5", true, false, false, descriptorUnitReply) == nil {
			t.Fatal("foreign manager supplied an observation")
		}
	}
}
