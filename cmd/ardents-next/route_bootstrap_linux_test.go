//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/route/channel"
	routereceiver "github.com/dianabuilds/ardents-network/internal/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

// One genuine receiving installation: separate signed authority, spend roots
// and Hosting budgets. Gates observe actual issuance/release; none mint a Grant
// or replace IssueCurrent. Used by retirement and competing-holder scenarios.
type routeIssuerFixture struct {
	network                                                                       *networkAdmissionFixture
	servers                                                                       []*routereceiver.Receiver
	budgets                                                                       []*hosting.Budget
	issuerBudget                                                                  *hosting.Budget
	stopIssuer                                                                    context.CancelFunc
	bootstrapHolds, bootstrapReturns, admissions, ordinaryIssues, bootstrapIssues atomic.Int32
	issuerAdmissionCalls, issuerAdmissions, issuerReturns                         atomic.Int32
	spends                                                                        []*routeFixtureSpends
}

// Retained by the test observer, separately from the production receiver. Only
// successfully admitted canonical tokens are copied; these facts mint no rights.
type routeFixtureSpends struct {
	owner   *receiving.Owner
	root    string
	binding receiving.Receiver
	observe func() (receiving.Observation, error)
	mu      sync.Mutex
	tokens  []struct {
		class admission.Class
		raw   []byte
	}
}

