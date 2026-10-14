//go:build linux

package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/bootstrap"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// routeTokenFlow obtains stock only for this original operation's requested
// channels. Source, selection, authority and provider budget remain real owners.
type routeTokenFlow struct {
	authority     admissionAuthority
	holder        *stock.Owner
	plan          routePrefixPlan
	leg           selection.Leg
	issuer        [32]byte
	mu            sync.Mutex
	presenting    chan struct{}
	source        *prefix.Prefix
	currentSource func(context.Context) (*prefix.SourceHandle, error)
	initial       *stock.Preparation
	retain        func(error)
}

func (flow *routeTokenFlow) challenges(class uint8, receivers [][32]byte) ([]token.ClosedTokenContext, error) {
	view, err := flow.authority.current()
	if err != nil {
		return nil, err
	}
	if view.Profile().Digest != flow.leg.Profile.Digest {
		return nil, errors.New("original Source issuing profile changed")
	}
	result := make([]token.ClosedTokenContext, 0, len(receivers))
	for _, id := range receivers {
		member, err := view.Member(id, view.ObservedAt())
		if err != nil {
			return nil, err
		}
		result = append(result, token.ClosedTokenContext{NetworkID: view.Profile().Network,
			ProfileDigest: view.Profile().Digest, IssuerNodeID: view.Profile().IssuerNodeID,
			ReceiverNodeID: id, ReceiverDutyGeneration: member.DutyGeneration, Class: class,
			WindowStart: view.ObservedAt().UTC().Truncate(time.Hour)})
	}
	return result, nil
}

func (flow *routeTokenFlow) bootstrap(ctx context.Context, budget *hosting.Budget, preparation *stock.Preparation, controlReceivers [][32]byte) (result error) {
	if len(controlReceivers) < 1 || len(controlReceivers) > admission.MaximumBatchTokens {
		return errors.New("initial issuer Control stock count invalid")
	}
	requested, err := flow.challenges(uint8(admission.ForwardClass), [][32]byte{flow.leg.Entry.NodeID, flow.leg.Interior.NodeID})
	if err != nil {
		return err
	}
	missing, err := preparation.Missing(requested)
	if err != nil || len(missing) == 0 {
		return err
	}
	if flow.holder.Status().BootstrapRemaining != 2 {
		return errors.New("original cold Source bootstrap allowance unavailable")
	}
	end := minRouteDeadline(flow.plan.Deadline, flow.leg.NotAfter, time.Now().Add(bootstrap.Lifetime).UTC().Truncate(time.Second))
	if bound, ok := ctx.Deadline(); ok {
		end = minRouteDeadline(end, bound.UTC().Truncate(time.Second))
	}
	reservation, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: flow.plan.Work, Termination: flow.plan.Termination,
		WorkUntil: end, HoldUntil: end.Add(5 * time.Second)})
	if err != nil {
		return err
	}
	physical, err := prefix.OpenBootstrap(ctx, prefix.BootstrapConfig{Leg: flow.leg, Current: flow.authority.current,
		Deadline: end, Release: func() error { return releaseRouteReservation(reservation) }})
	if err != nil {
		return errors.Join(err, releaseRouteReservation(reservation))
	}
	defer func() {
		result = errors.Join(result, physical.Close())
		cleanup, joined := physical.CleanupResult()
		if !joined {
			cleanup = errors.Join(cleanup, errors.New("original issuer bootstrap did not join"))
		}
		flow.retain(cleanup)
	}()
	var receivers [][32]byte
	for _, challenge := range missing {
		receivers = append(receivers, challenge.ReceiverNodeID)
	}
	if err := physical.Issue(ctx, end, routeIssuerPreparation(ctx, flow.authority, flow.holder, flow.leg,
		uint8(admission.ForwardClass), receivers, preparation)); err != nil {
		return err
	}
	// The second finite batch funds only the requested Control stock. It
	// reaches the issuer, never the private recipients or an upgraded child.
	return physical.Issue(ctx, end, routeIssuerPreparation(ctx, flow.authority, flow.holder, flow.leg,
		uint8(admission.ControlClass), controlReceivers, preparation))
}

func (flow *routeTokenFlow) present(ctx context.Context, hello ardp.Hello) (_ []byte, result error) {
	return flow.presentRequested(ctx, hello, [][32]byte{hello.RecipientNodeID})
}

// openingPresentation funds the two already selected forwarding recipients in
// one class-specific batch at the first actual presentation. Later refills and
// terminal roles retain their ordinary on-demand presentation. Only the missing
// stock is issued; each token is still durably presented at its own HELLO.
func (flow *routeTokenFlow) openingPresentation(leg selection.Leg) role.Presentation {
	var first sync.Once
	return func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
		opening := false
		first.Do(func() { opening = true })
		if !opening || hello.Purpose != ardp.PurposeForwarding || hello.RecipientNodeID != leg.Entry.NodeID {
			return flow.present(ctx, hello)
		}
		// The second recipient has not sent its HELLO yet. Reobserve its exact
		// retained duty before requesting stock; a changed member cannot renew
		// this opening's selection or its token challenge.
		interior := role.Authority{Current: flow.authority.current, Duty: leg.Interior, Profile: leg.Profile}
		if _, err := interior.Member(); err != nil {
			return nil, err
		}
		return flow.presentRequested(ctx, hello, [][32]byte{leg.Entry.NodeID, leg.Interior.NodeID})
	}
}

