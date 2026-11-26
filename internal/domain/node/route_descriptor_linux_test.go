//go:build linux

package node_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
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
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	framing "github.com/dianabuilds/ardents-network/internal/route/channel"
	routereceiver "github.com/dianabuilds/ardents-network/internal/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func descriptorRouteStock(t *testing.T, fixture *networkinputs.SignedAdmission, authority funding.AdmissionAuthority, facts admission.AuthorityFacts, now time.Time) *stock.Owner {
	t.Helper()
	holder := processfixture.OpenPermittedStock(t, t.TempDir(), admission.AllocationUser, [3]uint32{8, 4, 0}, authority.ObserveFacts, facts, fixture.Spec.Authority, now)
	for i, challenges := range descriptorRouteChallenges(facts, now) {
		attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{byte(41 + i)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter})
		if err != nil {
			t.Fatal(err)
		}
		batch, _, err := attempt.Request()
		if err != nil {
			t.Fatal(err)
		}
		issued := issuer.IssueCurrent(t.Context(), fixture.Plan, batch, quota.Bootstrap, fixture.ObserveIssuer)
		if issued.Outcome != "issued-offline" {
			t.Fatal(issued)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal(err)
		}
	}
	return holder
}

func descriptorRouteChallenges(facts admission.AuthorityFacts, now time.Time) [][]token.ClosedTokenContext {
	var forwards, controls []token.ClosedTokenContext
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}} {
		forwards = append(forwards, token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: id, ReceiverDutyGeneration: 9, Class: 2, WindowStart: now.UTC().Truncate(time.Hour)})
	}
	for range 8 {
		controls = append(controls, token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: [32]byte{17}, ReceiverDutyGeneration: 9, Class: 1, WindowStart: now.UTC().Truncate(time.Hour)})
	}
	return [][]token.ClosedTokenContext{forwards, controls}
}

func TestRouteDescriptorPostCommitRetirementTCP(t *testing.T) {
	t.Run("receiving-cancellation", func(t *testing.T) { testRouteDescriptorRetirement(t, false, "after-commit") })
	t.Run("signed-successor-State", func(t *testing.T) { testRouteDescriptorRetirement(t, true, "after-commit") })
}

func TestRouteDescriptorPreCommitRetirementTCP(t *testing.T) {
	for _, phase := range []string{"before-admission", "after-reservation", "after-spend"} {
		t.Run(phase, func(t *testing.T) {
			t.Run("receiving-cancellation", func(t *testing.T) { testRouteDescriptorRetirement(t, false, phase) })
			t.Run("signed-successor-State", func(t *testing.T) { testRouteDescriptorRetirement(t, true, phase) })
		})
	}
}

func TestRouteDescriptorOriginalCallerRetirementTCP(t *testing.T) {
	for _, phase := range []string{"after-spend", "after-commit"} {
		t.Run(phase, func(t *testing.T) { testRouteDescriptorRetirement(t, false, phase, true) })
	}
}