func newRouteIssuerFixture(t *testing.T, profile transport.CarrierProfile, afterIssue func(context.Context, bool, []byte, []byte) error, afterIssuerReturn func() error, admissionPoint func(string), issuerObservation func(int), resolution ...bool) *routeIssuerFixture {
	t.Helper()
	f, sockets, certificates := newRoleRouteFixtureConfigured(t, profile, 1, false, true,
		func(f *networkAdmissionFixture, _ map[[32]byte]func(), _ map[[32]byte]tls.Certificate) {
			if len(resolution) == 0 || !resolution[0] {
				return
			}
			// Signed recipient for a Stock-only oracle; no live Store is claimed.
			node := f.spec.Nodes[0]
			node.NodeID = [32]byte{77}
			node.PrivateKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{77}, ed25519.SeedSize))
			t.Cleanup(func() { clear(node.PrivateKey) })
			node.Endpoint, node.Carrier = "127.0.0.1:65000", string(profile)
			node.RoleDomain, node.Subrole = 2, 5
			f.spec.Nodes = append(f.spec.Nodes, node)
		})
	x := &routeIssuerFixture{network: f}
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}, f.profile.IssuerNodeID} {
		member, err := view.Member(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		duty, err := view.RetainDuty(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: id, DutyGeneration: member.DutyGeneration}
		root := t.TempDir()
		if err := os.Chmod(root, 0700); err != nil {
			t.Fatal(err)
		}
		owner, err := receiving.Open(root, binding, func() (receiving.Observation, error) { return f.authority.Receiver(binding, member.NotAfter()) })
		if err != nil {
			t.Fatal(err)
		}
		spent := &routeFixtureSpends{owner: owner, root: root, binding: binding,
			observe: func() (receiving.Observation, error) { return f.authority.Receiver(binding, member.NotAfter()) }}
		x.spends = append(x.spends, spent)
		t.Cleanup(func() {
			spent.mu.Lock()
			defer spent.mu.Unlock()
			for _, token := range spent.tokens {
				clear(token.raw)
			}
		})
		t.Cleanup(func() {
			if err := owner.Close(); err != nil {
				t.Error(err)
			}
		})
		budget := networkTestBudget(t)
		x.budgets = append(x.budgets, budget)
		isIssuer := id == f.profile.IssuerNodeID
		if isIssuer {
			x.issuerBudget = budget
		}
		config := routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id],
			ReserveBootstrap: func(ctx context.Context, end time.Time) (func() error, error) {
				release, err := networkTestReservation(t, budget, end)
				if err != nil {
					return nil, err
				}
				x.bootstrapHolds.Add(1)
				return func() error { x.bootstrapReturns.Add(1); return release() }, nil
			},
			Admit: func(ctx context.Context, c routereceiver.Channel, raw []byte) (receiving.Grant, error) {
				if isIssuer {
					x.issuerAdmissionCalls.Add(1)
				}
				class, err := role.AdmissionClass(c.Hello.Purpose)
				if err != nil {
					return receiving.Grant{}, err
				}
				if isIssuer && admissionPoint != nil {
					admissionPoint("before-accept")
				}
				grant, err := owner.Accept(ctx, class, raw, c.Hello.Deadline, func() (func() error, error) {
					held, err := networkTestHold(t, budget, c.Hello.Deadline)
					if err != nil {
						return nil, err
					}
					if isIssuer && admissionPoint != nil {
						admissionPoint("after-reservation")
					}
					if !isIssuer {
						return c.HoldReservation(held.ReleaseJoined)
					}
					return holdRouteIssuerReservation(c, held, func() error {
						if admissionPoint != nil {
							admissionPoint("before-return")
						}
						err := held.ReleaseJoined()
						x.issuerReturns.Add(1)
						return err
					}, afterIssuerReturn)
				})
				if err == nil {
					spent.mu.Lock()
					spent.tokens = append(spent.tokens, struct {
						class admission.Class
						raw   []byte
					}{class: class, raw: bytes.Clone(raw)})
					spent.mu.Unlock()
					x.admissions.Add(1)
					if isIssuer {
						x.issuerAdmissions.Add(1)
						if admissionPoint != nil {
							admissionPoint("after-accept")
						}
					}
				}
				return grant, err
			},
		}
		if isIssuer {
			config.Issue = func(ctx context.Context, batch []byte, bootstrap bool) ([]byte, error) {
				if bootstrap {
					x.bootstrapIssues.Add(1)
				} else {
					x.ordinaryIssues.Add(1)
				}
				authority := f.authority
				if !bootstrap && issuerObservation != nil {
					var observation int
					journal := filepath.Join(f.plan.AdmissionRoot, "admission.journal")
					before, err := os.Stat(journal)
					if err != nil {
						return nil, err
					}
					var beforeDebit, afterDebit bool
					original := authority.Issuer
					authority.Issuer = func(signer [32]byte) (admission.AuthorityFacts, time.Time, error) {
						observation++
						// Additional retained startup/custody checks are not debit
						// effects. Keep the existing boundary controls tied to the
						// real journal transition, rather than a raw call ordinal.
						if observation == 1 {
							issuerObservation(1)
						}
						current, err := os.Stat(journal)
						if err != nil {
							return admission.AuthorityFacts{}, time.Time{}, err
						}
						if observation > 1 && current.Size() == before.Size() && !beforeDebit {
							beforeDebit = true
							issuerObservation(2)
						} else if current.Size() > before.Size() && !afterDebit {
							afterDebit = true
							issuerObservation(3)
						}
						return original(signer)
					}
				}
				payload, err := issueRouteBatch(ctx, authority, f.plan, batch, bootstrap)
				if err == nil && afterIssue != nil {
					// A post-signing gate must observe actual successful signing,
					// not a same-sized unavailable/exhausted response.
					if !bootstrap {
						outcome, decodeErr := admission.DecodeClosedTokenBatchResult(payload)
						if decodeErr != nil || outcome.Status != admission.ClosedTokenIssued || len(outcome.Signatures) == 0 {
							clear(payload)
							return nil, errors.Join(errors.New("actual issuer did not sign the gated result"), decodeErr)
						}
						for _, signature := range outcome.Signatures {
							clear(signature)
						}
					}
					err = afterIssue(ctx, bootstrap, batch, payload)
				}
				if err != nil {
					clear(payload)
					return nil, err
				}
				return payload, nil
			}
		}
		sockets[id]()
		listenContext, stop := context.WithCancel(t.Context())
		t.Cleanup(stop)
		if isIssuer {
			x.stopIssuer = stop
		}
		server, err := routereceiver.Listen(listenContext, config)
		if err != nil {
			t.Fatal(err)
		}
		x.servers = append(x.servers, server)
		// Scenarios explicitly inspect the retained Close result. Always join
		// again here on an early assertion failure before storage cleanup runs.
		t.Cleanup(func() { _ = server.Close() })
	}
	return x
}

