//go:build linux

package receive_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	routereceiver "github.com/dianabuilds/ardents-network/internal/domain/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/domain/route/role"
	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

type issuerReceivingNetwork struct {
	*networkinputs.SignedAdmission
	authority        funding.AdmissionAuthority
	profile          admission.AuthorityFacts
	receiver         receiving.Receiver
	clockUnavailable *atomic.Bool
}

func (f *issuerReceivingNetwork) current() (network.RuntimeView, error) {
	return f.State.CurrentRuntime()
}

func (f *issuerReceivingNetwork) successor(t *testing.T) {
	t.Helper()
	next := f.Spec
	next.Number++
	next.Previous = f.profile.StateDigest
	next.Seed[0]++
	signed, err := networkfixture.BuildClosed(next)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.State.AcceptEpoch(t.Context(), signed.Epoch.Raw, signed.Epoch.Inputs, signed.Epoch.Materials); err != nil {
		t.Fatal(err)
	}
	if _, err := f.State.AcceptClosedProfile(signed.Profile); err != nil {
		t.Fatal(err)
	}
}

func routePermissionStock(t *testing.T, f *issuerReceivingNetwork, role admission.AllocationRole, maxima [3]uint32) *stock.Owner {
	t.Helper()
	return processfixture.OpenPermittedStock(t, t.TempDir(), role, maxima, f.authority.ObserveFacts, f.profile, f.Spec.Authority, time.Now())
}

func fundingAuthority(authority funding.AdmissionAuthority) funding.Authority {
	return funding.Observe(authority.Current, authority.Witness, authority.Presentation)
}

// One genuine receiving installation: separate signed authority, spend roots
// and Hosting budgets. Gates observe actual issuance/release; none mint a Grant
// or replace IssueCurrent. Used by retirement and competing-holder scenarios.
type routeIssuerFixture struct {
	network                                                                       *issuerReceivingNetwork
	servers                                                                       []*routereceiver.Receiver
	budgets                                                                       []*hosting.Budget
	issuerBudget                                                                  *hosting.Budget
	stopIssuer                                                                    context.CancelFunc
	bootstrapHolds, bootstrapReturns, admissions, ordinaryIssues, bootstrapIssues atomic.Int32
	issuerAdmissionCalls, issuerAdmissions, issuerReturns                         atomic.Int32
	spends                                                                        []*processfixture.ReceivingSpendEvidence
}

