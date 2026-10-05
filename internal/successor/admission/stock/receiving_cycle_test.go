//go:build linux

package stock

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func receiverFixture(h *spendHostFixture, p Presentation) receiving.Observation {
	return receiving.Observation{Profile: h.profile, Receiver: receiving.Receiver{NetworkID: p.NetworkID, StateGeneration: p.StateGeneration, StateDigest: p.StateDigest, ProfileDigest: p.ProfileDigest, NodeID: p.RecipientNodeID, DutyGeneration: p.RecipientDutyGeneration}, Now: h.now, NotAfter: h.profile.NotAfter}
}

func TestReceivingAllowanceCannotOutliveProfile(t *testing.T) {
	stock, h, p := issuedStockFixture(t)
	raw, err := stock.Take(context.Background(), p, 2)
	if err != nil {
		t.Fatal(err)
	}
	v := receiverFixture(h, p)
	v.Now = v.Profile.NotAfter.Add(-time.Minute)
	v.NotAfter = v.Profile.NotAfter.Add(time.Hour)
	owner, err := receiving.Open(t.TempDir(), v.Receiver, func() (receiving.Observation, error) { return v, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	grant, err := owner.Accept(context.Background(), admission.ForwardClass, raw, v.NotAfter, func() (func() error, error) { return func() error { return nil }, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer grant.Release()
	if grant.Allowance().Deadline() != v.Profile.NotAfter {
		t.Fatal("allowance outlived profile", grant.Allowance().Deadline())
	}
}

func TestReceivingShrinkingBoundRefusesBeforeAndAfterSpend(t *testing.T) {
	for _, afterSpend := range []bool{false, true} {
		t.Run(map[bool]string{false: "reservation", true: "spend"}[afterSpend], func(t *testing.T) {
			// Keep the initial bound beyond Now+1s even in the final second
			// of a token hour, so this observation always shortens it.
			stock, h, p := issuedStockFixtureHours(t, 2)
			raw, err := stock.Take(context.Background(), p, 2)
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			initial := receiverFixture(h, p)
			reserved, released := false, 0
			observe := func() (receiving.Observation, error) {
				v := initial
				shorten := reserved
				if afterSpend {
					info, err := os.Stat(filepath.Join(root, "closed-token-spends"))
					shorten = err == nil && info.Size() > 112
				}
				if shorten {
					v.NotAfter = v.Now.Add(time.Second)
				}
				return v, nil
			}
			owner, err := receiving.Open(root, initial.Receiver, observe)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			reserve := func() (func() error, error) { reserved = true; return func() error { released++; return nil }, nil }
			if _, err := owner.Accept(context.Background(), admission.ForwardClass, raw, initial.Now.Add(time.Minute), reserve); err == nil {
				t.Fatal("shortened terminal bound accepted")
			}
			if released != 1 {
				t.Fatal("reservation not released", released)
			}
			info, err := os.Stat(filepath.Join(root, "closed-token-spends"))
			if err != nil || (info.Size() > 112) != afterSpend {
				t.Fatal("unexpected spend boundary", err)
			}
		})
	}
}
func TestReceivingAuthorityFailureAfterSpendRetainsBurnAndReleasesCapacity(t *testing.T) {
	stock, h, p := issuedStockFixture(t)
	raw, err := stock.Take(context.Background(), p, 2)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	initial := receiverFixture(h, p)
	invalidate := true
	observe := func() (receiving.Observation, error) {
		v := initial
		info, err := os.Stat(filepath.Join(root, "closed-token-spends"))
		if err == nil && info.Size() > 112 && invalidate {
			return v, errors.New("authority lost after durable spend")
		}
		return v, nil
	}
	owner, err := receiving.Open(root, initial.Receiver, observe)
	if err != nil {
		t.Fatal(err)
	}
	reserved, released := 0, 0
	reserve := func() (func() error, error) { reserved++; return func() error { released++; return nil }, nil }
	if _, err := owner.Accept(context.Background(), admission.ForwardClass, raw, h.now.Add(time.Minute), reserve); err == nil {
		t.Fatal("post-commit authority loss accepted")
	}
	if reserved != 1 || released != 1 {
		t.Fatal("reservation leaked", reserved, released)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	invalidate = false
	owner, err = receiving.Open(root, initial.Receiver, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := owner.Accept(context.Background(), admission.ForwardClass, raw, h.now.Add(time.Minute), reserve); err == nil {
		t.Fatal("post-commit failure refunded token")
	}
	if reserved != 2 || released != 2 {
		t.Fatal("repeat refusal leaked reservation")
	}
}

func TestReceivingRefillPreservesDeadlineAndTransferredReservations(t *testing.T) {
	stock, h, p := issuedStockFixture(t)
	a, err := stock.Begin(nextIntent(h))
	if err != nil {
		t.Fatal(err)
	}
	request, _, err := a.Request()
	if err != nil {
		t.Fatal(err)
	}
	result := issuer.IssueCurrent(context.Background(), h.plan, request, quota.Bootstrap, h.ProfileLocked)
	if result.Outcome != "issued-offline" {
		t.Fatal(result.Outcome)
	}
	if err := a.Complete(result.Response, nil); err != nil {
		t.Fatal(err)
	}
	first, err := stock.Take(context.Background(), p, 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := stock.Take(context.Background(), p, 2)
	if err != nil {
		t.Fatal(err)
	}
	observation := receiverFixture(h, p)
	owner, err := receiving.Open(t.TempDir(), observation.Receiver, func() (receiving.Observation, error) { return observation, nil })
	if err != nil {
		t.Fatal(err)
	}
	released := 0
	reserve := func() (func() error, error) { return func() error { released++; return nil }, nil }
	original, err := owner.Accept(context.Background(), admission.ForwardClass, first, h.now.Add(time.Minute), reserve)
	if err != nil {
		t.Fatal(err)
	}
	observation.Now = observation.Now.Add(time.Second)
	refill, err := owner.Refill(context.Background(), original, 1, second, reserve)
	if err != nil {
		t.Fatal(err)
	}
	if refill.Allowance().Deadline() != original.Allowance().Deadline() || refill.Allowance().Bytes() != admission.ForwardClass.ByteLimit() {
		t.Fatal("refill changed deadline/allowance")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if released != 0 {
		t.Fatal("closing authority released admitted work")
	}
	if err := original.Release(); err != nil {
		t.Fatal(err)
	}
	if err := original.Release(); err != nil {
		t.Fatal(err)
	}
	if released != 1 {
		t.Fatal("copied grant release not idempotent")
	}
	if err := refill.Release(); err != nil {
		t.Fatal(err)
	}
	if released != 2 {
		t.Fatal("refill release lost")
	}
}
