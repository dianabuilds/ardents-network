package forwarding

import (
	posixpath "path"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
)

// NetworkInputs is a public declaration, not authenticated State or duty.
type NetworkInputs = deployment.NodeNetworkInputs

// Plan is a public Forwarding deployment declaration, not runtime authority.
type Plan = deployment.ForwardingPlan

// DecodePlan retains this transition's established refusal identity.
func DecodePlan(raw []byte) (Plan, error) {
	p, err := deployment.DecodeForwardingPlan(raw)
	if err != nil {
		return Plan{}, ErrInput
	}
	return p, nil
}

// MatchPlan compares only declarations. It opens no State, key or mutable root;
// full class coverage and actual protection/authority remain runtime decisions.
func (r Request) MatchPlan(p Plan) error {
	if p.Network.Root != r.StateRoot || p.Network.ClockObservationFile != r.Clock || p.ProcessProfile != r.Profile || p.ParticipationRoot != r.ParticipationRoot || p.SpendRoot != r.SpendRoot || p.HostingRoot != r.HostingRoot ||
		p.Certificate != posixpath.Join(r.CredentialDirectory, "certificate.pem") || p.PrivateKey != posixpath.Join(r.CredentialDirectory, "private-key.pem") {
		return ErrBinding
	}
	return nil
}
