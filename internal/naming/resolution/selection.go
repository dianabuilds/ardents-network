package resolution

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func selectPlan(view state.ResolutionView, input Selection, profile GatewayProfile) (plan, error) {
	epoch, available := view.Epoch(input.At, input.Deadline)
	if !available || input.ConnectionRendezvousNodeID == [32]byte{} || profile.NetworkID != epoch.NetworkID || profile.NodeID != input.GatewayNodeID ||
		profile.AssignmentNotAfter.Before(input.Deadline) {
		return plan{}, errors.New("private resolution selection is invalid")
	}
	relay, relayOK := findPosition(view, input.RelayNodeID, input.At, input.Deadline)
	gateway, gatewayOK := findPosition(view, input.GatewayNodeID, input.At, input.Deadline)
	rendezvous, rendezvousOK := findPosition(view, input.ConnectionRendezvousNodeID, input.At, input.Deadline)
	if !relayOK || !gatewayOK || !rendezvousOK || !validGatewayProfile(profile, gateway.PublicKey) ||
		relay.Domain != initiatorDomain || gateway.Domain != rendezvousDomain ||
		rendezvous.Domain != rendezvousDomain || !gateway.AssignmentNotAfter.Equal(profile.AssignmentNotAfter) ||
		collides(relay, gateway, rendezvous) || excluded(input, relay) || excluded(input, gateway) || excluded(input, rendezvous) {
		return plan{}, errors.New("private resolution roles conflict or are unavailable")
	}
	verifier, err := namespace.OpenResolutionVerifier(namespacePolicy(epoch), epoch.Number, epoch.Digest)
	if err != nil {
		return plan{}, errors.New("private resolution Namespace trust root is invalid")
	}
	result := plan{NetworkID: epoch.NetworkID, Generation: epoch.Generation, Epoch: epoch.Number, EpochDigest: epoch.Digest,
		ViewRoot: epoch.ViewRoot, SelectionAt: input.At.UnixNano(), Deadline: input.Deadline.UnixNano(),
		Relay: relay, Gateway: gateway, ConnectionRendezvous: rendezvous,
		GatewayKeyConfig: append([]byte(nil), profile.KeyConfig...), GatewayKeyConfigDigest: profile.KeyConfigDigest,
		ExcludedIdentities: appendDistinctIdentities(input.ExcludedIdentities, relay.NodeID, gateway.NodeID),
		ExcludedFamilies:   appendDistinctFamilies(input.ExcludedFamilies, relay.Family, gateway.Family),
		AdmissionChallenge: input.AdmissionChallenge, NamespaceVerifier: verifier}
	if err := validatePlan(result); err != nil {
		return plan{}, err
	}
	return result, nil
}

func namespacePolicy(epoch state.ResolutionEpoch) namespace.MaterializationPolicy {
	policy := namespace.MaterializationPolicy{Network: epoch.NetworkID, Rule: "ardents-namespace-materialization-v1",
		Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: int(epoch.Threshold)}
	for _, authority := range epoch.Authorities {
		key := append(ed25519.PublicKey(nil), authority.PublicKey[:]...)
		policy.Authorities[authority.ID] = key
	}
	return policy
}

func appendDistinctIdentities(existing [][32]byte, values ...[32]byte) [][32]byte {
	result := append([][32]byte(nil), existing...)
	for _, value := range values {
		found := false
		for _, current := range result {
			found = found || current == value
		}
		if !found {
			result = append(result, value)
		}
	}
	return result
}

func appendDistinctFamilies(existing []string, values ...string) []string {
	result := append([]string(nil), existing...)
	for _, value := range values {
		found := false
		for _, current := range result {
			found = found || current == value
		}
		if !found {
			result = append(result, value)
		}
	}
	return result
}

func validatePlan(plan plan) error {
	selectionAt, deadline := time.Unix(0, plan.SelectionAt), time.Unix(0, plan.Deadline)
	if plan.NetworkID == [32]byte{} || plan.Generation == "" || plan.Epoch == 0 || plan.EpochDigest == [32]byte{} ||
		plan.ViewRoot == [32]byte{} || plan.SelectionAt <= 0 || !selectionAt.Before(deadline) ||
		len(plan.GatewayKeyConfig) == 0 || sha256.Sum256(plan.GatewayKeyConfig) != plan.GatewayKeyConfigDigest ||
		plan.Relay.Domain != initiatorDomain || plan.Gateway.Domain != rendezvousDomain ||
		plan.ConnectionRendezvous.Domain != rendezvousDomain || collides(plan.Relay, plan.Gateway, plan.ConnectionRendezvous) ||
		plan.NamespaceVerifier == nil {
		return errors.New("private resolution Plan is invalid")
	}
	challenge := plan.AdmissionChallenge
	if challenge.Node != plan.Gateway.NodeID || challenge.Network != plan.NetworkID || challenge.Surface != "resolution" ||
		challenge.WorkBits != 16 || challenge.IssuedAt > selectionAt.UnixMilli() || challenge.ExpiresAt < deadline.UnixMilli() {
		return errors.New("private resolution admission Challenge is invalid")
	}
	for _, position := range []position{plan.Relay, plan.Gateway, plan.ConnectionRendezvous} {
		if position.NodeID == [32]byte{} || position.Family == "" || !literalEndpoint(position.Endpoint) ||
			position.AssignmentNotAfter.Before(deadline) {
			return errors.New("private resolution Plan contains an invalid role")
		}
	}
	return nil
}

func findPosition(view state.ResolutionView, nodeID [32]byte, at, deadline time.Time) (position, bool) {
	candidate, valid := view.Candidate(nodeID, at, deadline)
	if !valid || !literalEndpoint(candidate.Endpoint) {
		return position{}, false
	}
	return position{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Family: candidate.Family, Endpoint: candidate.Endpoint,
		Domain: candidate.Domain, AssignmentNotAfter: candidate.AssignmentNotAfter}, true
}

func collides(values ...position) bool {
	identities, families := map[[32]byte]bool{}, map[string]bool{}
	for _, value := range values {
		if identities[value.NodeID] || families[value.Family] {
			return true
		}
		identities[value.NodeID], families[value.Family] = true, true
	}
	return false
}

func excluded(input Selection, position position) bool {
	for _, identity := range input.ExcludedIdentities {
		if identity == position.NodeID {
			return true
		}
	}
	for _, family := range input.ExcludedFamilies {
		if family == position.Family {
			return true
		}
	}
	return false
}

func literalEndpoint(endpoint string) bool {
	host, port, err := net.SplitHostPort(endpoint)
	number, portErr := strconv.Atoi(port)
	return err == nil && net.ParseIP(host) != nil && portErr == nil && number > 0 && number <= 65535
}
