//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	routeprefix "github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	routereceiver "github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"

	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// This uses signed current State, genuine Publisher allocation/blind issuance,
// durable Receiving and physical Hosting. No successful authority or ACK is
// substituted. Compiled command acceptance is a separate scenario.
func TestRouteGenuineRegistrationBothCarriers(t *testing.T) {
	t.Parallel()
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		for _, mode := range []string{"withdraw", "withdraw-deferred-caller", "withdraw-deferred-stop", "prefix-close", "caller-cancel", "caller-deferred-spend", "caller-deferred-presentation", "expiry", "clock-loss", "post-spend-clock-loss", "lost-floor"} {
			t.Run(string(profile)+"/"+mode, func(t *testing.T) {
				f, reservations, certificates := newRoleRouteFixture(t, profile, 4, true)
				holder := routeRoleStock(t, f, true)
				registrationContext, cancelRegistration := context.WithCancel(t.Context())
				defer cancelRegistration()
				deferredCaller := &registrationDeferredCaller{Context: registrationContext}
				if mode == "caller-deferred-spend" || mode == "caller-deferred-presentation" {
					registrationContext = deferredCaller
				}
				var forwards, registrations atomic.Int32
				var registrationAttempts, presentations atomic.Int32
				var burntPresentation stock.Presentation
				var authorityFailure error
				burntRegistration := make(chan []byte, 1)
				var registrationSpendRoot string
				var registrationReceiving *receiving.Owner
				var registrationBinding receiving.Receiver
				var registrationReceiverConfig routereceiver.ReceiverConfig
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
					owner, err := receiving.Open(spendRoot, binding, func() (receiving.Observation, error) { return f.authority.receiver(binding, member.NotAfter()) })
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
						config.IntroductionRoot = t.TempDir()
						if err := os.Chmod(config.IntroductionRoot, 0700); err != nil {
							t.Fatal(err)
						}
						config.Receiving = owner
					}
					config.Admit = func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
						class := admission.ForwardClass
						if channel.Hello.Purpose == ardp.PurposeIntroduction {
							class = admission.RegistrationClass
							registrationAttempts.Add(1)
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
								if mode == "caller-deferred-spend" {
									cancelRegistration()
								}
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
					if i == 16 {
						registrationReceiverConfig = config
					}
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
				plan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: routeProcessBudget(t), Domain: 4, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				var prefix routeHandle
				var err error
				if mode == "caller-deferred-presentation" {
					prefix, err = registrationPresentationPrefix(t.Context(), plan, f, holder, func(presentation stock.Presentation) {
						burntPresentation = presentation
						presentations.Add(1)
						cancelRegistration()
					})
				} else {
					prefix, err = startRoutePrefix(t.Context(), plan, f.authority, holder)
				}
				if err != nil {
					t.Fatal("prefix", err)
				}
				defer prefix.close()
				var registrationEnd time.Time
				if mode == "expiry" {
					cancelRegistration()
					registrationEnd = time.Now().Add(4 * time.Second).UTC().Truncate(time.Second)
					registrationContext, cancelRegistration = context.WithDeadline(t.Context(), registrationEnd)
				}
				defer cancelRegistration()
				registration, err := prefix.register(registrationContext, 1)
				if mode == "caller-deferred-spend" || mode == "caller-deferred-presentation" {
					if err == nil {
						_ = registration.close()
						t.Fatal("canceled original caller accepted before its cancellation callback ran")
					}
					wantSpends := int32(1)
					if mode == "caller-deferred-presentation" {
						wantSpends = 0
						if presentations.Load() != 1 || registrationAttempts.Load() != 0 {
							t.Fatal("cancellation after genuine presentation emitted token bytes", presentations.Load(), registrationAttempts.Load())
						}
						raw, replayErr := holder.Take(t.Context(), burntPresentation, 3)
						clear(raw)
						if replayErr == nil {
							t.Fatal("canceled presentation restored holder stock")
						}
					}
					if !errors.Is(err, context.Canceled) || forwards.Load() != 2 || registrations.Load() != wantSpends || deferredCaller.stopped.Load() != 1 {
						t.Fatal("missing synchronous original caller refusal and joined callback", err, forwards.Load(), registrations.Load(), deferredCaller.stopped.Load())
					}
					if err := prefix.close(); err != nil {
						t.Fatal("prefix join after canceled caller", err)
					}
					for _, server := range servers {
						if err := server.Close(); err != nil && framing.TerminalFailureStage(err) != "peer-retired-write" {
							t.Fatal("receiver join after canceled caller", err)
						}
					}
					if mode == "caller-deferred-presentation" && registrationAttempts.Load() != 0 {
						t.Fatal("late Receiving attempt after joined cancellation")
					}
					for _, budget := range budgets {
						observation, err := budget.Observe(t.Context())
						if err != nil || observation.ReservedBytes != 0 {
							t.Fatal("canceled original caller retained joined reservation", observation.ReservedBytes, err)
						}
					}
					return
				}
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
						return f.authority.receiver(registrationBinding, f.profile.NotAfter)
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
				facts := registration.facts
				if facts.Slot != registration.slot || facts.Node != ([32]byte{16}) || facts.Network != f.profile.NetworkID || facts.Profile != f.profile.Digest || facts.Revision != 1 || (!registrationEnd.IsZero() && !facts.Expiry.Equal(registrationEnd)) || facts.Expiry.After(plan.Deadline) || facts.Created.IsZero() || !facts.Created.Before(time.Now()) || !facts.Created.Before(facts.Expiry) || facts.Acknowledgement == ([32]byte{}) {
					t.Fatal("receipt differs from actual registration", facts)
				}
				select {
				case <-registration.done:
					t.Fatal("registration already retired")
				default:
				}
				var terminal error
				if mode == "caller-cancel" || mode == "expiry" || mode == "clock-loss" {
					if mode == "caller-cancel" {
						cancelRegistration()
					} else if mode == "clock-loss" {
						select {
						case <-prefix.done:
							t.Fatal("prefix retired before the authority-loss event")
						default:
						}
						f.clockUnavailable.Store(true)
						_, authorityFailure = f.current()
						if authorityFailure == nil {
							t.Fatal("genuine Network remained available without clock confidence")
						}
					} else {
						if !time.Now().Before(registrationEnd.Add(-200 * time.Millisecond)) {
							t.Fatal("setup consumed the live expiry observation window")
						}
						select {
						case <-registration.done:
							t.Fatal("registration retired before its original deadline")
						case <-time.After(time.Until(registrationEnd.Add(-200 * time.Millisecond))):
						}
					}
					select {
					case <-registration.done:
					case <-time.After(2 * time.Second):
						t.Fatal("original caller cancellation did not retire registration")
					}
					terminal = registration.close()
					if terminal == nil {
						t.Fatal("interrupted original registration reported graceful completion")
					}
				} else if mode == "withdraw-deferred-caller" || mode == "withdraw-deferred-stop" {
					withdrawContext, cancelWithdraw := context.WithCancel(t.Context())
					defer cancelWithdraw()
					delayed := &registrationDeferredCaller{Context: withdrawContext}
					var caller context.Context = &withdrawCancelAfterObservation{registrationDeferredCaller: delayed, cancel: cancelWithdraw}
					if mode == "withdraw-deferred-stop" {
						caller = &withdrawCancelAtStop{registrationDeferredCaller: delayed, cancel: cancelWithdraw}
					}
					err := registration.withdraw(caller)
					terminal = registration.close()
					if !errors.Is(err, context.Canceled) || !errors.Is(terminal, context.Canceled) || delayed.stopped.Load() != 1 {
						t.Fatalf("WITHDRAW lost original caller cancellation before delayed callback: operation=%v retained=%v stopped=%d", err, terminal, delayed.stopped.Load())
					}
				} else if mode == "withdraw" || mode == "lost-floor" {
					if err := registration.withdraw(t.Context()); err != nil {
						t.Fatal("WITHDRAW", err)
					}
				} else if err := prefix.close(); err != nil {
					t.Fatal("prefix joined registration", err)
				}
				select {
				case <-registration.done:
				default:
					t.Fatal("joined registration did not retire")
				}
				if err := registration.close(); err != terminal {
					t.Fatal("retained child close changed original result", err, terminal)
				}
				prefixTerminal := prefix.close()
				if mode == "clock-loss" {
					// Receiving withdrawal can physically retire the peer before
					// the holder's next observation. Preserve that original failed
					// result rather than inventing a later local authority cause.
					if prefixTerminal == nil || prefix.close() != prefixTerminal {
						t.Fatal("prefix lost or replaced its joined failure", prefixTerminal)
					}
				} else if prefixTerminal != nil {
					t.Fatal("prefix close", prefixTerminal)
				}
				for index, server := range servers {
					if mode == "clock-loss" {
						select {
						case <-server.Done():
						case <-time.After(2 * time.Second):
							t.Fatal("receiving duty survived genuine authority loss")
						}
					}
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
					if mode == "clock-loss" && !errors.Is(first, authorityFailure) {
						t.Fatal("receiver lost its original Network refusal", first)
					}
				}
				for _, budget := range budgets {
					observation, err := budget.Observe(t.Context())
					if err != nil || observation.ReservedBytes != 0 {
						t.Fatal("joined work retains reservation", observation.ReservedBytes, err)
					}
				}
				if mode == "lost-floor" {
					if err := registrationReceiving.Close(); err != nil {
						t.Fatal(err)
					}
					root := registrationReceiverConfig.IntroductionRoot
					// Remove only the two known members of this test's own private
					// root, then its empty directory; recreate no retained evidence.
					for _, name := range []string{"closed-introduction-slots", ".ardents-introduction-lock"} {
						if err := os.Remove(filepath.Join(root, name)); err != nil {
							t.Fatal(err)
						}
					}
					if err := os.Remove(root); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(root, 0700); err != nil {
						t.Fatal(err)
					}
					reopened, err := receiving.Open(registrationSpendRoot, registrationBinding, func() (receiving.Observation, error) {
						return f.authority.receiver(registrationBinding, f.profile.NotAfter)
					})
					if err != nil {
						t.Fatal(err)
					}
					defer reopened.Close()
					registrationReceiverConfig.Receiving = reopened
					registrationReceiverConfig.Admit = func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
						return reopened.Accept(ctx, admission.RegistrationClass, raw, channel.Hello.Deadline, func() (func() error, error) {
							release, err := networkTestReservation(t, budgets[len(budgets)-1], channel.Hello.Deadline)
							if err != nil {
								return nil, err
							}
							return channel.HoldReservation(release)
						})
					}
					if server, err := routereceiver.Listen(t.Context(), registrationReceiverConfig); err == nil {
						_ = server.Close()
						t.Fatal("lost entire slot floor after genuine spend restarted receiving")
					}
					members, err := os.ReadDir(root)
					if err != nil || len(members) != 0 {
						t.Fatal("retained Admission initialized replacement slot history", err)
					}
				}
			})
		}
	}
}