func (flow *routeTokenFlow) presentRequested(ctx context.Context, hello ardp.Hello, requestedReceivers [][32]byte) (_ []byte, result error) {
	presentation := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration,
		StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID,
		RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
	if err := flow.authority.presentation(presentation); err != nil {
		return nil, err
	}
	class, err := role.AdmissionClass(hello.Purpose)
	if err != nil {
		return nil, err
	}
	// An admitted issuer exchange consumes already funded Control. It cannot
	// recursively issue its own missing token or reset bootstrap allowance.
	if class == admission.ControlClass && hello.RecipientNodeID == flow.issuer {
		return flow.holder.Take(ctx, presentation, uint8(class))
	}
	// Stock owns atomic consumption and allows already funded presentation
	// while another original preparation issues different missing rights.
	// Every Take independently observes authority and durably marks the token.
	// Only absence before consumption may enter preparation; other failures
	// cannot trigger another attempt or reconstruct a consumed token.
	raw, takeErr := flow.holder.Take(ctx, presentation, uint8(class))
	if takeErr == nil || stock.TransferFailureStage(takeErr) != "stock" {
		return raw, takeErr
	}
	release, err := flow.beginPresentation(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	flow.mu.Lock()
	preparation, source, currentSource := flow.initial, flow.source, flow.currentSource
	flow.mu.Unlock()
	if preparation == nil {
		preparation, err = flow.holder.Prepare(ctx)
		if err != nil {
			return nil, err
		}
		defer func() { result = errors.Join(result, preparation.Close()) }()
	}
	requested, err := flow.challenges(uint8(class), requestedReceivers)
	if err != nil {
		return nil, err
	}
	missing, err := preparation.Missing(requested)
	if err != nil {
		return nil, err
	}
	if len(missing) != 0 {
		if source == nil {
			return nil, errors.New("original Source stock unavailable before admission")
		}
		var issuingSource routeStockIssuer = source
		if currentSource != nil {
			handle, err := currentSource(ctx)
			if err != nil {
				return nil, err
			}
			defer handle.Close()
			issuingSource = handle
		}
		end := minRouteDeadline(flow.plan.Deadline, hello.Deadline, time.Now().Add(admission.ControlClass.Lifetime()).UTC().Truncate(time.Second))
		if err := flow.fundMissing(ctx, preparation, issuingSource, class, missing, end); err != nil {
			return nil, err
		}
	}
	return flow.holder.Take(ctx, presentation, uint8(class))
}

// Concurrent missing-token paths share one Stock preparation owner. Serialize
// preparation through the joined issuance and final Take, retaining each
// original caller's cancellation. The issuer's already funded Control path
// above must bypass this coordination: it is borrowed by the active issuance.
func (flow *routeTokenFlow) beginPresentation(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	flow.mu.Lock()
	if flow.presenting == nil {
		flow.presenting = make(chan struct{}, 1)
	}
	presenting := flow.presenting
	flow.mu.Unlock()
	select {
	case presenting <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		<-presenting
		return nil, err
	}
	return func() { <-presenting }, nil
}

func openFundedSource(ctx context.Context, plan routePrefixPlan, selected *selection.Owner, budget *hosting.Budget,
	authority admissionAuthority, holder *stock.Owner, controls int, resolution bool, retain func(error)) (_ *prefix.Prefix, _ selection.Leg, _ *routeTokenFlow, result error) {
	if retain == nil {
		return nil, selection.Leg{}, nil, errors.New("original Source cleanup owner absent")
	}
	preparation, err := holder.Prepare(ctx)
	if err != nil {
		return nil, selection.Leg{}, nil, err
	}
	defer func() { result = errors.Join(result, preparation.Close()) }()
	config, err := preparePublicationPrefix(ctx, plan, selected, budget, authority, holder)
	if err != nil {
		return nil, selection.Leg{}, nil, err
	}
	flow := &routeTokenFlow{authority: authority, holder: holder, plan: plan, leg: config.Leg, initial: preparation, retain: retain}
	view, err := authority.current()
	if err != nil {
		return nil, selection.Leg{}, nil, errors.Join(err, config.Release())
	}
	issuer, err := config.Leg.IssuerDuty(view)
	if err != nil {
		return nil, selection.Leg{}, nil, errors.Join(err, config.Release())
	}
	flow.issuer = issuer.NodeID
	if controls < 1 || controls > admission.MaximumBatchTokens {
		return nil, selection.Leg{}, nil, errors.Join(errors.New("initial issuer Control stock count invalid"), config.Release())
	}
	controlReceivers := make([][32]byte, controls)
	for index := range controlReceivers {
		controlReceivers[index] = flow.issuer
	}
	if resolution {
		// The signed inventory determines this recipient before bootstrap.
		// Its actual role operation still reselects and verifies it afresh.
		duty, err := config.Leg.ResolutionDuty(view, plan.Exclusions)
		if err != nil {
			return nil, selection.Leg{}, nil, errors.Join(err, config.Release())
		}
		controlReceivers = append(controlReceivers, duty.NodeID)
	}
	if err := flow.bootstrap(ctx, budget, preparation, controlReceivers); err != nil {
		return nil, selection.Leg{}, nil, errors.Join(err, config.Release())
	}
	config.Present = flow.present
	physical, err := prefix.Open(ctx, config)
	if err != nil {
		return nil, selection.Leg{}, nil, errors.Join(err, config.Release())
	}
	flow.mu.Lock()
	flow.source, flow.initial = physical, nil
	flow.mu.Unlock()
	return physical, config.Leg, flow, nil
}