func testRouteDescriptorRetirement(t *testing.T, stateLoss bool, phase string, callerLoss ...bool) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture, authority, _, reservations, certificates := openRouteReceivingFixture(t, 1, false, false,
				func(spec *networkfixture.ClosedSpec, reservations map[[32]byte]func(), certificates map[[32]byte]tls.Certificate) {
					processfixture.AddDescriptorFixtureNodes(t, spec, reservations, certificates, string(carrier), true)
				})
			current := fixture.State.CurrentRuntime
			facts, now, err := authority.ObserveFacts()
			if err != nil {
				t.Fatal(err)
			}
			operationContext, stopOperation := context.WithCancel(t.Context())
			defer stopOperation()
			loseCaller := len(callerLoss) != 0 && callerLoss[0]
			holder := descriptorRouteStock(t, fixture, authority, facts, now)
			storeRoot := t.TempDir()
			store, err := reachability.OpenStore(reachability.StoreConfig{Root: storeRoot, Network: facts.NetworkID})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := store.Close(); err != nil {
					t.Error(err)
				}
			})
			var controlSpends atomic.Int32
			var originalReceiving atomic.Value
			entered, resume := make(chan struct{}), make(chan struct{})
			var paused atomic.Bool
			pause := func(point string) {
				if point == phase {
					if paused.CompareAndSwap(false, true) {
						close(entered)
					}
					<-resume
					if loseCaller && phase == "after-commit" {
						// Local cancellation alone does not prove the peer has
						// retired. Keep this committed result behind the original
						// receiving cancellation, rather than racing its output
						// with CLOSE and creating a real TLS write failure.
						original, ok := originalReceiving.Load().(context.Context)
						if !ok {
							t.Error("original receiving context unavailable")
							return
						}
						<-original.Done()
						if original.Err() != context.Canceled {
							t.Errorf("original receiving cancellation missing: %v", original.Err())
						}
					}
				}
			}
			var released sync.Once
			release := func() { released.Do(func() { close(resume) }) }
			defer release()
			receivingContext, stopReceiving := context.WithCancel(t.Context())
			defer stopReceiving()
			var resolutionBudget *hosting.Budget
			var resolutionAdmission *receiving.Owner
			var resolutionRoot string
			var resolutionBinding receiving.Receiver
			var resolutionExpiry time.Time
			var presented []byte
			var presentedDeadline time.Time
			var servers []*routereceiver.Receiver
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {17}} {
				view, err := current()
				if err != nil {
					t.Fatal(err)
				}
				member, err := view.Member(id, view.ObservedAt())
				if err != nil {
					t.Fatal(err)
				}
				duty, err := view.RetainDuty(id, view.ObservedAt())
				if err != nil {
					t.Fatal(err)
				}
				binding := receiving.Receiver{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest, ProfileDigest: facts.Digest, NodeID: id, DutyGeneration: 9}
				spendRoot := t.TempDir()
				owner, err := receiving.Open(spendRoot, binding, func() (receiving.Observation, error) { return authority.Receiver(binding, member.NotAfter()) })
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := owner.Close(); err != nil {
						t.Error(err)
					}
				})
				budget := openReceivingFixtureBudget(t)
				config := routereceiver.ReceiverConfig{Authority: role.Authority{Current: current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id]}
				if id == [32]byte{17} {
					config.DescriptorStore = store
					resolutionBudget = budget
					resolutionAdmission, resolutionRoot, resolutionBinding, resolutionExpiry = owner, spendRoot, binding, member.NotAfter()
					config.Authority.Current = func() (network.RuntimeView, error) {
						// Observe only the actual committed record. This callback
						// blocks authority observation, never supplies an ACK or
						// substitutes successful Store/Network behavior.
						entries, err := os.ReadDir(filepath.Join(storeRoot, "records"))
						if err != nil {
							return network.RuntimeView{}, err
						}
						if len(entries) == 1 && len(entries[0].Name()) == 64 {
							pause("after-commit")
						}
						return current()
					}
				}
				config.Admit = func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
					class, err := role.AdmissionClass(channel.Hello.Purpose)
					if err != nil {
						return receiving.Grant{}, err
					}
					if id == [32]byte{17} {
						originalReceiving.Store(ctx)
						presented = bytes.Clone(raw)
						presentedDeadline = channel.Hello.Deadline
						pause("before-admission")
					}
					grant, err := owner.Accept(ctx, class, raw, channel.Hello.Deadline, func() (func() error, error) {
						release, err := reserveReceivingFixtureWork(ctx, budget, channel.Hello.Deadline)
						if err != nil {
							return nil, err
						}
						returned, err := channel.HoldReservation(release)
						if err == nil && id == [32]byte{17} {
							pause("after-reservation")
						}
						return returned, err
					})
					if err == nil && class == admission.ControlClass {
						controlSpends.Add(1)
						pause("after-spend")
					}
					return grant, err
				}
				reservations[id]()
				serverContext := t.Context()
				if id == [32]byte{17} {
					serverContext = receivingContext
				}
				server, err := routereceiver.Listen(serverContext, config)
				if err != nil {
					t.Fatal(err)
				}
				servers = append(servers, server)
				t.Cleanup(func() {
					if err := server.Close(); err != nil {
						if !descriptorRetirementOnly(err, stateLoss, loseCaller) {
							t.Error("receiving join", err)
						}
					}
				})
			}
			root := t.TempDir()
			plan := funding.Plan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			prefix, err := session.OpenPrefix(t.Context(), plan, funding.Observe(authority.Current, authority.Witness, authority.Presentation), holder)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				release()
				if err := prefix.Close(); err != nil {
					if !descriptorRetirementOnly(err, stateLoss, loseCaller) {
						t.Error("holder join", err)
					}
				}
			}()

			raw, target := processfixture.SignedDescriptor(t, facts, 3, [32]byte{18}, facts.Digest)
			completed := make(chan error, 1)
			go func() { completed <- prefix.PublishDescriptor(operationContext, raw) }()
			select {
			case <-entered:
			case err := <-completed:
				t.Fatal("actual Descriptor boundary not reached", phase, err)
			}
			held, err := resolutionBudget.Observe(t.Context())
			wantSpent := int32(0)
			if phase == "after-spend" || phase == "after-commit" {
				wantSpent = 1
			}
			if err != nil || (held.ReservedBytes != 0) != (phase != "before-admission") || controlSpends.Load() != wantSpent {
				t.Fatal("actual spend/reservation boundary differs", phase, err, held.ReservedBytes, controlSpends.Load())
			}
			if loseCaller {
				stopOperation()
			} else if stateLoss {
				next := fixture.Spec
				next.Number++
				next.Previous = facts.StateDigest
				next.Seed[0]++
				bundle, err := networkfixture.BuildClosed(next)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := fixture.State.AcceptEpoch(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
					t.Fatal(err)
				}
				if _, err := fixture.State.AcceptClosedProfile(bundle.Profile); err != nil {
					t.Fatal(err)
				}
			} else {
				stopReceiving()
			}
			stillHeld, err := resolutionBudget.Observe(t.Context())
			if err != nil || stillHeld.ReservedBytes != held.ReservedBytes {
				t.Error("retirement returned Hosting before original observer joined", err, stillHeld.ReservedBytes)
			}
			if t.Context().Err() != nil {
				t.Error("holder caller ended instead of receiving authority")
			}
			release()
			operationErr := <-completed
			if operationErr == nil {
				t.Error("obsolete committed Descriptor returned successful ACK")
			}
			if loseCaller && !errors.Is(operationErr, context.Canceled) {
				t.Error("original caller cancellation was not retained", operationErr)
			}
			terminal := prefix.Close()
			if repeated := prefix.Close(); terminal == nil && repeated != nil || terminal != nil && !errors.Is(repeated, terminal) {
				t.Error("holder changed retained retirement result", terminal, repeated)
			}
			for _, server := range servers {
				terminal := server.Close()
				if repeated := server.Close(); terminal == nil && repeated != nil || terminal != nil && !errors.Is(repeated, terminal) {
					t.Error("receiver changed retained retirement result", terminal, repeated)
				}
			}
			if !stateLoss {
				if err := resolutionAdmission.Close(); err != nil {
					t.Fatal(err)
				}
				reopened, err := receiving.Open(resolutionRoot, resolutionBinding, func() (receiving.Observation, error) {
					return authority.Receiver(resolutionBinding, resolutionExpiry)
				})
				if err != nil {
					t.Fatal("spent Admission root did not reopen", err)
				}
				grant, replayErr := reopened.Accept(t.Context(), admission.ControlClass, presented, presentedDeadline, func() (func() error, error) {
					return reserveReceivingFixtureWork(t.Context(), resolutionBudget, presentedDeadline)
				})
				if replayErr == nil {
					if err := grant.Release(); err != nil {
						t.Error(err)
					}
					if wantSpent != 0 {
						t.Error("cancellation refunded a spent Control token after reopen")
					}
				} else if wantSpent == 0 {
					t.Error("pre-spend refusal burned actual token", replayErr)
				} else if replayErr.Error() != "closed admission token is unavailable\nclosed token is already spent" {
					t.Error("replay refused for another reason, so no-refund is unproven", replayErr)
				}
				if err := reopened.Close(); err != nil {
					t.Error(err)
				}
			}
			after, err := resolutionBudget.Observe(t.Context())
			if err != nil || after.ReservedBytes != 0 || controlSpends.Load() != wantSpent {
				t.Error("retired operation retained work or changed spend", err, after.ReservedBytes, controlSpends.Load())
			}
			got, outcome, err := store.Lookup(target, facts.Digest, time.Now())
			wantOutcome := reachability.Stale
			var wantProof []byte
			if phase == "after-commit" {
				wantOutcome, wantProof = reachability.Accepted, raw
			}
			if err != nil || outcome != wantOutcome || !bytes.Equal(got, wantProof) {
				t.Error("retirement changed actual commit boundary", phase, outcome, err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			restored, err := reachability.OpenStore(reachability.StoreConfig{Root: storeRoot, Network: facts.NetworkID})
			if err != nil {
				t.Fatal(err)
			}
			got, outcome, lookupErr := restored.Lookup(target, facts.Digest, time.Now())
			closeErr := restored.Close()
			if lookupErr != nil || closeErr != nil || outcome != wantOutcome || !bytes.Equal(got, wantProof) {
				t.Error("reopen changed actual commit boundary", phase, outcome, lookupErr, closeErr)
			}
		})
	}
}

