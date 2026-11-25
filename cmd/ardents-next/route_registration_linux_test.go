//go:build linux

package main

import (
	"bytes"
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
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/route/channel"
	"github.com/dianabuilds/ardents-network/internal/route/introduction"

	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

// This uses signed current State, genuine Publisher allocation/blind issuance,
// durable Receiving and physical Hosting. No successful authority or ACK is
// substituted. Compiled command acceptance is a separate scenario.
func TestRouteGenuineRegistrationTCP(t *testing.T) {
	t.Parallel()
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		for _, mode := range []string{"withdraw", "post-spend-clock-loss"} {
			t.Run(string(profile)+"/"+mode, func(t *testing.T) {
				f, reservations, certificates := newRoleRouteFixture(t, profile, 4, true)
				holder := routeRoleStock(t, f, true)
				registrationContext, cancelRegistration := context.WithCancel(t.Context())
				defer cancelRegistration()
				var forwards, registrations atomic.Int32
				var authorityFailure error
				burntRegistration := make(chan []byte, 1)
				var registrationSpendRoot string
				var registrationReceiving *receiving.Owner
				var registrationBinding receiving.Receiver
				var servers []*routereceiver.Receiver
				var budgets []*hosting.Budget
				for i := byte(12); i <= 16; i++ {
					id := [32]byte{i}
					view, err := f.current()
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
					binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: id, DutyGeneration: 9}
					spendRoot := t.TempDir()
					owner, err := receiving.Open(spendRoot, binding, func() (receiving.Observation, error) { return f.authority.Receiver(binding, member.NotAfter()) })
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := owner.Close(); err != nil {
							t.Error(err)
						}
					})
					budget := networkTestBudget(t)
					budgets = append(budgets, budget)
					config := routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id]}
					if i == 16 {
						registrationSpendRoot, registrationReceiving, registrationBinding = spendRoot, owner, binding
						registrationHistoryRoot := t.TempDir()
						if err := os.Chmod(registrationHistoryRoot, 0700); err != nil {
							t.Fatal(err)
						}
						config.Introduction = prepareIntroductionRegistry(t, registrationHistoryRoot, owner, binding)
					}
					config.Admit = func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
						class := admission.ForwardClass
						if channel.Hello.Purpose == ardp.PurposeIntroduction {
							class = admission.RegistrationClass
						}
						grant, err := owner.Accept(ctx, class, raw, channel.Hello.Deadline, func() (func() error, error) {
							release, err := networkTestReservation(t, budget, channel.Hello.Deadline)
							if err != nil {
								return nil, err
							}
							return channel.HoldReservation(release)
						})
						if err == nil {
							if class == admission.RegistrationClass {
								registrations.Add(1)
								if mode == "post-spend-clock-loss" {
									burntRegistration <- bytes.Clone(raw)
									f.clockUnavailable.Store(true)
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
				plan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 4, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				prefix, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
				if err != nil {
					t.Fatal("prefix", err)
				}
				defer prefix.close()
				registration, err := prefix.register(registrationContext, 1)
				if mode == "post-spend-clock-loss" {
					if err == nil || forwards.Load() != 2 || registrations.Load() != 1 {
						t.Fatal("authority loss did not refuse after actual registration spend", err)
					}
					_, authorityFailure = f.current()
					if authorityFailure == nil {
						t.Fatal("genuine Network accepted unavailable clock")
					}
					_ = prefix.close()
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
					f.clockUnavailable.Store(false)
					if err := registrationReceiving.Close(); err != nil {
						t.Fatal(err)
					}
					reopened, err := receiving.Open(registrationSpendRoot, registrationBinding, func() (receiving.Observation, error) {
						return f.authority.Receiver(registrationBinding, f.profile.NotAfter)
					})
					if err != nil {
						t.Fatal(err)
					}
					defer reopened.Close()
					raw := <-burntRegistration
					defer clear(raw)
					deadline := time.Now().Add(5 * time.Second)
					grant, err := reopened.Accept(t.Context(), admission.RegistrationClass, raw, deadline, func() (func() error, error) {
						return networkTestReservation(t, budgets[len(budgets)-1], deadline)
					})
					if err == nil {
						_ = grant.Release()
						t.Fatal("post-spend failed registration refunded its token across reopen")
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
				defer registration.close()
				if registration.slot == [32]byte{} || forwards.Load() != 2 || registrations.Load() != 1 {
					t.Fatal("missing genuine accepted operations", forwards.Load(), registrations.Load())
				}
				select {
				case <-registration.done:
					t.Fatal("registration already retired")
				default:
				}
				checkPublicationRetired := exerciseRegisteredInstanceRecord(t, registration.original)
				if err := registration.withdraw(t.Context()); err != nil {
					t.Fatal("WITHDRAW", err)
				}
				checkPublicationRetired()
				select {
				case <-registration.done:
				default:
					t.Fatal("joined registration did not retire")
				}
				if err := registration.close(); err != nil {
					t.Fatal("retained child close changed original result", err)
				}
				if err := prefix.close(); err != nil {
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
}

// Every retained branch must be the observed genuine Network failure or an
// independently proven started peer-retired write. An unrelated sibling error
// cannot be hidden by errors.Is matching only the authority branch.
func registrationAuthorityRetirementOnly(err, authorityFailure error) bool {
	if err == nil || authorityFailure == nil {
		return false
	}
	if err == authorityFailure || framing.TerminalFailureStage(err) == "peer-retired-write" {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !registrationAuthorityRetirementOnly(child, authorityFailure) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return registrationAuthorityRetirementOnly(wrapped.Unwrap(), authorityFailure)
	}
	return false
}

func TestRegistrationAuthorityRetirementRejectsUnrelatedFailure(t *testing.T) {
	authorityFailure := errors.New("observed Network refusal")
	unrelated := errors.New("unrelated release failure")
	if !registrationAuthorityRetirementOnly(errors.Join(authorityFailure, authorityFailure), authorityFailure) {
		t.Fatal("exact original Network refusal rejected")
	}
	for _, err := range []error{nil, unrelated, errors.Join(authorityFailure, unrelated), errors.New(authorityFailure.Error())} {
		if registrationAuthorityRetirementOnly(err, authorityFailure) {
			t.Fatal("unrelated failure concealed", err)
		}
	}
}

// prepareIntroductionRegistry uses the genuine receiving birth fact and actual
// independent History; cleanup is registered before the borrowing Receiver.
func prepareIntroductionRegistry(t *testing.T, root string, owner *receiving.Owner, binding receiving.Receiver) *introduction.Registry {
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
