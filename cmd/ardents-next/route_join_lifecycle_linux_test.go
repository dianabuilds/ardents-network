//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	routejoin "github.com/dianabuilds/ardents-network/internal/successor/route/join"
	routeprefix "github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"

	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// One permission supplies two genuine tokens per receiver: a replacement Source
// must authenticate the installation's retained Entry again. Remaining quota is
// not stocked-token count; both authorized bootstrap batches are completed here.
func joinLifecycleStock(t *testing.T, f *networkAdmissionFixture) *stock.Owner {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	holder, err := stock.Open(root, admission.AllocationPublisher, f.authority.observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, digest, err := holder.Request([3]uint32{0, 20, 0})
	if err != nil {
		t.Fatal(err)
	}
	request, err := allocation.Prepare(raw, f.profile.NetworkID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := request.Decide(nil, f.profile.IssuanceAuthorityKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(t.TempDir(), "allocation")
	if err := os.WriteFile(journal, decision.Journal(), 0600); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(committed, decision.Journal()) {
		t.Fatal("lifecycle allocation durable readback", err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(f.spec.Authority, admission.PermissionTranscript(permission)))
	encoded, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Import(digest, encoded); err != nil {
		t.Fatal(err)
	}
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	var challenges []token.ClosedTokenContext
	for idByte := byte(12); idByte <= 21; idByte++ {
		id := [32]byte{idByte}
		member, err := view.Member(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		challenges = append(challenges, token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: id, ReceiverDutyGeneration: member.DutyGeneration, Class: 2, WindowStart: permission.NotBefore})
	}
	for index := byte(0); index < 2; index++ {
		attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{71, index}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: f.profile.NotAfter})
		if err != nil {
			t.Fatal(err)
		}
		batch, _, err := attempt.Request()
		if err != nil {
			t.Fatal(err)
		}
		issued := issuer.IssueCurrent(t.Context(), f.plan, batch, quota.Bootstrap, func() (admission.AuthorityFacts, time.Time, error) {
			return f.authority.issuer(f.plan.KeyBinding.Signer)
		})
		if issued.Outcome != "issued-offline" {
			t.Fatal(issued)
		}
		response, err := admission.DecodeClosedTokenBatchResult(issued.Response)
		if err != nil || response.Status != admission.ClosedTokenIssued || len(response.Signatures) != 10 {
			t.Fatal("lifecycle genuine ten-token batch", err)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal(err)
		}
	}
	return holder
}

// Test composition exposes original handles while retaining genuine signed
// Network, Stock, physical reservations and actual role admission at every hop.
func openJoinFixtureRole(t *testing.T, f *networkAdmissionFixture, installation *selection.Installation, holder *stock.Owner, budget *hosting.Budget, domain uint8, interior string, source *routeprefix.Prefix, afterTake func(context.Context, stock.Presentation)) (*routeprefix.Prefix, selection.Leg, error) {
	t.Helper()
	selected, err := installation.Borrow(selection.RoleConfig{InteriorRoot: interior, Domain: domain})
	if err != nil {
		return nil, selection.Leg{}, err
	}
	leg, err := selected.Select()
	if err != nil {
		return nil, leg, errors.Join(err, selected.Close())
	}
	end := time.Now().Add(45 * time.Second).UTC().Truncate(time.Second)
	reservation, err := budget.Reserve(t.Context(), hosting.ReservationRequest{Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: end, HoldUntil: end.Add(5 * time.Second)})
	if err != nil {
		return nil, leg, errors.Join(err, selected.Close())
	}
	var once sync.Once
	var terminal error
	release := func() error {
		once.Do(func() { terminal = errors.Join(releaseRouteReservation(reservation), selected.Close()) })
		return terminal
	}
	config := routeprefix.Config{Leg: leg, Current: f.current, Deadline: end, Release: release, Present: func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
		presentation := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
		if err := f.authority.presentation(presentation); err != nil {
			return nil, err
		}
		raw, err := holder.Take(ctx, presentation, 2)
		if err == nil && afterTake != nil {
			afterTake(ctx, presentation)
		}
		return raw, err
	}}
	var prefix *routeprefix.Prefix
	if domain == 3 {
		prefix, err = routeprefix.OpenResponder(t.Context(), source, config)
	} else {
		prefix, err = routeprefix.Open(t.Context(), config)
	}
	if err != nil {
		return nil, leg, errors.Join(err, release())
	}
	return prefix, leg, nil
}

func TestRouteStockedResponderCannotAdoptReplacementSource(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, sockets, certificates := newJoinRouteFixture(t, profile)
			holder := joinLifecycleStock(t, f)
			startJoinRouteReceivers(t, f, sockets, certificates)
			root := t.TempDir()
			installation, err := selection.OpenInstallation(selection.InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: f.current})
			if err != nil {
				t.Fatal(err)
			}
			defer installation.Close()
			budget := networkTestBudget(t)
			var presentations atomic.Int32
			afterTake := func(context.Context, stock.Presentation) { presentations.Add(1) }
			source, originalLeg, err := openJoinFixtureRole(t, f, installation, holder, budget, 1, filepath.Join(root, "source-original"), nil, afterTake)
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			responder, _, err := openJoinFixtureRole(t, f, installation, holder, budget, 3, filepath.Join(root, "responder"), source, afterTake)
			if err != nil {
				t.Fatal(err)
			}
			defer responder.Close()
			acquisition, err := routejoin.AcquireResponderJoin(t.Context(), responder)
			if err != nil {
				t.Fatal(err)
			}
			defer acquisition.Close()
			selection, err := selection.NewRendezvous(originalLeg, f.current, nil)
			if err != nil {
				t.Fatal(err)
			}
			duty, err := selection.Duty(0, nil)
			if err != nil {
				t.Fatal(err)
			}
			source.Seal()
			replacement, _, err := openJoinFixtureRole(t, f, installation, holder, budget, 1, filepath.Join(root, "source-replacement"), nil, afterTake)
			if err != nil {
				t.Fatal("real replacement Source", err)
			}
			defer replacement.Close()
			// Six actual presentations admitted the three prefixes. No Rendezvous
			// token has been presented; the two signed batches stocked both duties.
			before := presentations.Load()
			if before != 6 {
				t.Fatal("three genuine prefixes did not present exactly six tokens", before)
			}
			if next, err := routejoin.AcquireResponderJoin(t.Context(), responder); err == nil || next != nil {
				t.Fatal("Responder adopted replacement instead of sealed original", err)
			}
			intent := routejoin.JoinConfig{Duty: duty, Deadline: time.Now().Add(10 * time.Second).UTC().Truncate(time.Second), SetupDeadline: time.Now().Add(5 * time.Second).UTC().Truncate(time.Second)}
			if _, err := rand.Read(intent.Secret[:]); err != nil {
				t.Fatal(err)
			}
			if _, err := rand.Read(intent.Context[:]); err != nil {
				t.Fatal(err)
			}
			if stream, err := acquisition.Join(t.Context(), intent); err == nil || stream != nil {
				t.Fatal("obsolete original acquisition handed off JOIN", err)
			}
			if presentations.Load() != before {
				t.Fatal("sealed original consumed stocked token")
			}
			if err := errors.Join(acquisition.Close(), responder.Close(), source.Close()); err != nil {
				t.Fatal("original generation failed joined cleanup", err)
			}
			select {
			case <-replacement.Done():
				t.Fatal("original close retired replacement Source")
			default:
			}
			live, err := routejoin.AcquireSourceJoin(t.Context(), replacement)
			if err != nil {
				t.Fatal("replacement affected by original cleanup", err)
			}
			if err := errors.Join(live.Close(), replacement.Close(), installation.Close()); err != nil {
				t.Fatal(err)
			}
			observation, err := budget.Observe(t.Context())
			if err != nil || observation.ReservedBytes != 0 {
				t.Fatal("original and replacement reservations not joined", observation.ReservedBytes, err)
			}
		})
	}
}

