//go:build linux

package node_test

import (
	"context"
	"errors"
	routereceiver "github.com/dianabuilds/ardents-network/internal/route/receiver"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/route/channel"
	"github.com/dianabuilds/ardents-network/internal/route/introduction"

	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

// This uses signed current State, genuine Publisher allocation/blind issuance,
// durable Receiving and physical Hosting. No successful authority or ACK is
// substituted. Compiled command acceptance is a separate scenario.
func TestRouteGenuineRegistrationTCP(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"withdraw", "post-spend-clock-loss"} {
		t.Run(mode, func(t *testing.T) {
			fixture, authority, clockUnavailable, reservations, certificates := openRouteReceivingFixture(t, 4, true, false)
			current := fixture.State.CurrentRuntime
			profile, _, err := authority.ObserveFacts()
			if err != nil {
				t.Fatal(err)
			}
			holder := receivingFixtureStock(t, fixture, authority, 1, true)
			registrationContext, cancelRegistration := context.WithCancel(t.Context())
			defer cancelRegistration()
			var forwards, registrations atomic.Int32
			var authorityFailure error
			var spendEvidence *processfixture.ReceivingSpendEvidence
			var servers []*routereceiver.Receiver
			var budgets []*hosting.Budget
			for i := byte(12); i <= 16; i++ {
				id := [32]byte{i}
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
				binding := receiving.Receiver{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest, ProfileDigest: profile.Digest, NodeID: id, DutyGeneration: 9}
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
				budgets = append(budgets, budget)
				config := routereceiver.ReceiverConfig{Authority: role.Authority{Current: current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id]}
				if i == 16 {
					if mode == "post-spend-clock-loss" {
						spendEvidence = processfixture.ObserveReceivingSpends(t, owner, spendRoot, binding, func() (receiving.Observation, error) {
							return authority.Receiver(binding, profile.NotAfter)
						})
					}
					registrationHistoryRoot := t.TempDir()
					if err := os.Chmod(registrationHistoryRoot, 0700); err != nil {
						t.Fatal(err)
					}
					config.Introduction = openRegistrationFixtureRegistry(t, registrationHistoryRoot, owner, binding)
				}
				config.Admit = func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
					class := admission.ForwardClass
					if channel.Hello.Purpose == ardp.PurposeIntroduction {
						class = admission.RegistrationClass
					}
					grant, err := owner.Accept(ctx, class, raw, channel.Hello.Deadline, func() (func() error, error) {
						release, err := reserveReceivingFixtureWork(ctx, budget, channel.Hello.Deadline)
						if err != nil {
							return nil, err
						}
						return channel.HoldReservation(release)
					})
					if err == nil {
						if class == admission.RegistrationClass {
							registrations.Add(1)
							if mode == "post-spend-clock-loss" {
								spendEvidence.Record(class, raw)
								clockUnavailable.Store(true)
							}
						} else {
							forwards.Add(1)
						}
					}
					return grant, err
				}
				reservations[id]()
				server, err := routereceiver.Listen(t.Context(), config)
				if err != nil {
					t.Fatal(err)
				}
				servers = append(servers, server)
			}
			defer func() {
				for _, server := range servers {
					if err := server.Close(); err != nil && framing.TerminalFailureStage(err) != "peer-retired-write" && !registrationAuthorityRetirementOnly(err, authorityFailure) {
						t.Error("receiving join", err)
					}
				}
			}()
			root := t.TempDir()
			plan := funding.Plan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 4, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			prefix, err := session.OpenPrefix(t.Context(), plan, funding.Observe(authority.Current, authority.Witness, authority.Presentation), holder)
			if err != nil {
				t.Fatal("prefix", err)
			}
			defer prefix.Close()
			registered, err := prefix.Register(registrationContext, 1)
			if mode == "post-spend-clock-loss" {
				if err == nil || forwards.Load() != 2 || registrations.Load() != 1 {
					t.Fatal("authority loss did not refuse after actual registration spend", err)
				}
				_, authorityFailure = current()
				if authorityFailure == nil {
					t.Fatal("genuine Network accepted unavailable clock")
				}
				_ = prefix.Close()
				for _, server := range servers {
					select {
					case <-server.Done():
					case <-time.After(2 * time.Second):
						t.Fatal("post-spend authority loss did not join receiver")
					}
					first := server.Close()
					if !errors.Is(first, authorityFailure) || !registrationAuthorityRetirementOnly(first, authorityFailure) || server.Close() != first {
						t.Fatal("receiver lost or concealed post-spend authority failure", first)
					}
				}
				clockUnavailable.Store(false)
				if spendEvidence == nil {
					t.Fatal("registration spend observer unavailable")
				}
				replayed := spendEvidence.RequireRetained(t, t.Context(), func(deadline time.Time) (func() error, error) {
					return reserveReceivingFixtureWork(t.Context(), budgets[len(budgets)-1], deadline)
				})
				if replayed != 1 {
					t.Fatalf("reopened %d Registration spends, want one", replayed)
				}
				for _, budget := range budgets {
					observation, err := budget.Observe(t.Context())
					if err != nil || observation.ReservedBytes != 0 {
						t.Fatal("post-spend join or refused replay leaked reserve", observation.ReservedBytes, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal("REGISTER", err)
			}
			registration, ok := registered.Owner().(*introduction.HolderRegistration)
			if !ok {
				t.Fatal("missing original Registration owner")
			}
			defer registration.Close()
			if registration.Slot() == [32]byte{} || forwards.Load() != 2 || registrations.Load() != 1 {
				t.Fatal("missing genuine accepted operations", forwards.Load(), registrations.Load())
			}
			select {
			case <-registration.Done():
				t.Fatal("registration already retired")
			default:
			}
			checkPublicationRetired := exerciseRegisteredInstanceRecord(t, registration)
			if err := registration.Withdraw(t.Context()); err != nil {
				t.Fatal("WITHDRAW", err)
			}
			checkPublicationRetired()
			select {
			case <-registration.Done():
			default:
				t.Fatal("joined registration did not retire")
			}
			if err := registration.Close(); err != nil {
				t.Fatal("retained child close changed original result", err)
			}
			if err := prefix.Close(); err != nil {
				t.Fatal("prefix close", err)
			}
			for index, server := range servers {
				first := server.Close()
				if repeated := server.Close(); repeated != first {
					t.Fatal("receiving terminal result changed")
				}
				if first != nil {
					if framing.TerminalFailureStage(first) != "peer-retired-write" && !registrationAuthorityRetirementOnly(first, authorityFailure) {
						t.Fatalf("receiver %d close: %v", index+12, first)
					}
					t.Logf("receiver %d retained failed outcome: %v", index+12, first)
				}
			}
			for _, budget := range budgets {
				observation, err := budget.Observe(t.Context())
				if err != nil || observation.ReservedBytes != 0 {
					t.Fatal("joined work retains reservation", observation.ReservedBytes, err)
				}
			}

		})
	}
}

// Every retained branch must be the observed genuine Network failure or an
// independently proven started peer-retired write. An unrelated sibling error
// cannot be hidden by errors.Is matching only the authority branch.
func registrationAuthorityRetirementOnly(err, authorityFailure error) bool {
	if authorityFailure == nil {
		return false
	}
	return processfixture.OnlyFailureBranches(err, func(branch error) bool {
		return branch == authorityFailure || framing.TerminalFailureStage(branch) == "peer-retired-write"
	})
}

func TestRegistrationAuthorityRetirementRejectsUnrelatedFailure(t *testing.T) {
	authority := errors.New("observed Network refusal")
	unrelated := errors.New("unrelated cleanup failure")
	if !registrationAuthorityRetirementOnly(errors.Join(authority, authority), authority) {
		t.Fatal("joined original Network refusal was not recognized")
	}
	for _, err := range []error{nil, unrelated, errors.Join(authority, unrelated), errors.New(authority.Error())} {
		if registrationAuthorityRetirementOnly(err, authority) {
			t.Errorf("classified unrelated or reconstructed failure as authority retirement: %v", err)
		}
	}
}

// openRegistrationFixtureRegistry uses the genuine receiving birth fact and actual
// independent History; cleanup is registered before the borrowing Receiver.
func openRegistrationFixtureRegistry(t *testing.T, root string, owner *receiving.Owner, binding receiving.Receiver) *introduction.Registry {
	t.Helper()
	registry, err := introduction.OpenReceivingRegistry(root, introduction.Binding{NetworkID: binding.NetworkID, ProfileDigest: binding.ProfileDigest, ReceiverNodeID: binding.NodeID, ReceiverDutyGeneration: binding.DutyGeneration}, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := registry.Close(); err != nil {
			t.Error(err)
		}
	})
	return registry
}