// Two holders reach the same genuine issuer through independently admitted
// prefixes. The gate delays delivery after real durable debit/signing; it is
// neither a simulated signing success nor an artificial receiving Grant.
func TestRouteIssuerExclusiveWorkJoinsBeforeReturnTCP(t *testing.T) {
	t.Parallel()
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		for _, mode := range []string{"joined-result", "original-cancellation", "late-release-failure"} {
			t.Run(string(profile)+"/"+mode, func(t *testing.T) {
				started, resume := make(chan struct{}), make(chan struct{})
				var releaseOnce sync.Once
				releaseGate := func() { releaseOnce.Do(func() { close(resume) }) }
				defer releaseGate()
				var gated atomic.Bool
				var deliveries, physicalReturns atomic.Int32
				late := errors.New("late issuer Hosting return failure")
				x := newRouteIssuerFixture(t, profile, func(_ context.Context, bootstrap bool, _, _ []byte) error {
					if !bootstrap && gated.Load() && deliveries.Add(1) == 1 {
						close(started)
						<-resume // original context remains checked by serveIssuer afterward
					}
					return nil
				}, func() error {
					if physicalReturns.Add(1) == 1 && mode == "late-release-failure" {
						return late
					}
					return nil
				}, nil, nil)
				f := x.network
				var holders [2]*stock.Owner
				var prefixes [2]routeHandle
				for i := range holders {
					holders[i] = routePermissionStock(t, f, admission.AllocationUser, [3]uint32{4, 8, 0})
					local := t.TempDir()
					plan := routePrefixPlan{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
					boot, err := startRouteBootstrap(t.Context(), plan, f.authority, holders[i])
					if err != nil {
						t.Fatal("genuine bootstrap", err)
					}
					defer boot.close()
					if err := boot.issue(t.Context(), 2, nil); err != nil {
						t.Fatal("forwarding stock", err)
					}
					issuer := f.profile.IssuerNodeID
					if err := boot.issue(t.Context(), 1, [][32]byte{issuer, issuer, issuer}); err != nil {
						t.Fatal("Control stock", err)
					}
					if err := boot.close(); err != nil {
						t.Fatal("bootstrap join", err)
					}
					prefixes[i], err = startRoutePrefix(t.Context(), plan, f.authority, holders[i])
					if err != nil {
						t.Fatal("fresh admitted prefix", err)
					}
					defer prefixes[i].close()
				}
				gated.Store(true)
				caller, cancel := context.WithCancel(t.Context())
				defer cancel()
				completed := make(chan error, 1)
				go func() { completed <- prefixes[0].issue(caller, 2, nil) }()
				select {
				case <-started:
				case err := <-completed:
					t.Fatal("issuer did not reach actual signed result", err)
				}
				if err := prefixes[1].issue(t.Context(), 2, nil); err == nil {
					t.Fatal("concurrent issuer operation accepted")
				}
				if x.issuerAdmissionCalls.Load() != 1 || x.issuerAdmissions.Load() != 1 || x.ordinaryIssues.Load() != 1 || x.issuerReturns.Load() != 0 {
					t.Fatal("busy operation spent or released held issuer work", x.issuerAdmissionCalls.Load(), x.issuerAdmissions.Load(), x.ordinaryIssues.Load(), x.issuerReturns.Load())
				}
				if s := holders[1].Status(); !s.Pending || s.Busy || s.Remaining[1] != 4 {
					t.Fatal("busy exchange lost pending batch or refunded quota", s)
				}
				var firstErr error
				cancelled := mode == "original-cancellation"
				if cancelled {
					cancel()
					firstErr = <-completed
					if !errors.Is(firstErr, context.Canceled) {
						t.Error("original cancellation absent from completion", firstErr)
					}
					if s := holders[0].Status(); !s.Pending || s.Busy || s.Remaining[1] != 4 {
						t.Error("cancelled result deposited stock or refunded quota", s)
					}
				}
				observed, err := x.issuerBudget.Observe(t.Context())
				if err != nil || observed.ReservedBytes == 0 || x.issuerReturns.Load() != 0 {
					t.Fatal("actual work returned before delayed owner joined", observed.ReservedBytes, err)
				}
				releaseGate()
				if !cancelled {
					firstErr = <-completed
				}
				if mode == "joined-result" {
					if firstErr != nil {
						t.Fatal("first issuer completion", firstErr)
					}
					if err := prefixes[1].issue(t.Context(), 2, nil); err != nil {
						t.Fatal("explicit same-prefix retry after owner join", err)
					}
					if x.issuerAdmissions.Load() != 2 || x.ordinaryIssues.Load() != 2 || holders[1].Status().Pending || holders[1].Status().Remaining[1] != 4 {
						t.Fatal("healthy sibling did not complete genuine retry")
					}
				} else if cancelled && firstErr == nil {
					t.Error("failed original became successful completion")
				}
				var retired error
				for _, p := range prefixes {
					retired = errors.Join(retired, p.close())
				}
				var receiverErr error
				for _, server := range x.servers {
					receiverErr = errors.Join(receiverErr, server.Close())
				}
				if mode == "joined-result" && (retired != nil || receiverErr != nil) {
					t.Error("healthy sibling retirement", retired, receiverErr)
				}
				if mode == "late-release-failure" && !errors.Is(receiverErr, late) {
					// Hosting return follows physical joining. It is retained by
					// this receiving owner; no closed peer channel can convey a
					// later local callback failure or revoke a verified signature.
					t.Error("late genuine return failure erased", receiverErr)
				}
				for _, budget := range x.budgets {
					observed, err := budget.Observe(t.Context())
					if err != nil || observed.ReservedBytes != 0 {
						t.Error("joined receiving owner retained Hosting", observed.ReservedBytes, err)
					}
				}
				wantReturns := int32(1)
				if mode == "joined-result" {
					wantReturns = 2
				}
				if x.issuerReturns.Load() != wantReturns {
					t.Error("issuer reservation return was not exactly once", x.issuerReturns.Load(), wantReturns)
				}
			})
		}
	}
}

