//go:build linux

package node_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/domain/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/domain/route/selection"
	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

// These are genuine Route/Stock exchanges, not qualified Service operations.
// Signed permission and receiving owners are fixtures; no worker/Startup is
// supplied by the cleanup observer.
func TestRegistrationStockRetainsIssuerControlForNextForwardTCP(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			x := newRouteIssuerFixture(t, carrier, nil, nil, nil, nil, true)
			f := x.network
			holder := routePermissionStock(t, f, admission.AllocationPublisher, [3]uint32{12, 12, 2})
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			root := t.TempDir()
			plan := funding.Declaration{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"),
				HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second),
				Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: admission.ControlClass.ByteLimit(), Rx: admission.ControlClass.ByteLimit()}}
			selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: 1, Current: f.current})
			if err != nil {
				t.Fatal(err)
			}
			defer selected.Close()
			budget, err := hosting.Open(plan.HostingRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer budget.Close()
			source, leg, flow, err := funding.OpenSource(ctx, plan.Plan(), selected, budget, fundingAuthority(f.authority), holder, 3, true, func(err error) {
				if err != nil {
					t.Error(err)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := source.Close(); err != nil {
					t.Error("original Source join", err)
				}
				if cleanup, joined := source.CleanupResult(); !joined || cleanup != nil {
					t.Error("original Source lacks joined cleanup", joined, cleanup)
				}
			}()
			// Two actual admitted exchanges leave one original issuer Control,
			// matching the stock entering a later Publication refresh.
			for range 2 {
				if err := source.Issue(ctx, plan.Deadline, funding.IssuerPreparation(ctx, fundingAuthority(f.authority), holder, leg, uint8(admission.ForwardClass), nil)); err != nil {
					t.Fatal(err)
				}
			}
			preparation, err := holder.Prepare(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer preparation.Close()
			controls, err := fundingChallenges(t, f, uint8(admission.ControlClass), [][32]byte{f.profile.IssuerNodeID, f.profile.IssuerNodeID})
			if err != nil {
				t.Fatal(err)
			}
			if missing, err := preparation.Missing(controls); err != nil || len(missing) != 1 {
				t.Fatal("control did not establish one retained issuer token", len(missing), err)
			}
			before := x.ordinaryIssues.Load()
			beforeRights := holder.Status().Remaining
			// Signed recipient stock isolates issuance, not an accepted physical
			// Registration or JOIN. Those remain genuine Service scenarios.
			if err := preparation.Close(); err != nil {
				t.Fatal(err)
			}
			present := func(id [32]byte, purpose ardp.Purpose) {
				view, err := f.current()
				if err != nil {
					t.Fatal(err)
				}
				member, err := view.Member(id, view.ObservedAt())
				if err != nil {
					t.Fatal(err)
				}
				hello := ardp.Hello{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration,
					StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, RecipientNodeID: id,
					RecipientDutyGeneration: member.DutyGeneration, ChannelNonce: [32]byte{1},
					Deadline: plan.Deadline, Purpose: purpose}
				if raw, err := flow.Present(ctx, hello); err != nil || len(raw) == 0 {
					t.Fatal("genuine funded presentation", err)
				}
			}
			present(leg.Entry.NodeID, ardp.PurposeIntroduction)
			preparation, err = holder.Prepare(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if missing, err := preparation.Missing(controls); err != nil || len(missing) != 0 {
				t.Fatal("Registration failed to retain issuer Control for next Forward", len(missing), err)
			}
			if err := preparation.Close(); err != nil {
				t.Fatal(err)
			}
			present([32]byte{77}, ardp.PurposeDataJoin)
			if x.ordinaryIssues.Load() != before+3 || x.bootstrapIssues.Load() != 2 || holder.Status().BootstrapRemaining != 0 {
				t.Fatal("Registration and next Forward required another Control refill or reset bootstrap", x.ordinaryIssues.Load(), before, holder.Status())
			}
			if holder.Status().Remaining != [3]uint32{beforeRights[0] - 3, beforeRights[1] - 1, beforeRights[2] - 1} {
				t.Fatal("combined preparation lost its exact irreversible permission debit", beforeRights, holder.Status())
			}
		})
	}
}

func TestReaderRoutePreparationFundsNextSourceWithoutBootstrapReset(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			x := newRouteIssuerFixture(t, carrier, nil, nil, nil, nil, true)
			f := x.network
			holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{12, 12, 0})
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			root := t.TempDir()
			plan := funding.Declaration{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"),
				HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second),
				Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: admission.ControlClass.ByteLimit(), Rx: admission.ControlClass.ByteLimit()}}
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
			var physicalFailure error
			retain := func(err error) { physicalFailure = errors.Join(physicalFailure, err) }
			source, leg, flow, err := funding.OpenSource(ctx, plan.Plan(), selected, budget, fundingAuthority(f.authority), holder, 3, true, retain)
			if err != nil {
				t.Fatal(err)
			}
			scope, err := prefix.NewSourceScope(ctx, source, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer scope.Close()
			handle, err := scope.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := flow.PrepareRoute(ctx, handle, leg.Interior, leg.Entry); err != nil {
				t.Fatal(err)
			}
			handle.Close()
			if err := scope.Close(); err != nil {
				t.Fatal(err)
			}
			before := holder.Status()
			if before.BootstrapRemaining != 0 {
				t.Fatal("original bootstrap was not consumed")
			}
			// A distinct caller opens from retained genuine unpresented Stock.
			nextCaller, stop := context.WithCancel(ctx)
			defer stop()
			next, _, _, err := funding.OpenSource(nextCaller, plan.Plan(), selected, budget, fundingAuthority(f.authority), holder, 1, true, retain)
			if err != nil {
				t.Fatal("next Source attempted exhausted cold bootstrap", err)
			}
			if err := next.Close(); err != nil {
				t.Fatal(err)
			}
			if physicalFailure != nil {
				t.Fatal("original physical cleanup failed", physicalFailure)
			}
			if x.bootstrapIssues.Load() != 2 || holder.Status().BootstrapRemaining != 0 || holder.Status().Remaining != before.Remaining {
				t.Fatal("next admitted Source reset bootstrap or allocated more quota", holder.Status())
			}
		})
	}
}

