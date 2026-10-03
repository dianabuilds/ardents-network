//go:build linux

package stock

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
	"testing"
	"time"
)

type stockHostFixture struct {
	profile admission.AuthorityFacts
	now     time.Time
}

func (h *stockHostFixture) ProfileLocked() (admission.AuthorityFacts, time.Time, error) {
	return h.profile, h.now, nil
}
func fixtureID(v byte) (id [32]byte) { id[0] = v; return }

func TestPermissionIssuancePresentationAndDurableSpend(t *testing.T) {
	owner, host, presentation := issuedStockFixture(t)
	raw, err := owner.Take(context.Background(), presentation, 2)
	if err != nil {
		t.Fatal(err)
	}
	receiver := receiving.Receiver{NetworkID: presentation.NetworkID, StateGeneration: presentation.StateGeneration, StateDigest: presentation.StateDigest, ProfileDigest: presentation.ProfileDigest, NodeID: presentation.RecipientNodeID, DutyGeneration: presentation.RecipientDutyGeneration}
	hour, err := receiving.VerifyToken(host.profile, receiver, admission.ForwardClass, raw, host.now)
	if err != nil {
		t.Fatal(err)
	}
	wrong := receiver
	wrong.DutyGeneration++
	if _, err := receiving.VerifyToken(host.profile, wrong, admission.ForwardClass, raw, host.now); err == nil {
		t.Fatal("foreign generation accepted")
	}
	if _, err := receiving.VerifyToken(host.profile, receiver, admission.ForwardClass, raw, host.profile.NotAfter); err == nil {
		t.Fatal("expired authority accepted")
	}
	root := t.TempDir()
	binding := spending.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration}
	ledger, err := spending.Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	request := receiving.Redemption{Class: admission.ForwardClass, Token: raw, Deadline: host.now.Add(time.Minute)}
	authorize := func() (receiving.Approval, error) { return receiving.Approval{Window: hour}, nil }
	if _, err := receiving.Redeem(request, ledger, func() time.Time { return host.now }, authorize, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := receiving.Redeem(request, ledger, func() time.Time { return host.now }, authorize, nil); err == nil {
		t.Fatal("double spend accepted")
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	ledger, err = spending.Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	if _, err := receiving.Redeem(request, ledger, func() time.Time { return host.now }, authorize, nil); err == nil {
		t.Fatal("restart refunded spend")
	}
	if _, err := owner.Take(context.Background(), presentation, 2); err == nil {
		t.Fatal("presented stock reused")
	}
}
