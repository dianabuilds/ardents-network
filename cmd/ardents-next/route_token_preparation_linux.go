//go:build linux

package main

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// Both the initial physical Source and an acquired generation implement this
// existing issuance operation. Connection preparation supplies its exact handle;
// it cannot resolve a replacement Source while funding the selected recipients.
type routeStockIssuer interface {
	Issue(context.Context, time.Time, func(prefix.IssuerBinding) (prefix.IssuerBatch, error)) error
}

// prepareInitialOpening funds the retained Publication legs in one batch
// before any of their role channels open. The initial Source is still owned by command construction;
// live Publication replacement must acquire its own retained Source instead.
func (flow *routeTokenFlow) prepareInitialOpening(ctx context.Context, legs ...selection.Leg) (result error) {
	flow.mu.Lock()
	source, transferred := flow.source, flow.currentSource != nil
	flow.mu.Unlock()
	if source == nil || transferred {
		return errors.New("initial Source preparation owner unavailable")
	}
	if err := source.Current(); err != nil {
		return err
	}
	release, err := flow.beginPresentation(ctx)
	if err != nil {
		return err
	}
	defer release()
	preparation, err := flow.holder.Prepare(ctx)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, preparation.Close()) }()
	view, err := flow.authority.current()
	if err != nil {
		return err
	}
	nodes := make([][32]byte, 0, 2*len(legs))
	for _, leg := range legs {
		if err := leg.Check(view, time.Now()); err != nil {
			return err
		}
		nodes = append(nodes, leg.Entry.NodeID, leg.Interior.NodeID)
	}
	challenges, err := flow.challenges(uint8(admission.ForwardClass), nodes)
	if err != nil {
		return err
	}
	missing, err := preparation.Missing(challenges)
	if err != nil {
		return err
	}
	end := minRouteDeadline(flow.plan.Deadline, time.Now().Add(admission.ControlClass.Lifetime()).UTC().Truncate(time.Second))
	if err := flow.fundMissing(ctx, preparation, source, admission.ForwardClass, missing, end); err != nil {
		return err
	}
	view, err = flow.authority.current()
	if err != nil {
		return err
	}
	for _, leg := range legs {
		if err := leg.Check(view, time.Now()); err != nil {
			return err
		}
	}
	return errors.Join(source.Current(), ctx.Err())
}

func (flow *routeTokenFlow) fundMissing(ctx context.Context, preparation *stock.Preparation, source routeStockIssuer, class admission.Class, missing []token.ClosedTokenContext, end time.Time) error {
	if len(missing) == 0 {
		return nil
	}
	receivers := make([][32]byte, 0, len(missing)+1)
	for _, challenge := range missing {
		receivers = append(receivers, challenge.ReceiverNodeID)
	}
	if class == admission.ControlClass {
		// Replace the issuer Control consumed by this same admitted exchange.
		receivers = append(receivers, flow.issuer)
	} else {
		controlsNeeded := [][32]byte{flow.issuer, flow.issuer}
		if class == admission.RegistrationClass {
			// Registration consumes one Control, then an incoming JOIN needs
			// Forward issuance while retaining the next issuing path. Preserve
			// the same two-Control margin as initial Publication preparation;
			// a refresh must not move its missing Control refill into capsule
			// setup. All three rights are charged to the original permission.
			controlsNeeded = append(controlsNeeded, flow.issuer)
		}
		issuerChallenges, err := flow.challenges(uint8(admission.ControlClass), controlsNeeded)
		if err != nil {
			return err
		}
		issuerMissing, err := preparation.Missing(issuerChallenges)
		if err != nil {
			return err
		}
		if len(issuerMissing) != 0 {
			controls := make([][32]byte, len(issuerMissing)+1)
			for index := range controls {
				controls[index] = flow.issuer
			}
			if err := source.Issue(ctx, end, routeIssuerPreparation(ctx, flow.authority, flow.holder, flow.leg,
				uint8(admission.ControlClass), controls, preparation)); err != nil {
				return err
			}
		}
	}
	return source.Issue(ctx, end, routeIssuerPreparation(ctx, flow.authority, flow.holder, flow.leg,
		uint8(class), receivers, preparation))
}

