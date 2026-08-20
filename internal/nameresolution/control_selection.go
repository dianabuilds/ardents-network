package nameresolution

import (
	"errors"
	"net/http"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/openpcc/ohttp"
)

// OpenControl constructs one single-use control client over the selected
// authenticated Relay/Gateway boundary.
func OpenControl(view state.Snapshot, selection Selection, profile GatewayProfile,
	isolation [32]byte, base *http.Transport,
) (*controlClient, error) {
	plan, err := selectControlPlan(view, selection, profile)
	if err != nil || base == nil || isolation == [32]byte{} ||
		!selection.AdmissionChallenge.BindsIsolation(isolation) {
		return nil, errors.New("private naming control configuration is invalid")
	}
	var key ohttp.KeyConfig
	if err := key.UnmarshalBinary(plan.GatewayKeyConfig); err != nil {
		return nil, errors.New("private naming control Gateway key is invalid")
	}
	client := isolatedHTTPClient(base)
	transport, err := ohttp.NewTransport(key, "https://"+plan.Relay.Endpoint+"/ohttp", ohttp.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &controlClient{plan: plan, client: client, transport: transport}, nil
}

func selectControlPlan(view state.Snapshot, input Selection, profile GatewayProfile) (controlPlan, error) {
	if view.Generation == "" || view.NetworkID == [32]byte{} || view.Digest == [32]byte{} ||
		view.ViewRoot == [32]byte{} || view.Freshness != "fresh" || view.Conflicting || view.CandidateCount < 2 ||
		input.At.IsZero() || !input.At.Before(input.Deadline) || input.Deadline.After(input.At.Add(15*time.Second)) ||
		input.Deadline.After(view.ValidUntil) || input.ConnectionRendezvousNodeID != [32]byte{} ||
		profile.NetworkID != view.NetworkID || profile.NodeID != input.GatewayNodeID ||
		profile.AssignmentNotAfter.Before(input.Deadline) {
		return controlPlan{}, errors.New("private naming control selection is invalid")
	}
	relay, relayOK := findPosition(view, input.RelayNodeID, input.At, input.Deadline)
	gateway, gatewayOK := findPosition(view, input.GatewayNodeID, input.At, input.Deadline)
	if !relayOK || !gatewayOK || !validGatewayProfile(profile, gateway.PublicKey) ||
		relay.Domain != initiatorDomain || gateway.Domain != rendezvousDomain ||
		relay.NodeID == gateway.NodeID || relay.Family == gateway.Family ||
		excludedControl(input, relay) || excludedControl(input, gateway) {
		return controlPlan{}, errors.New("private naming control roles conflict or are unavailable")
	}
	challenge := input.AdmissionChallenge
	if challenge.Node != gateway.NodeID || challenge.Network != view.NetworkID ||
		(challenge.Surface != "renewal-update" && challenge.Surface != "policy-recovery" && challenge.Surface != "root-claim") ||
		challenge.IssuedAt > input.At.UnixMilli() || challenge.ExpiresAt < input.Deadline.UnixMilli() {
		return controlPlan{}, errors.New("private naming control admission is invalid")
	}
	return controlPlan{NetworkID: view.NetworkID, SelectionAt: input.At.UnixNano(), Deadline: input.Deadline.UnixNano(),
		Relay: relay, Gateway: gateway, GatewayKeyConfig: append([]byte(nil), profile.KeyConfig...),
		AdmissionChallenge: challenge}, nil
}

func excludedControl(input Selection, value position) bool {
	for _, identity := range input.ExcludedIdentities {
		if identity == value.NodeID {
			return true
		}
	}
	for _, family := range input.ExcludedFamilies {
		if family == value.Family {
			return true
		}
	}
	return false
}