// The underlying genuine caller cancels normally; only its registered callback
// is deliberately left unscheduled. This isolates the effect-boundary gap,
// without replacing signed authority, token verification, spending or operation.
type registrationDeferredCaller struct {
	context.Context
	stopped atomic.Int32
}

// Capture one genuine live observation, then cancel its original context before
// the next effect. Only callback scheduling is delayed; this supplies no ACK,
// signed State, token, spend or successful registration substitute.
type withdrawCancelAfterObservation struct {
	*registrationDeferredCaller
	cancel   context.CancelFunc
	observed atomic.Bool
}

func (c *withdrawCancelAfterObservation) Err() error {
	err := c.Context.Err()
	if err == nil && c.observed.CompareAndSwap(false, true) {
		c.cancel()
	}
	return err
}

// The original caller is canceled during completion of its callback stop,
// after the genuine request/ACK exchange. Return must observe that caller
// before publishing success; successful ACK is supplied by the real Registry.
type withdrawCancelAtStop struct {
	*registrationDeferredCaller
	cancel context.CancelFunc
}

func (c *withdrawCancelAtStop) AfterFunc(func()) func() bool {
	var once sync.Once
	return func() bool {
		stopped := false
		once.Do(func() { c.stopped.Add(1); c.cancel(); stopped = true })
		return stopped
	}
}

