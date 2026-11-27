package receive

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	nodeadmission "github.com/dianabuilds/ardents-network/internal/domain/node/admission"
)

// routeAdmissionAuthority obtains each Admission observation from the original
// current Network owner; no detached fact is retained as live authority.
type routeAdmissionAuthority struct {
	current func() (network.RuntimeView, error)
}

func (authority routeAdmissionAuthority) observe() (network.RuntimeView, admission.AuthorityFacts, time.Time, error) {
	view, err := authority.current()
	if err != nil {
		return network.RuntimeView{}, admission.AuthorityFacts{}, time.Time{}, err
	}
	facts, now := nodeadmission.AuthorityFactsFromProfile(view.Profile()), view.ObservedAt()
	return view, facts, now, facts.ValidateAt(now)
}

func (authority routeAdmissionAuthority) issuer(signer [32]byte) (admission.AuthorityFacts, time.Time, error) {
	view, facts, now, err := authority.observe()
	if err != nil {
		return admission.AuthorityFacts{}, time.Time{}, err
	}
	if err := issuer.ValidateCurrentAuthority(view, facts, now, signer); err != nil {
		return admission.AuthorityFacts{}, time.Time{}, err
	}
	return facts, now, nil
}

func (authority routeAdmissionAuthority) receiver(receiver receiving.Receiver, end time.Time) (receiving.Observation, error) {
	view, facts, now, err := authority.observe()
	if err != nil {
		return receiving.Observation{}, err
	}
	return receiving.ObserveCurrentAuthority(view, facts, now, receiver, end)
}
