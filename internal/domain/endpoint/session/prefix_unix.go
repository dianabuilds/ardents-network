//go:build linux || darwin

package session

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/route/introduction"
	routeprefix "github.com/dianabuilds/ardents-network/internal/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
)

func OpenPrefix(ctx context.Context, plan funding.Plan, authority funding.Authority, holder *stock.Owner) (_ Prefix, result error) {
	if !authority.Available() {
		return Prefix{}, errors.New("route requires opened Network")
	}
	if !pathgrammar.DisjointRoots(plan.EntryRoot, plan.InteriorRoot, plan.HostingRoot) {
		return Prefix{}, errors.New("route roots invalid")
	}
	selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: plan.Domain, Current: authority.Current, Exclusions: plan.Exclusions})
	if err != nil {
		return Prefix{}, err
	}
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		return Prefix{}, errors.Join(err, selected.Close())
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, selected.Close(), budget.Close())
		}
	}()
	leg, err := selected.Select()
	if err != nil {
		return Prefix{}, err
	}
	reservation, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: plan.Deadline, HoldUntil: plan.Deadline.Add(5 * time.Second)})
	if err != nil {
		return Prefix{}, err
	}
	var releaseOnce sync.Once
	var releaseErr error
	release := func() error {
		releaseOnce.Do(func() {
			releaseErr = errors.Join(reservation.ReleaseJoined(), selected.Close(), budget.Close())
		})
		return releaseErr
	}
	prefix, err := routeprefix.Open(ctx, routeprefix.Config{Leg: leg, Current: authority.Current, Deadline: plan.Deadline, Release: release,
		HoldRefill: func(ctx context.Context, additional uint64) (func() error, error) {
			limit := admission.ForwardClass.ByteLimit()
			if err := budget.CoversJoint(ctx, reservation, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
				return nil, err
			}
			held, err := budget.ReserveAdditionalJoint(ctx, reservation, hosting.JointTraffic{Tx: additional, Rx: additional, Total: additional})
			if err != nil {
				return nil, err
			}
			return held.ReleaseJoined, nil
		},
		Present: func(ctx context.Context, h ardp.Hello) ([]byte, error) {
			presentation := stock.Presentation{NetworkID: h.NetworkID, StateGeneration: h.StateGeneration, StateDigest: h.StateDigest, ProfileDigest: h.ProfileDigest, RecipientNodeID: h.RecipientNodeID, RecipientDutyGeneration: h.RecipientDutyGeneration, ChannelNonce: h.ChannelNonce, Deadline: h.Deadline}
			if err := authority.Presentation(presentation); err != nil {
				return nil, err
			}
			class, err := role.AdmissionClass(h.Purpose)
			if err != nil {
				return nil, err
			}
			return holder.Take(ctx, presentation, uint8(class))
		}})
	if err != nil {
		return Prefix{}, errors.Join(err, release())
	}
	submit := func(ctx context.Context, recipient Recipient, envelope capsule.Envelope) error {
		if err := prefix.CheckSourceScope(leg.Profile.Network, leg.Profile.Digest, plan.Deadline); err != nil {
			return err
		}
		duty, err := prefix.ResolveSubmission(recipient.Node, plan.Exclusions)
		if err != nil || duty.RecordGeneration != recipient.Generation {
			return errors.Join(errors.New("exact submission recipient generation unavailable"), err)
		}
		return introduction.Submit(ctx, prefix, duty, envelope, plan.Exclusions)
	}
	return Prefix{close: prefix.Close, done: prefix.Done(), replenish: prefix.Replenish,
		sealSubmitCapsule: func(ctx context.Context, recipient Recipient, input CapsuleIntent) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			request, err := capsule.ParseRequest(input.Request)
			if err != nil {
				return err
			}
			envelope, _, err := capsule.Seal(capsule.Header{Slot: input.Slot, DeliveryNonce: input.DeliveryNonce, Revision: input.Revision, Expiry: input.Expiry}, input.RecipientKey, request)
			if err != nil {
				return err
			}
			return submit(ctx, recipient, envelope)
		},
		submitCapsule: func(ctx context.Context, recipient Recipient, raw []byte) error {
			envelope, err := capsule.Parse(raw)
			if err != nil {
				return err
			}
			return submit(ctx, recipient, envelope)
		},
		publishDescriptor: func(ctx context.Context, raw []byte) error {
			end := minRouteDeadline(plan.Deadline, time.Now().Add(admission.ControlClass.Lifetime()).UTC().Truncate(time.Second))
			return prefix.PublishDescriptor(ctx, end, raw, plan.Exclusions)
		},
		lookupDescriptor: func(ctx context.Context, target [32]byte, history *reachability.History) ([]byte, error) {
			end := minRouteDeadline(plan.Deadline, time.Now().Add(admission.ControlClass.Lifetime()).UTC().Truncate(time.Second))
			proof, err := prefix.LookupDescriptor(ctx, end, target, history, plan.Exclusions)
			if err != nil {
				return nil, err
			}
			return proof.Bytes(), nil
		}, issue: func(ctx context.Context, class uint8, receivers [][32]byte) error {
			end := minRouteDeadline(plan.Deadline, time.Now().Add(admission.ControlClass.Lifetime()).UTC().Truncate(time.Second))
			return prefix.Issue(ctx, end, funding.IssuerPreparation(ctx, authority, holder, leg, class, receivers))
		}, register: func(ctx context.Context, revision uint64) (Registration, error) {
			view, err := authority.Current()
			if err != nil {
				return Registration{}, err
			}
			duty, err := leg.IntroductionDuty(view, plan.Exclusions)
			if err != nil {
				return Registration{}, err
			}
			end := minRouteDeadline(plan.Deadline, view.ProfileBinding().NotAfter, duty.RecordValidUntil, duty.Epoch.ValidUntil, time.Now().Add(admission.RegistrationClass.Lifetime()).UTC().Truncate(time.Second))
			if bound, exists := ctx.Deadline(); exists && bound.Before(end) {
				end = bound.UTC().Truncate(time.Second)
			}
			registration, err := introduction.Register(ctx, prefix, introduction.RegistrationConfig{Duty: duty, Revision: revision, Deadline: end})
			if err != nil {
				return Registration{}, err
			}
			receipt, err := registration.Receipt()
			if err != nil {
				return Registration{}, errors.Join(err, registration.Close())
			}
			facts := receipt.Facts()
			return Registration{owner: registration, facts: facts}, nil
		}}, nil
}