// The successor is signed and accepted by the actual Network owner. The gates
// surround actual receiving Accept or actual committed issuer output, not a
// supplied authority boolean. Every original binding remains unchanged.
func TestRouteIssuerStateLossAtRealBoundariesTCP(t *testing.T) {
	t.Parallel()
	routeIssuerRetirementCases(t, false)
}

func TestRouteIssuerReceiverCancellationAtRealBoundariesTCP(t *testing.T) {
	t.Parallel()
	routeIssuerRetirementCases(t, true)
}

func routeIssuerRetirementCases(t *testing.T, cancelReceiver bool) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		phases := []string{"before-accept", "after-reservation", "after-accept", "before-debit", "after-debit", "after-signing"}
		if cancelReceiver {
			phases = phases[:len(phases)-1] // The real-process after-signing case is covered by E2E.
		}
		for _, phase := range phases {
			t.Run(string(profile)+"/"+phase, func(t *testing.T) {
				entered, resume := make(chan struct{}), make(chan struct{})
				var releaseOnce sync.Once
				release := func() { releaseOnce.Do(func() { close(resume) }) }
				defer release()
				pause := func(point string) {
					if point == phase {
						close(entered)
						<-resume
					}
				}
				x := newRouteIssuerFixture(t, profile, func(ctx context.Context, bootstrap bool, request, result []byte) error {
					if !bootstrap {
						pause("after-signing")
					}
					return nil
				}, nil, pause, func(observation int) {
					// The fixture maps these phase controls to the actual quota
					// journal transition. The assertions below independently verify
					// the retained debit and result bytes before authority changes.
					switch observation {
					case 2:
						pause("before-debit")
					case 3:
						pause("after-debit")
					}
				})
				f := x.network
				holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{2, 8, 0})
				local := t.TempDir()
				plan := routePrefixPlan{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				boot, err := startRouteBootstrap(t.Context(), plan, f.authority, holder)
				if err != nil {
					t.Fatal("bootstrap", err)
				}
				defer boot.close()
				for _, class := range []uint8{2, 1} {
					if err := boot.issue(t.Context(), class, nil); err != nil {
						t.Fatal("genuine bootstrap stock", err)
					}
				}
				if err := boot.close(); err != nil {
					t.Fatal("bootstrap join", err)
				}
				journalSize := func(root, name string) int64 {
					info, err := os.Stat(filepath.Join(root, name))
					if err != nil {
						t.Fatal("actual issuer journal unavailable", err)
					}
					return info.Size()
				}
				debitSize := journalSize(f.plan.AdmissionRoot, "admission.journal")
				resultSize := journalSize(f.plan.ResultRoot, "results.journal")
				original, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
				if err != nil {
					t.Fatal("actual admitted original", err)
				}
				defer original.close()
				completed := make(chan error, 1)
				go func() { completed <- original.issue(t.Context(), 2, nil) }()
				select {
				case <-entered:
				case err := <-completed:
					t.Fatal("actual issuer boundary not reached", phase, err)
				}
				wantAdmitted, wantIssued := int32(0), int32(0)
				if phase != "before-accept" && phase != "after-reservation" {
					wantAdmitted = 1
				}
				wantReserved := int32(0)
				if phase != "before-accept" {
					wantReserved = 1
				}
				if phase == "before-debit" || phase == "after-debit" || phase == "after-signing" {
					wantIssued = 1
				}
				if x.issuerAdmissions.Load() != wantAdmitted || x.ordinaryIssues.Load() != wantIssued || x.issuerReturns.Load() != 0 {
					t.Fatal("boundary did not retain actual effects", phase, x.issuerAdmissions.Load(), x.ordinaryIssues.Load(), x.issuerReturns.Load())
				}
				before, err := x.issuerBudget.Observe(t.Context())
				// These are the explicit test operator inputs to real Hosting,
				// not a multiplier inferred from token bytes or TLS overhead.
				wantReservedBytes := uint64(wantReserved) * 2 * ((2 << 20) + (64 << 10))
				if err != nil || before.ReservedBytes != wantReservedBytes {
					t.Fatal("real Hosting boundary differs", before.ReservedBytes, err)
				}
				wantDebit := phase == "after-debit" || phase == "after-signing"
				committedDebit := journalSize(f.plan.AdmissionRoot, "admission.journal")
				committedResult := journalSize(f.plan.ResultRoot, "results.journal")
				if (committedDebit > debitSize) != wantDebit {
					t.Fatal("durable quota effect differs at actual boundary", phase)
				}
				if (committedResult > resultSize) != (phase == "after-signing") {
					t.Fatal("committed signing effect differs at actual boundary", phase)
				}
				if cancelReceiver {
					// Cancel the real listener generation, not the holder caller.
					// Its borrower is gated, so cleanup cannot already have joined.
					x.stopIssuer()
					retiring, err := x.issuerBudget.Observe(t.Context())
					if err != nil || retiring.ReservedBytes != wantReservedBytes {
						t.Fatal("receiver cancellation returned capacity before borrower join", retiring.ReservedBytes, err)
					}
				} else {
					// Durable successor intake, never a fabricated RuntimeView.
					f.successor(t)
				}
				if t.Context().Err() != nil {
					t.Fatal("test caller ended instead of State loss")
				}
				release()
				if err := <-completed; err == nil {
					t.Error("obsolete State produced successful issuer completion", phase)
				}
				if x.issuerAdmissions.Load() != wantAdmitted || x.ordinaryIssues.Load() != wantIssued {
					t.Error("old request acquired a successor effect", phase)
				}
				if s := holder.Status(); s.Busy || s.Pending != cancelReceiver || s.Remaining[1] != 4 {
					t.Error("State loss deposited/rebound or refunded original batch", s)
				}
				terminal := original.close() // retain genuine authority retirement, never renew it
				if repeated := original.close(); terminal == nil && repeated != nil || terminal != nil && !errors.Is(repeated, terminal) {
					t.Error("original retirement result changed", terminal, repeated)
				}
				for _, server := range x.servers {
					terminal := server.Close()
					if repeated := server.Close(); terminal == nil && repeated != nil || terminal != nil && !errors.Is(repeated, terminal) {
						t.Error("receiving retirement result changed", terminal, repeated)
					}
				}
				if journalSize(f.plan.AdmissionRoot, "admission.journal") != committedDebit || journalSize(f.plan.ResultRoot, "results.journal") != committedResult {
					t.Error("obsolete issuance changed durable debit/result history", phase)
				}
				if x.issuerReturns.Load() != wantReserved {
					t.Error("original reservation return differed", x.issuerReturns.Load(), wantReserved)
				}
				for _, budget := range x.budgets {
					observed, err := budget.Observe(t.Context())
					if err != nil || observed.ReservedBytes != 0 {
						t.Error("obsolete physical work not joined", observed.ReservedBytes, err)
					}
				}
			})
		}
	}
}

