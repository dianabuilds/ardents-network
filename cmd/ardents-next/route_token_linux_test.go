//go:build linux

package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
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
			x := newRouteIssuerFixture(t, carrier, nil, nil, nil, nil, true)
			f := x.network
			holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{9, 10, 0})
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
			source, leg, funding, err := openFundedSource(ctx, plan, selected, budget, f.authority, holder, 3, true,
				func(err error) { bootstrapCleanup = errors.Join(bootstrapCleanup, err) })
			if err != nil {
				t.Fatal("cold genuine Source", err)
			}
			defer source.Close()
			storeChallenge, err := funding.challenges(uint8(admission.ControlClass), [][32]byte{{77}})
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := holder.Prepare(ctx)
			if err != nil {
				t.Fatal(err)
			}
			missingStore, missingErr := prepared.Missing(storeChallenge)
			if err := errors.Join(missingErr, prepared.Close()); err != nil || len(missingStore) != 0 {
				t.Fatal("second bootstrap batch did not bind genuine Store Control", missingStore, err)
			}

			if bootstrapCleanup != nil || x.bootstrapIssues.Load() != 2 || holder.Status().BootstrapRemaining != 0 || x.admissions.Load() == 0 {
				t.Fatal("cold Source did not join exactly two bootstrap batches before real admission", bootstrapCleanup, x.bootstrapIssues.Load(), holder.Status())
			}
			// Two later openings share selected recipients but each needs its own
			// Forward presentations. One batch must preserve that multiplicity
			// without opening either leg or refilling issuer Control.
			config, err := preparePublicationPrefix(ctx, plan, selected, budget, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			second, err := preparePublicationPrefix(ctx, plan, selected, budget, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			second.Present = funding.openingPresentation(second.Leg)
			config.Present = funding.openingPresentation(config.Leg)
			if err := funding.prepareInitialOpening(ctx, config.Leg, second.Leg); err != nil {
				t.Fatal("prepare Forward before physical opening", err)
			}
			if x.ordinaryIssues.Load() != 1 {
				t.Fatal("opening began without completed Forward issuance", x.ordinaryIssues.Load())
			}
			// A separate preparation may hold the issuance gate. Already funded
			// real openings must consume their own tokens without waiting for it.
			func() {
				release, err := funding.beginPresentation(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer release()
				preparation, err := holder.Prepare(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer preparation.Close()
				for _, prepared := range []prefix.Config{config, second} {
					next, err := prefix.Open(ctx, prepared)
					if err != nil {
						t.Fatal("genuine prepared opening", errors.Join(err, prepared.Release()))
					}
					if err := next.Close(); err != nil {
						t.Fatal("original prepared opening join", err)
					}
					if cleanup, joined := next.CleanupResult(); !joined || cleanup != nil {
						t.Fatal("prepared opening lacks physical receipt", joined, cleanup)
					}
				}
			}()
			if x.ordinaryIssues.Load() != 1 || x.bootstrapIssues.Load() != 2 || holder.Status().Remaining != [3]uint32{5, 4, 0} {
				t.Fatal("two prepared openings repeated issuance or changed finite allowance", x.ordinaryIssues.Load(), holder.Status())
			}
			// Replenish the consumed rights through a separate genuine exchange.
			if err := source.Issue(ctx, plan.Deadline, routeIssuerPreparation(ctx, f.authority, holder, leg, uint8(admission.ForwardClass), nil)); err != nil {
				t.Fatal("actual admitted issuer exchange", err)
			}
			if x.ordinaryIssues.Load() != 2 || x.bootstrapIssues.Load() != 2 || holder.Status().Remaining[1] != 2 {
				t.Fatal("admitted work renewed bootstrap or lost irreversible quota", x.ordinaryIssues.Load(), x.bootstrapIssues.Load(), holder.Status())
			}
			// Exercise Stock preparation through an actual acquired Source. This
			// isolates issuance: these fixture duties do not claim Service roles.
			scope, err := prefix.NewSourceScope(ctx, source, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer scope.Close()
			handle, err := scope.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer handle.Close()
			// Forward for Entry is already stocked. Only Introduction Control
			// plus replacement of the spent issuer Control may consume quota.
			if err := funding.prepareRoute(ctx, handle, leg.Interior, leg.Entry); err != nil {
				t.Fatal("prepare existing Forward", err)
			}
			if x.ordinaryIssues.Load() != 3 || holder.Status().Remaining != [3]uint32{3, 2, 0} {
				t.Fatal("prepared Forward caused unnecessary issuer refill", x.ordinaryIssues.Load(), holder.Status())
			}
			before := holder.Status()
			if err := funding.prepareRoute(ctx, handle, leg.Interior, leg.Entry); err != nil {
				t.Fatal("reuse unpresented prepared Stock", err)
			}
			if x.ordinaryIssues.Load() != 3 || holder.Status().Remaining != before.Remaining {
				t.Fatal("preparation spent recipient Stock or repeated issuance", holder.Status())
			}
			canceled, stop := context.WithCancel(ctx)
			stop()
			if err := funding.prepareRoute(canceled, handle, leg.Interior, leg.Entry); !errors.Is(err, context.Canceled) {
				t.Fatal("canceled preparation accepted", err)
			}
			if x.ordinaryIssues.Load() != 3 || holder.Status().Remaining != before.Remaining || holder.Status().BootstrapRemaining != 0 {
				t.Fatal("canceled preparation changed finite allowance", holder.Status())
			}
			if err := funding.prepareJoin(canceled, handle, leg.Entry); !errors.Is(err, context.Canceled) {
				t.Fatal("canceled JOIN Stock preparation accepted", err)
			}
			// A recipient outside the opened leg has no Forward yet. Funding
			// must obtain the missing issuer Control and Forward, once each.
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}} {
				if id == leg.Entry.NodeID || id == leg.Interior.NodeID {
					continue
				}
				duty, err := view.RetainDuty(id, view.ObservedAt())
				if err != nil {
					t.Fatal(err)
				}
				if err := funding.prepareJoin(ctx, handle, duty); err != nil {
					t.Fatal("prepare missing Forward", err)
				}
				if x.ordinaryIssues.Load() != 5 || holder.Status().Remaining != [3]uint32{1, 1, 0} || x.bootstrapIssues.Load() != 2 {
					t.Fatal("missing Forward changed finite issuance", x.ordinaryIssues.Load(), holder.Status())
				}
				break
			}
			handle.Close()
			if err := scope.Close(); err != nil {
				t.Fatal(err)
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

// The real Network loses clock authority after validating HELLO. Take must
// retain that failure, without falling through to the deliberately occupied
// preparation or trying to obtain replacement rights.
func TestFundedPresentationDoesNotPrepareAfterAuthorityFailure(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{8, 8, 0})
	preparation, err := holder.Prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer preparation.Close()
	before := holder.Status()
	authority := f.authority
	authority.presentation = func(presentation stock.Presentation) error {
		if err := f.authority.presentation(presentation); err != nil {
			return err
		}
		f.clockUnavailable.Store(true)
		return nil
	}
	defer f.clockUnavailable.Store(false)
	flow := &routeTokenFlow{holder: holder, authority: authority, issuer: f.profile.IssuerNodeID}
	hello := ardp.Hello{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration,
		StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, RecipientNodeID: f.receiver.NodeID,
		RecipientDutyGeneration: f.receiver.DutyGeneration, ChannelNonce: [32]byte{1},
		Deadline: time.Now().Add(5 * time.Second).UTC().Truncate(time.Second), Purpose: ardp.PurposeDataJoin}
	raw, err := flow.present(t.Context(), hello)
	if !f.clockUnavailable.Load() || len(raw) != 0 || err == nil || err.Error() == "stock preparation unavailable" {
		t.Fatal("fresh Stock authority failure reached preparation or returned bytes", err)
	}
	after := holder.Status()
	if after.Pending || after.Remaining != before.Remaining || after.BootstrapRemaining != before.BootstrapRemaining {
		t.Fatal("authority failure changed the original finite allocation", before, after)
	}
}