func TestRouteResponderOpeningLosesOriginalSourceDuringPresentation(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		for _, checkpoint := range []int32{1, 2} {
			t.Run(fmt.Sprintf("%s/presentation-%d", profile, checkpoint), func(t *testing.T) {
				f, sockets, certificates := newJoinRouteFixture(t, profile)
				holder := joinRouteStock(t, f, admission.AllocationPublisher)
				receivers, receivingBudgets := startJoinRouteReceivers(t, f, sockets, certificates)
				root := t.TempDir()
				installation, err := selection.OpenInstallation(selection.InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: f.current})
				if err != nil {
					t.Fatal(err)
				}
				defer installation.Close()
				budget := networkTestBudget(t)
				source, _, err := openJoinFixtureRole(t, f, installation, holder, budget, 1, filepath.Join(root, "source"), nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer source.Close()
				before, err := budget.Observe(t.Context())
				if err != nil || before.ReservedBytes == 0 {
					t.Fatal("original Source lacks real reservation", before.ReservedBytes, err)
				}
				type openingResult struct {
					prefix *routeprefix.Prefix
					err    error
				}
				entered := make(chan context.Context, 1)
				release := make(chan struct{})
				var releaseOnce sync.Once
				unblock := func() { releaseOnce.Do(func() { close(release) }) }
				opened := make(chan openingResult, 1)
				var workers sync.WaitGroup
				defer func() { unblock(); workers.Wait() }()
				var presentations atomic.Int32
				var burnt stock.Presentation
				workers.Add(1)
				go func() {
					defer workers.Done()
					prefix, _, err := openJoinFixtureRole(t, f, installation, holder, budget, 3, filepath.Join(root, "responder"), source, func(ctx context.Context, presentation stock.Presentation) {
						if presentations.Add(1) == checkpoint {
							// Actual Stock has already journalled this authenticated
							// HELLO presentation; only callback completion is delayed.
							burnt = presentation
							entered <- ctx
							<-release
						}
					})
					opened <- openingResult{prefix, err}
				}()
				var openingContext context.Context
				select {
				case openingContext = <-entered:
				case result := <-opened:
					if result.prefix != nil {
						_ = result.prefix.Close()
					}
					t.Fatal("opening failed before real presentation gate", result.err)
				case <-time.After(10 * time.Second):
					t.Fatal("real Responder presentation did not reach gate")
				}
				during, err := budget.Observe(t.Context())
				if err != nil || during.ReservedBytes != 2*before.ReservedBytes {
					t.Fatal("original and opening must retain separate reservations", during.ReservedBytes, before.ReservedBytes, err)
				}
				source.Seal()
				select {
				case <-openingContext.Done():
				case <-time.After(5 * time.Second):
					t.Fatal("original Source did not revoke in-flight Responder")
				}
				closed := make(chan error, 1)
				workers.Add(1)
				go func() { defer workers.Done(); closed <- source.Close() }()
				unblock()
				result := <-opened
				if result.prefix != nil {
					_ = result.prefix.Close()
				}
				if result.prefix != nil || !errors.Is(result.err, context.Canceled) {
					t.Error("late Responder opening escaped retired original Source", result.err)
				}
				if presentations.Load() != checkpoint {
					t.Error("retirement allowed another Stock presentation", presentations.Load())
				}
				if err := <-closed; err != nil {
					t.Error("original Source joined cleanup", err)
				}
				// No retry or fake refund: the real single token for this receiver
				// was consumed before the interrupted callback returned.
				raw, err := holder.Take(t.Context(), burnt, 2)
				clear(raw)
				if err == nil {
					t.Error("cancelled Responder presentation was refunded")
				}
				if err := installation.Close(); err != nil {
					t.Error("opening retained selection borrow", err)
				}
				for _, receiver := range receivers {
					if err := receiver.Close(); err != nil {
						t.Error("receiver joined cleanup", err)
					}
				}
				for _, physicalBudget := range append(receivingBudgets, budget) {
					observation, err := physicalBudget.Observe(t.Context())
					if err != nil || observation.ReservedBytes != 0 {
						t.Error("late opening retained physical reservation", observation.ReservedBytes, err)
					}
				}
			})
		}
	}
}