// This harness uses the same real selection, Stock, transport and Hosting
// contracts as the command. Its only scheduling control cancels after genuine
// durable presentation; it supplies no successful authority or receiving ACK.
func registrationPresentationPrefix(ctx context.Context, plan routePrefixPlan, f *networkAdmissionFixture, holder *stock.Owner, afterPresentation func(stock.Presentation)) (routeHandle, error) {
	selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: plan.Domain, Current: f.current, Exclusions: plan.Exclusions})
	if err != nil {
		return routeHandle{}, err
	}
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		return routeHandle{}, errors.Join(err, selected.Close())
	}
	leg, err := selected.Select()
	if err != nil {
		return routeHandle{}, errors.Join(err, selected.Close(), budget.Close())
	}
	held, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: plan.Deadline, HoldUntil: plan.Deadline.Add(5 * time.Second)})
	if err != nil {
		return routeHandle{}, errors.Join(err, selected.Close(), budget.Close())
	}
	var once sync.Once
	var releaseErr error
	release := func() error {
		once.Do(func() { releaseErr = errors.Join(releaseRouteReservation(held), selected.Close(), budget.Close()) })
		return releaseErr
	}
	prefix, err := routeprefix.Open(ctx, routeprefix.Config{Leg: leg, Current: f.current, Deadline: plan.Deadline, Release: release,
		Present: func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
			presentation := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
			if err := f.authority.presentation(presentation); err != nil {
				return nil, err
			}
			class := uint8(2)
			if hello.Purpose == ardp.PurposeIntroduction {
				class = 3
			}
			raw, err := holder.Take(ctx, presentation, class)
			if err == nil && class == 3 {
				afterPresentation(presentation)
			}
			return raw, err
		}})
	if err != nil {
		return routeHandle{}, errors.Join(err, release())
	}
	return routeHandle{close: prefix.Close, done: prefix.Done(), register: func(ctx context.Context, revision uint64) (routeRegistration, error) {
		view, err := f.current()
		if err != nil {
			return routeRegistration{}, err
		}
		duty, err := leg.IntroductionDuty(view, plan.Exclusions)
		if err != nil {
			return routeRegistration{}, err
		}
		registration, err := introduction.Register(ctx, prefix, introduction.RegistrationConfig{Duty: duty, Revision: revision, Deadline: plan.Deadline})
		if err != nil {
			return routeRegistration{}, err
		}
		return routeRegistration{close: registration.Close, withdraw: registration.Withdraw, done: registration.Done(), slot: registration.Slot()}, nil
	}}, nil
}

func (c *registrationDeferredCaller) Value(any) any { return nil }
func (c *registrationDeferredCaller) AfterFunc(func()) func() bool {
	var once sync.Once
	return func() bool {
		stopped := false
		once.Do(func() { c.stopped.Add(1); stopped = true })
		return stopped
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