// This test oracle accepts only the existing leaf outcomes of this exact
// retirement scenario. An unrelated sibling failure cannot be hidden by a
// joined authority refusal or context.Canceled. These messages are diagnostic
// outcomes, never inputs that authorize runtime work.
func descriptorRetirementOnly(err error, stateLoss, callerLoss bool) bool {
	return processfixture.OnlyFailureBranches(err, func(branch error) bool {
		if branch == context.Canceled {
			return true
		}
		if (stateLoss || callerLoss) && (framing.TerminalFailureStage(branch) == "peer-retired-write" || transport.IsPeerRetirementCause(branch)) {
			return true
		}
		// Text dispositions belong to the underlying leaf, not to an
		// arbitrary wrapper that happens to repeat the same message.
		switch branch.(type) {
		case interface{ Unwrap() error }, interface{ Unwrap() []error }:
			return false
		}
		if !stateLoss {
			return false
		}
		if branch == io.EOF {
			return true
		}
		switch branch.Error() {
		case "closed profile is unavailable", "route profile changed", "retained Route authority unavailable":
			return true
		default:
			return false
		}
	})
}

func TestDescriptorRetirementOracleRejectsUnrelatedCleanup(t *testing.T) {
	authorityLoss := errors.New("closed profile is unavailable")
	if !descriptorRetirementOnly(errors.Join(authorityLoss, io.EOF, context.Canceled), true, false) {
		t.Fatal("exact authority retirement refused")
	}
	unrelated := errors.New("unrelated durable lease release failed")
	for _, err := range []error{unrelated, errors.Join(authorityLoss, unrelated), errors.Join(context.Canceled, unrelated)} {
		if descriptorRetirementOnly(err, true, false) {
			t.Fatal("unrelated cleanup hidden", err)
		}
	}
	if descriptorRetirementOnly(authorityLoss, false, true) || descriptorRetirementOnly(io.EOF, false, false) {
		t.Fatal("another scenario acquired State-loss exceptions")
	}
}
