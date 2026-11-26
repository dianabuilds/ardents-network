package funding

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// AdmissionAuthority retains one authority source and its joined cleanup.
type AdmissionAuthority struct {
	Current      func() (network.RuntimeView, error)
	Witness      func() (state.RuntimeWitness, error)
	ObserveFacts func() (admission.AuthorityFacts, time.Time, error)
	Issuer       func([32]byte) (admission.AuthorityFacts, time.Time, error)
	Receiver     func(receiving.Receiver, time.Time) (receiving.Observation, error)
	Close        func() error
	Intent       func(stock.IssuanceIntent) error
	Presentation func(stock.Presentation) error
}

// ValidAdmissionAuthority checks that detached authority inputs and roots do not overlap.
func ValidAdmissionAuthority(profile string, plan *state.AuthorityDeclaration, roots ...string) bool {
	if plan == nil {
		return pathgrammar.Strict(profile)
	}
	if profile != "" || !plan.ValidShape() {
		return false
	}
	for _, root := range roots {
		if !pathgrammar.DisjointRoots(plan.Root, root) {
			return false
		}
	}
	return true
}

// OpenAdmissionAuthority opens the selected original authority source.
func OpenAdmissionAuthority(profile string, plan *state.AuthorityDeclaration) (AdmissionAuthority, error) {
	if !ValidAdmissionAuthority(profile, plan) {
		return AdmissionAuthority{}, errors.New("invalid Admission authority source")
	}
	if plan == nil {
		observe := func() (admission.AuthorityFacts, time.Time, error) {
			var facts admission.AuthorityFacts
			raw, err := artifacts.ReadBoundedFile(profile, 64<<10)
			if err == nil {
				err = artifacts.DecodeObject(raw, &facts)
			}
			now := time.Now().UTC()
			if err == nil {
				err = facts.ValidateAt(now)
			}
			return facts, now, err
		}
		return AdmissionAuthority{ObserveFacts: observe, Issuer: func([32]byte) (admission.AuthorityFacts, time.Time, error) { return observe() }, Close: func() error { return nil },
			Receiver: func(receiver receiving.Receiver, end time.Time) (receiving.Observation, error) {
				facts, now, err := observe()
				return receiving.Observation{Profile: facts, Receiver: receiver, Now: now, NotAfter: end}, err
			}}, nil
	}
	config, err := plan.Config()
	if err != nil {
		return AdmissionAuthority{}, err
	}
	owner, err := state.OpenRetained(config)
	if err != nil {
		return AdmissionAuthority{}, err
	}
	return NetworkAdmissionAuthority(owner.CurrentRuntime, owner.Close, owner.CurrentRuntimeWitness), nil
}

// NetworkAdmissionAuthority binds fresh observations to the existing State owner.
// It never substitutes a diagnostic or cached approval for current authority.
func NetworkAdmissionAuthority(current func() (network.RuntimeView, error), closeOwner func() error, witness func() (state.RuntimeWitness, error)) AdmissionAuthority {
	observe := func() (network.RuntimeView, admission.AuthorityFacts, time.Time, error) {
		view, err := current()
		if err != nil {
			return network.RuntimeView{}, admission.AuthorityFacts{}, time.Time{}, err
		}
		facts, now, err := NetworkFacts(view)
		if err != nil {
			return network.RuntimeView{}, admission.AuthorityFacts{}, time.Time{}, err
		}
		return view, facts, now, nil
	}
	authority := AdmissionAuthority{Close: closeOwner, Current: current, Witness: witness,
		ObserveFacts: func() (admission.AuthorityFacts, time.Time, error) {
			_, facts, now, err := observe()
			return facts, now, err
		},
		Issuer: func(signer [32]byte) (admission.AuthorityFacts, time.Time, error) {
			view, facts, now, err := observe()
			if err != nil {
				return admission.AuthorityFacts{}, time.Time{}, err
			}
			if err := issuer.ValidateCurrentAuthority(view, facts, now, signer); err != nil {
				return admission.AuthorityFacts{}, time.Time{}, err
			}
			return facts, now, nil
		},
		Receiver: func(receiver receiving.Receiver, end time.Time) (receiving.Observation, error) {
			view, facts, now, err := observe()
			if err != nil {
				return receiving.Observation{}, err
			}
			return receiving.ObserveCurrentAuthority(view, facts, now, receiver, end)
		},
	}
	authority.Presentation = PresentationGuard(current)
	authority.Intent = func(intent stock.IssuanceIntent) error {
		view, err := current()
		if err != nil {
			return err
		}
		return ValidateIntent(view, intent)
	}
	return authority
}
