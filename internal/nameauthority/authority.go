package nameauthority

import (
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/namelease"
)

// Plan returns one low-level lease transition and optional policy update.
func Plan(current *namelease.Record, now int64, req Request, cfg Config, state *PolicyState) (namelease.Op, *PolicyState, error) {
	lease := namelease.Policy{
		DefaultLeaseDuration: cfg.DefaultLeaseDuration,
		DefaultGraceDuration: cfg.DefaultGraceDuration,
	}
	var policyState *PolicyState
	if state != nil {
		policyState = ActivatePolicy(copyPolicyState(state), now)
	}

	switch req.Kind {
	case "claim":
		if req.Actor == "" && current != nil {
			req.Actor = current.Authority
		}
		if req.Actor == "" {
			return namelease.Op{}, policyState, errors.New("claim actor is required")
		}
		op := namelease.Op{
			Kind:          "claim",
			Name:          req.Name,
			Generation:    req.Generation,
			Authority:     req.Actor,
			Target:        req.NewTarget,
			ParentName:    req.Parent,
			LeaseDuration: req.LeaseDuration,
			GraceDuration: req.GraceDuration,
		}
		if req.Generation == 0 && current != nil && current.State != "released" {
			return namelease.Op{}, policyState, errors.New("generation is required to overwrite an existing name state")
		}
		if req.Name == "" {
			return namelease.Op{}, policyState, errors.New("claim name is required")
		}
		if req.NewTarget == "" {
			return namelease.Op{}, policyState, errors.New("claim target is required")
		}
		if current != nil && req.Generation != 0 && req.Generation <= current.Generation {
			return namelease.Op{}, policyState, fmt.Errorf("generation %d is not increasing", req.Generation)
		}
		if req.Name == "" {
			return namelease.Op{}, policyState, errors.New("claim name is required")
		}
		_, err := namelease.Apply(current, now, op, lease)
		if err != nil {
			return namelease.Op{}, policyState, err
		}
		return op, policyState, nil

	case "renew":
		if req.Actor == "" || req.Name == "" {
			return namelease.Op{}, policyState, errors.New("renew requires actor and name")
		}
		if current == nil || current.Authority != req.Actor {
			return namelease.Op{}, policyState, errors.New("actor is not current authority")
		}
		op := namelease.Op{
			Kind:          "renew",
			Name:          req.Name,
			Authority:     req.Actor,
			Target:        req.NewTarget,
			Generation:    req.Generation,
			LeaseDuration: req.LeaseDuration,
			GraceDuration: req.GraceDuration,
		}
		_, err := namelease.Apply(current, now, op, lease)
		if err != nil {
			return namelease.Op{}, policyState, err
		}
		return op, policyState, nil

	case "release":
		if req.Actor == "" || req.Name == "" {
			return namelease.Op{}, policyState, errors.New("release requires actor and name")
		}
		if current == nil || current.Authority != req.Actor {
			return namelease.Op{}, policyState, errors.New("actor is not current authority")
		}
		op := namelease.Op{
			Kind:      "release",
			Name:      req.Name,
			Authority: req.Actor,
		}
		_, err := namelease.Apply(current, now, op, lease)
		if err != nil {
			return namelease.Op{}, policyState, err
		}
		return op, policyState, nil

	case "rotate", "transfer":
		if req.Actor == "" || req.Name == "" || req.NewAuthority == "" {
			return namelease.Op{}, policyState, errors.New("rotation requires actor, name, and new authority")
		}
		if current == nil || current.Authority != req.Actor {
			return namelease.Op{}, policyState, errors.New("actor is not current authority")
		}
		op := namelease.Op{
			Kind:          "transfer",
			Name:          req.Name,
			Authority:     req.Actor,
			NewAuthority:  req.NewAuthority,
			Target:        req.NewTarget,
			LeaseDuration: req.LeaseDuration,
			GraceDuration: req.GraceDuration,
			Generation:    req.Generation,
		}
		_, err := namelease.Apply(current, now, op, lease)
		if err != nil {
			return namelease.Op{}, policyState, err
		}
		return op, policyState, nil

	case "set-recovery-policy":
		if req.Actor == "" || req.Name == "" || req.RecoveryPolicy == nil {
			return namelease.Op{}, policyState, errors.New("set-recovery-policy requires actor, name, and policy")
		}
		if current == nil || current.Authority != req.Actor {
			return namelease.Op{}, policyState, errors.New("actor is not current authority")
		}
		if policyState == nil {
			policyState = &PolicyState{}
		}
		policy, err := normalizePolicy(*req.RecoveryPolicy)
		if err != nil {
			return namelease.Op{}, policyState, err
		}
		activeVersion := uint64(0)
		if policyState.Active != nil {
			activeVersion = policyState.Active.Version
		}
		if policyState.Pending != nil && policyState.Pending.Version > activeVersion {
			activeVersion = policyState.Pending.Version
		}
		if policy.Version <= activeVersion {
			return namelease.Op{}, policyState, fmt.Errorf("policy version %d is not increasing", policy.Version)
		}
		if policyState.Pending != nil {
			policyState.Pending = nil
		}
		delay := cfg.DefaultPolicyDelay
		if delay <= 0 {
			delay = time.Minute
		}
		policyState.Pending = &policy
		policyState.PendingActivated = now + int64(delay.Seconds())
		return namelease.Op{}, policyState, nil

	case "start-recovery":
		if req.Actor == "" || req.Name == "" {
			return namelease.Op{}, policyState, errors.New("start-recovery requires actor and name")
		}
		policy, ok := activeRecoveryPolicy(policyState)
		if !ok {
			return namelease.Op{}, policyState, errors.New("no active recovery policy")
		}
		if !CanWitness(policy, req.Actor, req.RecoveryWitnesses) {
			return namelease.Op{}, policyState, errors.New("recovery witnesses are insufficient")
		}
		op := namelease.Op{
			Kind:          "start-recovery",
			Name:          req.Name,
			Authority:     req.Actor,
			RecoveryDelay: policy.Delay,
		}
		if current != nil {
			op.Generation = current.Generation
		}
		_, err := namelease.Apply(current, now, op, lease)
		if err != nil {
			return namelease.Op{}, policyState, err
		}
		return op, policyState, nil

	case "complete-recovery":
		if req.Actor == "" || req.Name == "" || req.NewAuthority == "" {
			return namelease.Op{}, policyState, errors.New("complete-recovery requires actor, name, and new authority")
		}
		policy, ok := activeRecoveryPolicy(policyState)
		if !ok {
			return namelease.Op{}, policyState, errors.New("no active recovery policy")
		}
		if !CanWitness(policy, req.Actor, req.RecoveryWitnesses) {
			return namelease.Op{}, policyState, errors.New("recovery witnesses are insufficient")
		}
		op := namelease.Op{
			Kind:         "install-successor",
			Name:         req.Name,
			NewAuthority: req.NewAuthority,
			Target:       req.NewTarget,
			Generation:   req.Generation,
		}
		if current != nil {
			op.Generation = current.Generation
		}
		_, err := namelease.Apply(current, now, op, lease)
		if err != nil {
			return namelease.Op{}, policyState, err
		}
		return op, policyState, nil

	case "conflict":
		if req.Name == "" || req.ConflictContext == "" {
			return namelease.Op{}, policyState, errors.New("conflict requires name and context")
		}
		op := namelease.Op{
			Kind:            "conflict",
			Name:            req.Name,
			ConflictContext: req.ConflictContext,
		}
		_, err := namelease.Apply(current, now, op, lease)
		if err != nil {
			return namelease.Op{}, policyState, err
		}
		return op, policyState, nil

	case "resolve-conflict":
		if req.Actor == "" || req.Name == "" {
			return namelease.Op{}, policyState, errors.New("resolve-conflict requires actor and name")
		}
		op := namelease.Op{
			Kind:       "resolve-conflict",
			Name:       req.Name,
			Authority:  req.Actor,
			Target:     req.NewTarget,
			Generation: req.Generation,
		}
		_, err := namelease.Apply(current, now, op, lease)
		if err != nil {
			return namelease.Op{}, policyState, err
		}
		return op, policyState, nil

	default:
		return namelease.Op{}, policyState, fmt.Errorf("unsupported request kind %q", req.Kind)
	}
}