func TestRouteContextReopenRetainsChoiceAndOriginalHandles(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, sockets, certificates := newJoinRouteFixture(t, profile)
			holder := joinLifecycleStock(t, f)
			startJoinRouteReceivers(t, f, sockets, certificates)
			root := t.TempDir()
			plan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(60 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			owner, err := newRouteJoinContext(t.Context(), plan, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.close()
			first, err := owner.open(t.Context())
			if err != nil {
				t.Fatal("first genuine opening", err)
			}
			defer first.close()
			var choices [2]routeRecipient
			for slot := range choices {
				choices[slot], err = first.recipient(uint8(slot))
				if err != nil {
					t.Fatal("initial retained choices", err)
				}
			}
			if err := first.close(); err != nil {
				t.Fatal("first joined retirement", err)
			}
			if borrowed, err := selection.OpenInstallation(selection.InstallationConfig{EntryRoot: plan.EntryRoot, Current: f.current}); err == nil {
				_ = borrowed.Close()
				t.Fatal("physical retirement released installation Entry selection")
			}
			second, err := owner.open(t.Context())
			if err != nil {
				t.Fatal("reopen with genuine remaining stock", err)
			}
			defer second.close()
			for slot, original := range choices {
				again, err := second.recipient(uint8(slot))
				if err != nil || again != original {
					t.Fatal("physical reopen changed retained choice or bound", slot, original, again, err)
				}
			}
			if _, err := first.recipient(0); err == nil {
				t.Fatal("original handle acquired replacement selection")
			}
			intent := routeJoinIntent{Node: choices[0].Node, Generation: choices[0].Generation, Deadline: choices[0].NotAfter, SetupDeadline: time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)}
			if _, err := rand.Read(intent.Secret[:]); err != nil {
				t.Fatal(err)
			}
			if _, err := rand.Read(intent.Context[:]); err != nil {
				t.Fatal(err)
			}
			if stream, err := first.join(t.Context(), intent); err == nil || stream != nil {
				t.Fatal("original handle acquired replacement JOIN", err)
			}
			if err := first.close(); err != nil {
				t.Fatal("original close result changed", err)
			}
			select {
			case <-second.done:
				t.Fatal("repeated original Close retired replacement")
			default:
			}
			if again, err := second.recipient(0); err != nil || again != choices[0] {
				t.Fatal("replacement affected by original cleanup", err)
			}
			if err := errors.Join(second.close(), owner.close()); err != nil {
				t.Fatal("context joined retirement", err)
			}
			budget, err := hosting.Open(plan.HostingRoot)
			if err != nil {
				t.Fatal("context retained Hosting root", err)
			}
			observed, err := budget.Observe(t.Context())
			if err := errors.Join(err, budget.Close()); err != nil || observed.ReservedBytes != 0 {
				t.Fatal("context retained physical reservation", observed.ReservedBytes, err)
			}
		})
	}
}