// Genuine signed Network and ordinary Stock/spend/Hosting exercise the actual
// receiver. Pending bootstrap allocation is not successful issuer exchange.
// Linux is required by the real Network, spend and Hosting roots in this test;
// portable queue, framing and capacity controls stay with their rule owners.
func TestRoutePendingBootstrapLimitLeavesOrdinaryChildUsableOnSameCarrier(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(profile), func(t *testing.T) {
			f, sockets, certificates := newRoleRouteFixture(t, profile, 1, false)
			// Two distinct genuine tokens: refusal must not refund/reuse the
			// durably presented restricted token to make the sibling succeed.
			holder := routeRoleStock(t, f, false, true)
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			member, err := view.Member([32]byte{14}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			duty, err := view.RetainDuty(member.NodeID, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			a := role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}
			binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: member.NodeID, DutyGeneration: member.DutyGeneration}
			owner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return f.authority.Receiver(binding, member.NotAfter()) })
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			budget := networkTestBudget(t)
			var admits, releases, bootstrapHolds, bootstrapReturns atomic.Int32
			sockets[member.NodeID]()
			server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: a, Certificate: certificates[member.NodeID], ReserveBootstrap: func(ctx context.Context, end time.Time) (func() error, error) {
				held, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: end, HoldUntil: end.Add(time.Second)})
				if err != nil {
					return nil, err
				}
				bootstrapHolds.Add(1)
				return func() error { bootstrapReturns.Add(1); return held.ReleaseJoined() }, nil
			}, Admit: func(ctx context.Context, c routereceiver.Channel, raw []byte) (receiving.Grant, error) {
				admits.Add(1)
				return owner.Accept(ctx, admission.ForwardClass, raw, c.Hello.Deadline, func() (func() error, error) {
					release, err := networkTestReservation(t, budget, c.Hello.Deadline)
					if err != nil {
						return nil, err
					}
					return c.HoldReservation(func() error { releases.Add(1); return release() })
				})
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			end := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
			request := transport.ClosedNodeCarrierRequest{CarrierProfile: profile, Endpoint: member.Endpoint, Certificate: certificates[[32]byte{12}], ExpectedPeerKey: member.PublicKey, Deadline: end}
			carrier := routeFixtureNodeCarrier(t, request)
			defer carrier.Close()
			conn, ok := carrier.(net.Conn)
			if !ok {
				t.Fatal("actual Node Carrier lacks connection contract")
			}
			if err := conn.SetDeadline(end); err != nil {
				t.Fatal(err)
			}
			outer, err := a.FreshHello(end, ardp.PurposeForwarding, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := framing.SendHello(conn, outer); err != nil {
				t.Fatal(err)
			}
			if err := framing.Accepted(conn); err != nil {
				t.Fatal(err)
			}
			opened := ardp.Open{RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, Purpose: uint8(ardp.PurposeForwarding), Deadline: end}
			restricted, err := ardp.EncodeNodeOpen(ardp.NodeOpen{Recipient: opened, Restriction: ardp.IssuerBootstrapChild})
			if err != nil {
				t.Fatal(err)
			}
			s := framing.New(t.Context(), conn, end, 32<<20, func() error { _, err := a.Hello(outer, true); return err }, false, framing.NewBudget(1<<20), nil)
			defer s.Close()
			var restrictedLanes []*framing.Lane
			for range 4 {
				lane, err := s.Open(t.Context(), t.Context(), restricted)
				if err != nil {
					t.Fatal(err)
				}
				defer lane.Finish()
				restrictedLanes = append(restrictedLanes, lane)
			}
			if !s.Live() || admits.Load() != 0 {
				t.Fatal("bootstrap capacity reached private spend or retired shared Carrier", admits.Load())
			}
			waitUntil := time.Now().Add(2 * time.Second)
			for bootstrapHolds.Load() != 4 && time.Now().Before(waitUntil) {
				time.Sleep(time.Millisecond)
			}
			heldObservation, err := budget.Observe(t.Context())
			if err != nil || bootstrapHolds.Load() != 4 || bootstrapReturns.Load() != 0 || heldObservation.ReservedBytes == 0 {
				t.Fatal("pending TLS did not retain genuine Hosting work", err, bootstrapHolds.Load(), bootstrapReturns.Load())
			}
			// Complete TLS on one restricted child, then attempt actual private
			// admission with a genuinely issued and durably presented token.
			// Route must reject before invoking receiving Admission at all.
			restrictedTLS, err := routeFixtureRoleTLS(t, restrictedLanes[0], member.PublicKey, end)
			if err != nil {
				t.Fatal("restricted TLS setup", err)
			}
			restrictedHello, err := a.FreshHello(end, ardp.PurposeForwarding, false)
			if err != nil {
				t.Fatal(err)
			}
			presented := false
			err = role.Present(t.Context(), t.Context(), restrictedTLS, a, restrictedHello, func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
				p := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
				if err := f.authority.Presentation(p); err != nil {
					return nil, err
				}
				raw, err := holder.Take(ctx, p, 2)
				if err != nil {
					return nil, err
				}
				presented = true // Take has independently verified the signature.
				return raw, nil
			})
			if err == nil || errors.Is(err, os.ErrDeadlineExceeded) || !presented || admits.Load() != 0 || !s.Live() || !time.Now().Before(end) {
				t.Fatal("restricted valid-token admission crossed receiving boundary", err, presented, admits.Load(), s.Live())
			}
			if n, err := restrictedLanes[0].Read(make([]byte, 1)); n != 0 || !errors.Is(err, io.EOF) {
				t.Fatal("restricted refusal omitted actual peer lane termination", n, err)
			}
			// This intentional protocol violation ends at the peer's CLOSE.
			// Retain the resulting local TLS shutdown failure rather than
			// reclassifying the forbidden operation as a successful exchange.
			if err := restrictedTLS.Close(); !errors.Is(err, io.EOF) {
				t.Fatal("restricted TLS refusal lost peer-retirement failure", err)
			}
			ordinary, err := ardp.EncodeNodeOpen(ardp.NodeOpen{Recipient: opened, Restriction: ardp.OrdinaryChild})
			if err != nil {
				t.Fatal(err)
			}
			lane, err := s.Open(t.Context(), t.Context(), ordinary)
			if err != nil {
				t.Fatal(err)
			}
			defer lane.Finish()
			secured, err := routeFixtureRoleTLS(t, lane, member.PublicKey, end)
			if err != nil {
				t.Fatal("ordinary TLS failed alongside pending restricted children", err)
			}
			defer secured.Close()
			h, err := a.FreshHello(end, ardp.PurposeForwarding, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := role.Present(t.Context(), t.Context(), secured, a, h, func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
				p := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
				if err := f.authority.Presentation(p); err != nil {
					return nil, err
				}
				return holder.Take(ctx, p, 2)
			}); err != nil {
				t.Fatal("ordinary genuine admission unavailable on same Carrier", err)
			}
			if admits.Load() != 1 {
				t.Fatal("restricted allocations entered receiving Admission", admits.Load())
			}
			for _, l := range restrictedLanes {
				if err := l.Close(); err != nil {
					t.Fatal("restricted child retirement failed", err)
				}
			}
			// Keep the shared Carrier alive until the peer has completed its
			// TLS shutdown and emitted this child's actual terminal frame.
			if err := secured.CloseWrite(); err != nil {
				t.Fatal("ordinary TLS half-close failed", err)
			}
			if _, err := io.Copy(io.Discard, secured); err != nil {
				t.Fatal("ordinary TLS peer shutdown failed", err)
			}
			if n, err := lane.Read(make([]byte, 1)); n != 0 || err == nil || errors.Is(err, os.ErrDeadlineExceeded) || !s.Live() {
				t.Fatal("ordinary child did not reach its peer terminal", n, err)
			}
			if err := secured.Close(); err != nil {
				t.Fatal("ordinary TLS retirement failed", err)
			}
			if err := s.Close(); err != nil {
				t.Fatal("shared Carrier retirement failed", err)
			}
			if err := server.Close(); err != nil {
				t.Fatal("receiving mixed Carrier did not join cleanly", err)
			}
			observation, err := budget.Observe(t.Context())
			if err != nil || observation.ReservedBytes != 0 || releases.Load() != 1 || bootstrapHolds.Load() != 4 || bootstrapReturns.Load() != 4 {
				t.Fatal("joined mixed Carrier lost physical reservation", err, releases.Load(), bootstrapHolds.Load(), bootstrapReturns.Load())
			}
		})
	}
}

func releaseRouteCapacity(release func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return release(ctx)
}

func holdRouteIssuerReservation(c routereceiver.Channel, held *hosting.Reservation, work, afterTerminal func() error) (func() error, error) {
	terminal, err := held.DetachTermination()
	if err != nil {
		release, holdErr := c.HoldReservation(work)
		return release, errors.Join(err, holdErr)
	}
	return c.HoldIssuerReservation(work, func() error {
		err := releaseRouteCapacity(terminal)
		if afterTerminal != nil {
			err = errors.Join(err, afterTerminal())
		}
		return err
	})
}

func issueRouteBatch(ctx context.Context, authority admissionAuthority, plan issuer.Plan, batch []byte, bootstrap bool) ([]byte, error) {
	kind := quota.Admitted
	if bootstrap {
		kind = quota.Bootstrap
	}
	return issuer.IssueCurrentBatch(ctx, plan, batch, kind, func() (admission.AuthorityFacts, time.Time, error) {
		return authority.Issuer(plan.KeyBinding.Signer)
	})
}
