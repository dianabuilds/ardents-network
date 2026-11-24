package main

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

// networkAuthorityPlan pins the local Network owner, never a claimed current
// profile. Signed Epoch/profile bytes and their durable history supply facts.
// These local Admission commands own no Route or public transport selection.
type networkAuthorityPlan = state.AuthorityDeclaration

type admissionAuthority struct {
	current      func() (network.RuntimeView, error)
	witness      func() (state.RuntimeWitness, error)
	observe      func() (admission.AuthorityFacts, time.Time, error)
	issuer       func([32]byte) (admission.AuthorityFacts, time.Time, error)
	receiver     func(receiving.Receiver, time.Time) (receiving.Observation, error)
	close        func() error
	intent       func(stock.IssuanceIntent) error
	presentation func(stock.Presentation) error
}

func validAdmissionAuthority(profile string, plan *networkAuthorityPlan, roots ...string) bool {
	if plan == nil {
		return pathgrammar.Strict(profile)
	}
	if profile != "" || !plan.ValidShape() {
		return false
	}
	for _, root := range roots {
		if !pathgrammar.Strict(root) || root == plan.Root || strings.HasPrefix(root, plan.Root+string(filepath.Separator)) ||
			strings.HasPrefix(plan.Root, root+string(filepath.Separator)) {
			return false
		}
	}
	return true
}

func openAdmissionAuthority(profile string, plan *networkAuthorityPlan) (admissionAuthority, error) {
	if !validAdmissionAuthority(profile, plan) {
		return admissionAuthority{}, errors.New("invalid Admission authority source")
	}
	if plan == nil {
		observe := admissionObserver(profile)
		return admissionAuthority{observe: observe, issuer: func([32]byte) (admission.AuthorityFacts, time.Time, error) { return observe() }, close: func() error { return nil },
			receiver: func(receiver receiving.Receiver, end time.Time) (receiving.Observation, error) {
				facts, now, err := observe()
				return receiving.Observation{Profile: facts, Receiver: receiver, Now: now, NotAfter: end}, err
			}}, nil
	}
	config, err := networkStateConfig(plan)
	if err != nil {
		return admissionAuthority{}, err
	}
	owner, err := state.OpenRetained(config)
	if err != nil {
		return admissionAuthority{}, err
	}
	return networkAdmissionAuthority(owner.CurrentRuntime, owner.Close, owner.CurrentRuntimeWitness), nil
}

func networkStateConfig(plan *networkAuthorityPlan) (state.Config, error) {
	return plan.Config()
}

// networkAdmissionAuthority is command composition, not Network or Admission
// policy. Each callback gets one new observation; no diagnostic flags, external
// profile file or cached approval can substitute for the opened State owner.
func networkAdmissionAuthority(current func() (network.RuntimeView, error), closeOwner func() error, witness func() (state.RuntimeWitness, error)) admissionAuthority {
	observe := func() (network.RuntimeView, admission.AuthorityFacts, time.Time, error) {
		view, err := current()
		if err != nil {
			return network.RuntimeView{}, admission.AuthorityFacts{}, time.Time{}, err
		}
		facts, now, err := funding.NetworkFacts(view)
		if err != nil {
			return network.RuntimeView{}, admission.AuthorityFacts{}, time.Time{}, err
		}
		return view, facts, now, nil
	}
	authority := admissionAuthority{close: closeOwner, current: current, witness: witness,
		observe: func() (admission.AuthorityFacts, time.Time, error) {
			_, facts, now, err := observe()
			return facts, now, err
		},
		issuer: func(signer [32]byte) (admission.AuthorityFacts, time.Time, error) {
			view, facts, now, err := observe()
			if err != nil {
				return admission.AuthorityFacts{}, time.Time{}, err
			}
			member, err := view.Member(facts.IssuerNodeID, now)
			if err != nil || signer == [32]byte{} || member.PublicKey != signer || member.DutyGeneration != facts.IssuerDutyGeneration || member.RoleDomain != 2 || member.Subrole != 6 {
				return admission.AuthorityFacts{}, time.Time{}, errors.Join(errors.New("network issuer duty is unavailable"), err)
			}
			return facts, now, nil
		},
		receiver: func(receiver receiving.Receiver, end time.Time) (receiving.Observation, error) {
			view, facts, now, err := observe()
			if err != nil {
				return receiving.Observation{}, err
			}
			member, err := view.Member(receiver.NodeID, now)
			if err != nil || member.DutyGeneration != receiver.DutyGeneration || facts.NetworkID != receiver.NetworkID ||
				facts.StateGeneration != receiver.StateGeneration || facts.StateDigest != receiver.StateDigest || facts.Digest != receiver.ProfileDigest {
				return receiving.Observation{}, errors.Join(errors.New("network receiver duty is unavailable"), err)
			}
			if member.NotAfter().Before(end) {
				end = member.NotAfter()
			}
			if facts.NotAfter.Before(end) {
				end = facts.NotAfter
			}
			return receiving.Observation{Profile: facts, Receiver: receiver, Now: now, NotAfter: end}, nil
		},
	}
	authority.presentation = funding.PresentationGuard(current)
	authority.intent = func(intent stock.IssuanceIntent) error {
		view, err := current()
		if err != nil {
			return err
		}
		return funding.ValidateIntent(view, intent)
	}
	return authority
}
