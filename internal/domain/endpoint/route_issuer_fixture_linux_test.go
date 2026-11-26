//go:build linux && document_worker_installed

package endpoint_test

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	routereceiver "github.com/dianabuilds/ardents-network/internal/domain/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/domain/route/role"
	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

// The installed Execution scenario observes issuance, admission and joined Hosting on genuine receivers.
type routeIssuerFixture struct {
	network                                                                       *networkAdmissionFixture
	servers                                                                       []*routereceiver.Receiver
	budgets                                                                       []*hosting.Budget
	bootstrapHolds, bootstrapReturns, admissions, ordinaryIssues, bootstrapIssues atomic.Int32
	spends                                                                        []*processfixture.ReceivingSpendEvidence
}

func newInstalledRouteIssuerFixture(t *testing.T, profile transport.CarrierProfile, afterIssue func(context.Context, bool) error) *routeIssuerFixture {
	t.Helper()
	f, sockets, certificates := newRoleRouteFixtureConfigured(t, profile, 1, false, true, nil)
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
		budget := networkTestBudget(t)
		x.budgets = append(x.budgets, budget)
		isIssuer := id == f.profile.IssuerNodeID
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
				class, err := role.AdmissionClass(c.Hello.Purpose)
				if err != nil {
					return receiving.Grant{}, err
				}
				grant, err := owner.Accept(ctx, class, raw, c.Hello.Deadline, func() (func() error, error) {
					held, err := networkTestHold(t, budget, c.Hello.Deadline)
					if err != nil {
						return nil, err
					}
					if !isIssuer {
						return c.HoldReservation(held.ReleaseJoined)
					}
					return holdRouteIssuerReservation(c, held)
				})
				if err == nil {
					spent.Record(class, raw)
					x.admissions.Add(1)
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
				payload, err := issueRouteBatch(ctx, f.authority, f.plan, batch, bootstrap)
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
					err = afterIssue(ctx, bootstrap)
				}
				if err != nil {
					clear(payload)
					return nil, err
				}
				return payload, nil
			}
		}
		sockets[id]()
		server, err := routereceiver.Listen(t.Context(), config)
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

func holdRouteIssuerReservation(c routereceiver.Channel, held *hosting.Reservation) (func() error, error) {
	terminal, err := held.DetachTermination()
	if err != nil {
		release, holdErr := c.HoldReservation(held.ReleaseJoined)
		return release, errors.Join(err, holdErr)
	}
	return c.HoldIssuerReservation(held.ReleaseJoined, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return terminal(ctx)
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
