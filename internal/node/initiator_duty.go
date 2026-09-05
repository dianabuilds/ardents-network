package node

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func initiatorDuty(profile InitiatorProfile, snapshot dutyFacts, admit route.EntryBindingAdmitter) (initiatorConfig, error) {
	if snapshot.Profile != route.Profile || snapshot.Assignment != "initiator" || snapshot.ProbeEndpoint == "" || admit == nil ||
		profile.HandshakeLimit == 0 || profile.RelayLimit == 0 || profile.RelayByteLimit == 0 || !validAdmissionTimeout(profile.AdmissionTimeout) || profile.DrainTimeout <= 0 {
		return initiatorConfig{}, errors.New("initiator profile or State assignment is incomplete")
	}
	notAfter := snapshot.ValidUntil
	if snapshot.RecordValidUntil.Before(notAfter) {
		notAfter = snapshot.RecordValidUntil
	}
	var peer initiatorPeer
	var gateway resolutionGateway
	var issuer credentialIssuer
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		candidate := snapshot.Candidates[index]
		if candidate.Assignment != "rendezvous" || candidate.NodeID == [32]byte{} || candidate.PublicKey == [32]byte{} ||
			candidate.NodeID == snapshot.NodeID {
			continue
		}
		if candidate.ValidFrom.After(snapshot.EpochValidFrom) || candidate.ValidUntil.Before(notAfter) || peer.NodeID != [32]byte{} {
			return initiatorConfig{}, errors.New("initiator State Rendezvous peer is incomplete or not valid for the duty")
		}
		peer = initiatorPeer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Endpoint: candidate.Endpoint,
			CarrierProfile: route.CarrierProfile(candidate.CarrierProfile)}
	}
	if peer.NodeID == [32]byte{} {
		return initiatorConfig{}, errors.New("initiator State supplies no Rendezvous peer")
	}
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		candidate := snapshot.Candidates[index]
		if candidate.Assignment != "destination-resolution" || candidate.NodeID == [32]byte{} || candidate.PublicKey == [32]byte{} ||
			candidate.NodeID == snapshot.NodeID {
			continue
		}
		if candidate.ValidFrom.After(snapshot.EpochValidFrom) || candidate.ValidUntil.Before(notAfter) || gateway.NodeID != [32]byte{} {
			return initiatorConfig{}, errors.New("initiator State Destination Resolution Gateway is incomplete or not valid for the duty")
		}
		if !literalNodeEndpoint(candidate.Endpoint) {
			return initiatorConfig{}, errors.New("initiator State Destination Resolution Gateway endpoint is invalid")
		}
		gateway = resolutionGateway{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, URL: "https://" + candidate.Endpoint}
	}
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		candidate := snapshot.Candidates[index]
		if candidate.Assignment != "transit-issuance" || candidate.NodeID == [32]byte{} || candidate.PublicKey == [32]byte{} ||
			candidate.NodeID == snapshot.NodeID {
			continue
		}
		if candidate.ValidFrom.After(snapshot.EpochValidFrom) || candidate.ValidUntil.Before(notAfter) || issuer.NodeID != [32]byte{} {
			return initiatorConfig{}, errors.New("initiator State transit issuer is incomplete or not valid for the duty")
		}
		if !literalNodeEndpoint(candidate.Endpoint) {
			return initiatorConfig{}, errors.New("initiator State transit issuer endpoint is invalid")
		}
		if snapshot.TransitIssuerProfileDigest == [32]byte{} {
			return initiatorConfig{}, errors.New("initiator State transit issuer profile is unavailable")
		}
		issuer = credentialIssuer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey,
			ProfileDigest: snapshot.TransitIssuerProfileDigest, URL: "https://" + candidate.Endpoint}
	}
	return initiatorConfig{ListenAddress: snapshot.ProbeEndpoint, Certificate: profile.Certificate, NetworkID: snapshot.NetworkID,
		EpochDigest: snapshot.Digest, NodeID: snapshot.NodeID, NodePublicKey: snapshot.NodePublicKey, Epoch: snapshot.Epoch,
		NotAfter: notAfter.UTC(), rendezvous: peer, resolutionGateway: gateway, credentialIssuer: issuer, Admit: admit, HandshakeLimit: profile.HandshakeLimit,
		RelayLimit: profile.RelayLimit, RelayByteLimit: profile.RelayByteLimit, AdmissionTimeout: profile.AdmissionTimeout, DrainTimeout: profile.DrainTimeout}, nil
}

func validateNativeDutyProfile(config runtimeConfig, snapshot dutyFacts) error {
	switch snapshot.Assignment {
	case "rendezvous":
		_, err := rendezvousDuty(config.Rendezvous, snapshot)
		return err
	case "initiator":
		// This validation runs before startup and on every live State poll.
		// Opening the durable Entry Admitter here would contend with the one
		// owned by a live Initiator and withdraw a healthy listener. Entry view
		// shape is pure State validation; the actual owner is opened exactly
		// once by startDuty.
		if _, err := entryView(snapshot); err != nil {
			return err
		}
		_, err := initiatorDuty(config.Initiator, snapshot, func([]byte, [32]byte, [32]byte, [32]byte, time.Time) (route.EntryAdmission, error) {
			return route.EntryAdmission{}, nil
		})
		return err
	case "introduction":
		if snapshot.AuthorityCount == 0 {
			return errors.New("introduction State authority verification set is incomplete")
		}
		_, err := introductionDuty(config.Introduction, snapshot, stateTransitGrantAdmitter(config.LocalRoleStateRoot, snapshot,
			func() (dutyFacts, error) { return snapshot, nil }, config.now))
		return err
	case "responder":
		if snapshot.AuthorityCount == 0 {
			return errors.New("responder State authority verification set is incomplete")
		}
		_, err := responderDuty(config.Responder, snapshot, stateTransitGrantAdmitter(config.LocalRoleStateRoot, snapshot,
			func() (dutyFacts, error) { return snapshot, nil }, config.now))
		return err
	case "transit-issuance":
		return validateTransitIssuerProfile(config.TransitIssuer, snapshot, config.now())
	default:
		return errors.New("native Route assignment is not implemented")
	}
}
