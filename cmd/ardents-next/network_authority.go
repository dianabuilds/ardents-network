package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// networkAuthorityPlan pins the local Network owner, never a claimed current
// profile. Signed Epoch/profile bytes and their durable history supply facts.
// These local Admission commands own no Route or public transport selection.
type networkAuthorityPlan struct {
	Root                 string     `json:"root"`
	NetworkID            [32]byte   `json:"network_id"`
	Authorities          [][32]byte `json:"authorities"`
	Threshold            int        `json:"threshold"`
	ProfileAuthority     [32]byte   `json:"profile_authority"`
	ClockObservationFile string     `json:"clock_observation_file"`
}

type admissionAuthority struct {
	current      func() (network.RuntimeView, error)
	observe      func() (admission.AuthorityFacts, time.Time, error)
	issuer       func([32]byte) (admission.AuthorityFacts, time.Time, error)
	receiver     func(receiving.Receiver, time.Time) (receiving.Observation, error)
	close        func() error
	intent       func(stock.IssuanceIntent) error
	presentation func(stock.Presentation) error
}

func validAdmissionAuthority(profile string, plan *networkAuthorityPlan, roots ...string) bool {
	if plan == nil {
		return absoluteAdmissionPath(profile)
	}
	if profile != "" || !absoluteAdmissionPath(plan.Root) || !absoluteAdmissionPath(plan.ClockObservationFile) ||
		plan.NetworkID == [32]byte{} || plan.ProfileAuthority == [32]byte{} || len(plan.Authorities) == 0 || len(plan.Authorities) > 16 {
		return false
	}
	for _, root := range roots {
		if !absoluteAdmissionPath(root) || root == plan.Root || strings.HasPrefix(root, plan.Root+string(filepath.Separator)) ||
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
	owner, err := state.Open(config)
	if err != nil {
		return admissionAuthority{}, err
	}
	return networkAdmissionAuthority(owner.CurrentRuntime, owner.Close), nil
}

func networkStateConfig(plan *networkAuthorityPlan) (state.Config, error) {
	if plan == nil || !validAdmissionAuthority("", plan) {
		return state.Config{}, errors.New("invalid Network owner plan")
	}
	authorities := make(map[[32]byte]ed25519.PublicKey, len(plan.Authorities))
	for _, key := range plan.Authorities {
		id := sha256.Sum256(key[:])
		if _, duplicate := authorities[id]; duplicate || key == [32]byte{} {
			return state.Config{}, errors.New("invalid Network authority pins")
		}
		authorities[id] = append(ed25519.PublicKey(nil), key[:]...)
	}
	return state.Config{Root: plan.Root, NetworkID: plan.NetworkID, Authorities: authorities,
		Threshold: plan.Threshold, ClosedProfileAuthority: append(ed25519.PublicKey(nil), plan.ProfileAuthority[:]...),
		AcceptedProfile: "ardents-route-v3", Clock: time.Now, ClockObservationFile: plan.ClockObservationFile}, nil
}

// networkAdmissionAuthority is command composition, not Network or Admission
// policy. Each callback gets one new observation; no diagnostic flags, external
// profile file or cached approval can substitute for the opened State owner.
func networkAdmissionAuthority(current func() (network.RuntimeView, error), closeOwner func() error) admissionAuthority {
	observe := func() (network.RuntimeView, admission.AuthorityFacts, time.Time, error) {
		view, err := current()
		if err != nil {
			return network.RuntimeView{}, admission.AuthorityFacts{}, time.Time{}, err
		}
		profile := view.Profile()
		facts := admission.AuthorityFacts{NetworkID: profile.Network, StateGeneration: profile.Generation,
			StateDigest: profile.EpochDigest, Digest: profile.Digest, IssuanceAuthorityKey: profile.IssuanceAuthorityKey,
			IssuerNodeID: profile.IssuerNodeID, IssuerDutyGeneration: profile.IssuerDutyGeneration, Epoch: profile.Epoch,
			NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, TokenKeyCount: uint8(len(profile.TokenKeys))}
		for index, key := range profile.TokenKeys {
			facts.TokenKeys[index] = admission.TokenKey{WindowStart: key.WindowStart, Class: key.Class, SPKI: key.SPKI}
		}
		now := view.ObservedAt()
		if err := facts.ValidateAt(now); err != nil {
			return network.RuntimeView{}, admission.AuthorityFacts{}, time.Time{}, err
		}
		return view, facts, now, nil
	}
	authority := admissionAuthority{close: closeOwner, current: current,
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
	authority.presentation = func(p stock.Presentation) error {
		v, err := authority.receiver(receiving.Receiver{NetworkID: p.NetworkID, StateGeneration: p.StateGeneration, StateDigest: p.StateDigest,
			ProfileDigest: p.ProfileDigest, NodeID: p.RecipientNodeID, DutyGeneration: p.RecipientDutyGeneration}, p.Deadline)
		if err != nil || !v.Now.Before(p.Deadline) || p.Deadline.After(v.NotAfter) {
			return errors.Join(errors.New("network presentation binding is unavailable"), err)
		}
		return nil
	}
	authority.intent = func(intent stock.IssuanceIntent) error {
		view, facts, now, err := observe()
		if err != nil {
			return err
		}
		member, err := view.Member(facts.IssuerNodeID, now)
		if err != nil || member.DutyGeneration != facts.IssuerDutyGeneration || member.RoleDomain != 2 || member.Subrole != 6 ||
			!now.Before(intent.Deadline) || intent.Deadline.After(member.NotAfter()) || intent.Deadline.After(facts.NotAfter) || intent.Selection.ProfileDigest != facts.Digest {
			return errors.Join(errors.New("network issuance selection is unavailable"), err)
		}
		for _, challenge := range intent.Challenges {
			if challenge.NetworkID != facts.NetworkID || challenge.ProfileDigest != facts.Digest || challenge.IssuerNodeID != facts.IssuerNodeID {
				return errors.New("network issuance challenge binding differs")
			}
			receiver, err := view.Member(challenge.ReceiverNodeID, now)
			if err != nil || receiver.DutyGeneration != challenge.ReceiverDutyGeneration || intent.Deadline.After(receiver.NotAfter()) {
				return errors.Join(errors.New("network issuance recipient is unavailable"), err)
			}
		}
		return nil
	}
	return authority
}