func newRouteIssuerFixture(t *testing.T, profile transport.CarrierProfile, afterIssue func(context.Context, bool, []byte, []byte) error, afterIssuerReturn func() error, admissionPoint func(string), issuerObservation func(int), resolution ...bool) *routeIssuerFixture {
	t.Helper()
	sockets := make(map[[32]byte]func())
	certificates := make(map[[32]byte]tls.Certificate)
	signed, authority, clockUnavailable := openReceivingFixtureState(t, func(spec *networkfixture.ClosedSpec) {
		processfixture.AddRouteFixtureNodes(t, spec, sockets, certificates, "127.0.0.1", string(profile), profile == transport.ClosedCarrierTCP, 1, false, true)
		if len(resolution) != 0 && resolution[0] {
			// Signed recipient for a Stock-only oracle; no live Store is claimed.
			node := spec.Nodes[0]
			node.NodeID = [32]byte{77}
			node.PrivateKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{77}, ed25519.SeedSize))
			t.Cleanup(func() { clear(node.PrivateKey) })
			node.Endpoint, node.Carrier = "127.0.0.1:65000", string(profile)
			node.RoleDomain, node.Subrole = 2, 5
			spec.Nodes = append(spec.Nodes, node)
		}
	})
	facts, _, err := authority.ObserveFacts()
	if err != nil {
		t.Fatal(err)
	}
	f := &issuerReceivingNetwork{SignedAdmission: signed, authority: authority, profile: facts, clockUnavailable: clockUnavailable,
		receiver: receiving.Receiver{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest,
			ProfileDigest: facts.Digest, NodeID: signed.Spec.Nodes[1].NodeID, DutyGeneration: signed.Spec.Nodes[1].Generation}}
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
		spent := processfixture.ObserveReceivingSpends(t, owner, root, binding,
			func() (receiving.Observation, error) { return f.authority.Receiver(binding, member.NotAfter()) })
		x.spends = append(x.spends, spent)
		t.Cleanup(func() {
			if err := owner.Close(); err != nil {
				t.Error(err)
			}
		})
		budget := openReceivingFixtureBudget(t)
		x.budgets = append(x.budgets, budget)
		isIssuer := id == f.profile.IssuerNodeID
		if isIssuer {
			x.issuerBudget = budget
		}
		config := routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id],
			ReserveBootstrap: func(ctx context.Context, end time.Time) (func() error, error) {
				release, err := reserveReceivingFixtureWork(ctx, budget, end)
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
					held, err := budget.Reserve(ctx, hosting.ReservationRequest{
						Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10},
						WorkUntil: c.Hello.Deadline, HoldUntil: c.Hello.Deadline.Add(time.Second),
					})
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
					spent.Record(class, raw)
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
					journal := filepath.Join(f.Plan.AdmissionRoot, "admission.journal")
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
				payload, err := issueRouteBatch(ctx, authority, f.Plan, batch, bootstrap)
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
				var prefixes [2]session.Prefix
				for i := range holders {
					holders[i] = routePermissionStock(t, f, admission.AllocationUser, [3]uint32{4, 8, 0})
					local := t.TempDir()
					plan := funding.Declaration{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
					boot, err := session.OpenBootstrap(t.Context(), plan.Plan(), fundingAuthority(f.authority), holders[i])
					if err != nil {
						t.Fatal("genuine bootstrap", err)
					}
					defer boot.Close()
					if err := boot.Issue(t.Context(), 2, nil); err != nil {
						t.Fatal("forwarding stock", err)
					}
					issuer := f.profile.IssuerNodeID
					if err := boot.Issue(t.Context(), 1, [][32]byte{issuer, issuer, issuer}); err != nil {
						t.Fatal("Control stock", err)
					}
					if err := boot.Close(); err != nil {
						t.Fatal("bootstrap join", err)
					}
					prefixes[i], err = session.OpenPrefix(t.Context(), plan.Plan(), fundingAuthority(f.authority), holders[i])
					if err != nil {
						t.Fatal("fresh admitted prefix", err)
					}
					defer prefixes[i].Close()
				}
				gated.Store(true)
				caller, cancel := context.WithCancel(t.Context())
				defer cancel()
				completed := make(chan error, 1)
				go func() { completed <- prefixes[0].Issue(caller, 2, nil) }()
				select {
				case <-started:
				case err := <-completed:
					t.Fatal("issuer did not reach actual signed result", err)
				}
				if err := prefixes[1].Issue(t.Context(), 2, nil); err == nil {
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
					if err := prefixes[1].Issue(t.Context(), 2, nil); err != nil {
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
					retired = errors.Join(retired, p.Close())
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
				plan := funding.Declaration{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				boot, err := session.OpenBootstrap(t.Context(), plan.Plan(), fundingAuthority(f.authority), holder)
				if err != nil {
					t.Fatal("bootstrap", err)
				}
				defer boot.Close()
				for _, class := range []uint8{2, 1} {
					if err := boot.Issue(t.Context(), class, nil); err != nil {
						t.Fatal("genuine bootstrap stock", err)
					}
				}
				if err := boot.Close(); err != nil {
					t.Fatal("bootstrap join", err)
				}
				journalSize := func(root, name string) int64 {
					info, err := os.Stat(filepath.Join(root, name))
					if err != nil {
						t.Fatal("actual issuer journal unavailable", err)
					}
					return info.Size()
				}
				debitSize := journalSize(f.Plan.AdmissionRoot, "admission.journal")
				resultSize := journalSize(f.Plan.ResultRoot, "results.journal")
				original, err := session.OpenPrefix(t.Context(), plan.Plan(), fundingAuthority(f.authority), holder)
				if err != nil {
					t.Fatal("actual admitted original", err)
				}
				defer original.Close()
				completed := make(chan error, 1)
				go func() { completed <- original.Issue(t.Context(), 2, nil) }()
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
				committedDebit := journalSize(f.Plan.AdmissionRoot, "admission.journal")
				committedResult := journalSize(f.Plan.ResultRoot, "results.journal")
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
				terminal := original.Close() // retain genuine authority retirement, never renew it
				if repeated := original.Close(); terminal == nil && repeated != nil || terminal != nil && !errors.Is(repeated, terminal) {
					t.Error("original retirement result changed", terminal, repeated)
				}
				for _, server := range x.servers {
					terminal := server.Close()
					if repeated := server.Close(); terminal == nil && repeated != nil || terminal != nil && !errors.Is(repeated, terminal) {
						t.Error("receiving retirement result changed", terminal, repeated)
					}
				}
				if journalSize(f.Plan.AdmissionRoot, "admission.journal") != committedDebit || journalSize(f.Plan.ResultRoot, "results.journal") != committedResult {
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

func issueRouteBatch(ctx context.Context, authority funding.AdmissionAuthority, plan issuer.Plan, batch []byte, bootstrap bool) ([]byte, error) {
	kind := quota.Admitted
	if bootstrap {
		kind = quota.Bootstrap
	}
	return issuer.IssueCurrentBatch(ctx, plan, batch, kind, func() (admission.AuthorityFacts, time.Time, error) {
		return authority.Issuer(plan.KeyBinding.Signer)
	})
}