func TestFundedSourceBootstrapAndAdmittedIssuanceTCP(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			x := newRouteIssuerFixture(t, carrier, nil, nil, nil, nil, true)
			f := x.network
			holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{9, 10, 0})
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			root := t.TempDir()
			plan := funding.Declaration{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"),
				HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second),
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
			source, leg, flow, err := funding.OpenSource(ctx, plan.Plan(), selected, budget, fundingAuthority(f.authority), holder, 3, true,
				func(err error) { bootstrapCleanup = errors.Join(bootstrapCleanup, err) })
			if err != nil {
				t.Fatal("cold genuine Source", err)
			}
			defer source.Close()
			storeChallenge, err := fundingChallenges(t, f, uint8(admission.ControlClass), [][32]byte{{77}})
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
			config, err := funding.PreparePrefix(ctx, plan.Plan(), selected, budget, fundingAuthority(f.authority), holder)
			if err != nil {
				t.Fatal(err)
			}
			second, err := funding.PreparePrefix(ctx, plan.Plan(), selected, budget, fundingAuthority(f.authority), holder)
			if err != nil {
				t.Fatal(err)
			}
			second.Present = flow.OpeningPresentation(second.Leg)
			config.Present = flow.OpeningPresentation(config.Leg)
			if err := flow.PrepareInitialOpening(ctx, config.Leg, second.Leg); err != nil {
				t.Fatal("prepare Forward before physical opening", err)
			}
			if x.ordinaryIssues.Load() != 1 {
				t.Fatal("opening began without completed Forward issuance", x.ordinaryIssues.Load())
			}
			// A separate preparation may hold the issuance gate. Already funded
			// real openings must consume their own tokens without waiting for it.
			func() {
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
			if err := source.Issue(ctx, plan.Deadline, funding.IssuerPreparation(ctx, fundingAuthority(f.authority), holder, leg, uint8(admission.ForwardClass), nil)); err != nil {
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
			// Entry has one token, but it appears both as this fixture's requested
			// recipient and as the next Source recipient. Fund the genuine deficit
			// without presenting that next opening or renewing bootstrap.
			if err := flow.PrepareRoute(ctx, handle, leg.Interior, leg.Entry); err != nil {
				t.Fatal("prepare existing Forward", err)
			}
			if x.ordinaryIssues.Load() != 4 || holder.Status().Remaining != [3]uint32{2, 1, 0} {
				t.Fatal("prepared current recipient and next Source changed exact finite debit", x.ordinaryIssues.Load(), holder.Status())
			}
			before := holder.Status()
			if err := flow.PrepareRoute(ctx, handle, leg.Interior, leg.Entry); err != nil {
				t.Fatal("reuse unpresented prepared Stock", err)
			}
			if x.ordinaryIssues.Load() != 4 || holder.Status().Remaining != before.Remaining {
				t.Fatal("preparation spent recipient Stock or repeated issuance", holder.Status())
			}
			canceled, stop := context.WithCancel(ctx)
			stop()
			if err := flow.PrepareRoute(canceled, handle, leg.Interior, leg.Entry); !errors.Is(err, context.Canceled) {
				t.Fatal("canceled preparation accepted", err)
			}
			if x.ordinaryIssues.Load() != 4 || holder.Status().Remaining != before.Remaining || holder.Status().BootstrapRemaining != 0 {
				t.Fatal("canceled preparation changed finite allowance", holder.Status())
			}
			if err := flow.PrepareJoin(canceled, handle, leg.Entry); !errors.Is(err, context.Canceled) {
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
				if err := flow.PrepareJoin(ctx, handle, duty); err != nil {
					t.Fatal("prepare missing Forward", err)
				}
				if x.ordinaryIssues.Load() != 6 || holder.Status().Remaining != [3]uint32{0, 0, 0} || x.bootstrapIssues.Load() != 2 {
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

// The real Network loses clock authority after validating HELLO. Take must
// retain that failure, without falling through to the deliberately occupied
// preparation or trying to obtain replacement rights.
func TestFundedPresentationDoesNotPrepareAfterAuthorityFailure(t *testing.T) {
	x := newRouteIssuerFixture(t, transport.ClosedCarrierTCP, nil, nil, nil, nil, true)
	f := x.network
	holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{8, 8, 0})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	root := t.TempDir()
	plan := funding.Declaration{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"),
		HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second),
		Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: admission.ControlClass.ByteLimit(), Rx: admission.ControlClass.ByteLimit()}}
	selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: 1, Current: f.current})
	if err != nil {
		t.Fatal(err)
	}
	defer selected.Close()
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	authority := f.authority
	var loseClock atomic.Bool
	authority.Presentation = func(presentation stock.Presentation) error {
		if err := f.authority.Presentation(presentation); err != nil {
			return err
		}
		if loseClock.Load() {
			f.clockUnavailable.Store(true)
		}
		return nil
	}
	defer f.clockUnavailable.Store(false)
	source, _, flow, err := funding.OpenSource(ctx, plan.Plan(), selected, budget, fundingAuthority(authority), holder, 3, true, func(err error) {
		if err != nil {
			t.Error(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	preparation, err := holder.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer preparation.Close()
	before := holder.Status()
	loseClock.Store(true)
	hello := ardp.Hello{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration,
		StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, RecipientNodeID: f.receiver.NodeID,
		RecipientDutyGeneration: f.receiver.DutyGeneration, ChannelNonce: [32]byte{1},
		Deadline: time.Now().Add(5 * time.Second).UTC().Truncate(time.Second), Purpose: ardp.PurposeDataJoin}
	raw, err := flow.Present(ctx, hello)
	if !f.clockUnavailable.Load() || len(raw) != 0 || err == nil || err.Error() == "stock preparation unavailable" {
		t.Fatal("fresh Stock authority failure reached preparation or returned bytes", err)
	}
	after := holder.Status()
	if after.Pending || after.Remaining != before.Remaining || after.BootstrapRemaining != before.BootstrapRemaining {
		t.Fatal("authority failure changed the original finite allocation", before, after)
	}
}

// Inspect genuine holder inventory independently of funding's private challenge builder.
func fundingChallenges(t *testing.T, f *issuerReceivingNetwork, class uint8, receivers [][32]byte) ([]token.ClosedTokenContext, error) {
	t.Helper()
	view, err := f.current()
	if err != nil {
		return nil, err
	}
	var challenges []token.ClosedTokenContext
	for _, id := range receivers {
		member, err := view.Member(id, view.ObservedAt())
		if err != nil {
			return nil, err
		}
		challenges = append(challenges, token.ClosedTokenContext{NetworkID: f.profile.NetworkID,
			ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: id,
			ReceiverDutyGeneration: member.DutyGeneration, Class: class, WindowStart: view.ObservedAt().UTC().Truncate(time.Hour)})
	}
	return challenges, nil
}
