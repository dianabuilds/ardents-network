package forwarding

import (
	"encoding/json"
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

// CheckPlanPreparationOutput keeps the original request and the plan output
// outside every other declared input and reserved lifecycle namespace.
func (r Request) CheckPlanPreparationOutput(output, requestFile string) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if _, err := DecodeRequest(append(raw, '\n')); err != nil {
		return err
	}
	if !deployment.PlanOutputIndependent(output, requestFile, r.Plan, "forwarding-plan.json", append(r.paths(), r.reservedSiblingPaths()...)) {
		return ErrBinding
	}
	return nil
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