// prepareJoin funds the accepted incoming Rendezvous on one exact existing
// Source. It creates no channel or HELLO, and returns only after issuance joins.
func (flow *routeTokenFlow) prepareJoin(ctx context.Context, source *prefix.SourceHandle, rendezvous network.RetainedDuty) (result error) {
	if source == nil {
		return errors.New("JOIN preparation original Source absent")
	}
	if err := errors.Join(ctx.Err(), source.Check()); err != nil {
		return err
	}
	release, err := flow.beginPresentation(ctx)
	if err != nil {
		return err
	}
	defer release()
	preparation, err := flow.holder.Prepare(ctx)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, preparation.Close()) }()
	view, err := flow.authority.current()
	if err != nil {
		return err
	}
	if err := view.MatchDuty(rendezvous, time.Now()); err != nil {
		return err
	}
	challenges, err := flow.challenges(uint8(admission.ForwardClass), [][32]byte{rendezvous.NodeID})
	if err != nil {
		return err
	}
	missing, err := preparation.Missing(challenges)
	if err != nil {
		return err
	}
	end := minRouteDeadline(flow.plan.Deadline, time.Now().Add(admission.ControlClass.Lifetime()).UTC().Truncate(time.Second))
	if err := flow.fundMissing(ctx, preparation, source, admission.ForwardClass, missing, end); err != nil {
		return err
	}
	view, err = flow.authority.current()
	if err != nil {
		return err
	}
	return errors.Join(view.MatchDuty(rendezvous, time.Now()), ctx.Err(), source.Check())
}

// prepareRoute obtains only missing Stock before Connection starts its capsule
// deadline. No HELLO nonce is generated and no recipient token is presented here.
// Each actual HELLO later performs its own durable Take and authority checks.
func (flow *routeTokenFlow) prepareRoute(ctx context.Context, source *prefix.SourceHandle, introduction, rendezvous network.RetainedDuty) (result error) {
	if source == nil {
		return errors.New("original Connection preparation Source absent")
	}
	if err := source.Check(); err != nil {
		return err
	}
	release, err := flow.beginPresentation(ctx)
	if err != nil {
		return err
	}
	defer release()
	preparation, err := flow.holder.Prepare(ctx)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, preparation.Close()) }()
	view, err := flow.authority.current()
	if err != nil {
		return err
	}
	for _, original := range []network.RetainedDuty{introduction, rendezvous} {
		current, err := view.RetainDuty(original.NodeID, view.ObservedAt())
		if err != nil || current != original {
			return errors.Join(errors.New("original Connection preparation recipient changed"), err)
		}
	}
	end := minRouteDeadline(flow.plan.Deadline, time.Now().Add(admission.ControlClass.Lifetime()).UTC().Truncate(time.Second))
	if err := flow.leg.Check(view, time.Now()); err != nil {
		return err
	}
	// The holder outlives this Reader Job, but its Source does not. Retain one
	// genuine unpresented Source pair in Stock before retiring the only live
	// issuing path. A later caller still needs fresh local authority and its own
	// admitted channels; it cannot repeat the consumed cold bootstrap allowance.
	forwardReceivers := [][32]byte{rendezvous.NodeID, flow.leg.Entry.NodeID, flow.leg.Interior.NodeID}
	forward, err := flow.challenges(uint8(admission.ForwardClass), forwardReceivers)
	if err != nil {
		return err
	}
	missingForward, err := preparation.Missing(forward)
	if err != nil {
		return err
	}
	controls := [][32]byte{introduction.NodeID, flow.issuer}
	if len(missingForward) != 0 {
		controls = append(controls, flow.issuer)
	}
	// Control funds the selected Introduction and the next Forward issuance,
	// retaining one issuer Control afterward. These are the same needed rights,
	// obtained in two class-specific exchanges rather than a third refill.
	for _, request := range []struct {
		class admission.Class
		nodes [][32]byte
	}{
		{admission.ControlClass, controls},
		{admission.ForwardClass, forwardReceivers},
	} {
		challenges, err := flow.challenges(uint8(request.class), request.nodes)
		if err != nil {
			return err
		}
		missing, err := preparation.Missing(challenges)
		if err != nil {
			return err
		}
		if err := flow.fundMissing(ctx, preparation, source, request.class, missing, end); err != nil {
			return err
		}
	}
	view, err = flow.authority.current()
	if err != nil {
		return err
	}
	if err := errors.Join(view.MatchDuty(introduction, time.Now()), view.MatchDuty(rendezvous, time.Now())); err != nil {
		return err
	}
	return errors.Join(flow.leg.Check(view, time.Now()), ctx.Err(), source.Check())
}
