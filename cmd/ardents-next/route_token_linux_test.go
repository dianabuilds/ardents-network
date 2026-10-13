//go:build linux

package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// These are genuine Route/Stock exchanges, not qualified Service operations.
// Signed permission and receiving owners are fixtures; no worker/Startup is
// supplied by the cleanup observer.
func TestFundedSourceBootstrapAndAdmittedIssuanceBothCarriers(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			x := newRouteIssuerFixture(t, carrier, nil, nil, nil, nil)
			f := x.network
			holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{8, 8, 0})
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			root := t.TempDir()
			plan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"),
				HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second),
				Work:        hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20},
				Termination: hosting.Traffic{Tx: admission.ControlClass.ByteLimit(), Rx: admission.ControlClass.ByteLimit()}}
			installation, err := selection.OpenInstallation(selection.InstallationConfig{EntryRoot: plan.EntryRoot, Current: f.current})
			if err != nil {
				t.Fatal(err)
			}
			defer installation.Close()
			selected, err := installation.Borrow(selection.RoleConfig{InteriorRoot: plan.InteriorRoot, Domain: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer selected.Close()
			budget, err := hosting.Open(plan.HostingRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer budget.Close()
			var bootstrapCleanup error
			source, leg, funding, err := openFundedSource(ctx, plan, selected, budget, f.authority, holder,
				func(err error) { bootstrapCleanup = errors.Join(bootstrapCleanup, err) })
			if err != nil {
				t.Fatal("cold genuine Source", err)
			}
			defer source.Close()
			if bootstrapCleanup != nil || x.bootstrapIssues.Load() != 2 || holder.Status().BootstrapRemaining != 0 || x.admissions.Load() == 0 {
				t.Fatal("cold Source did not join exactly two bootstrap batches before real admission", bootstrapCleanup, x.bootstrapIssues.Load(), holder.Status())
			}
			// A second actual opening needs fresh Forward presentations for the
			// same selected nodes. Obtain them through the original admitted
			// Source, including its class-specific issuer Control replacement.
			config, err := preparePublicationPrefix(ctx, plan, selected, budget, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			config.Present = funding.present
			next, err := prefix.Open(ctx, config)
			if err != nil {
				t.Fatal("genuine on-demand funded opening", errors.Join(err, config.Release()))
			}
			if err := next.Close(); err != nil {
				t.Fatal("original on-demand opening join", err)
			}
			if cleanup, joined := next.CleanupResult(); !joined || cleanup != nil {
				t.Fatal("on-demand opening lacks physical receipt", joined, cleanup)
			}
			if x.ordinaryIssues.Load() != 4 || x.bootstrapIssues.Load() != 2 || holder.Status().Remaining[0] != 3 || holder.Status().Remaining[1] != 4 {
				t.Fatal("on-demand funding changed original finite allowance", x.ordinaryIssues.Load(), holder.Status())
			}
			if err := source.Issue(ctx, plan.Deadline, routeIssuerPreparation(ctx, f.authority, holder, leg, uint8(admission.ForwardClass), nil)); err != nil {
				t.Fatal("actual admitted issuer exchange", err)
			}
			if x.ordinaryIssues.Load() != 5 || x.bootstrapIssues.Load() != 2 || holder.Status().Remaining[1] != 2 {
				t.Fatal("admitted work renewed bootstrap or lost irreversible quota", x.ordinaryIssues.Load(), x.bootstrapIssues.Load(), holder.Status())
			}
			if err := source.Close(); err != nil {
				t.Fatal("original funded Source join", err)
			}
			cleanup, joined := source.CleanupResult()
			if !joined || cleanup != nil {
				t.Fatal("Source retirement lacks original physical receipt", joined, cleanup)
			}
		})
	}
}

// A copied draft view supplies no effect authority. The genuine Stock must
// observe clock loss before reserving a batch; no actual Prefix is supplied.
func TestRouteIssuerDraftCannotBypassFreshStockObservation(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{8, 8, 0})
	before := holder.Status()
	authority := f.authorityLosingClockAfterDraft(t)
	prepare := routeIssuerPreparation(t.Context(), authority, holder, selection.Leg{}, uint8(admission.ForwardClass), [][32]byte{f.receiver.NodeID})
	batch, err := prepare(prefix.IssuerBinding{ID: [32]byte{1}, ProfileDigest: f.profile.Digest,
		Deadline: time.Now().Add(5 * time.Second).UTC().Truncate(time.Second), Bootstrap: true})
	if err == nil || len(batch.Request) != 0 || batch.Complete != nil {
		t.Fatal("old draft view bypassed fresh Stock authority", err)
	}
	after := holder.Status()
	if after.Pending || after.BootstrapRemaining != before.BootstrapRemaining || after.Remaining != before.Remaining {
		t.Fatal("clock loss reserved a blind batch or changed finite allowance", before, after)
	}
}
