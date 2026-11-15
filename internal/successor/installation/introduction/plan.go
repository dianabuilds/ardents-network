package introduction

import (
	"encoding/json"
	posixpath "path"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
)

// NetworkInputs is a public declaration, not authenticated State or duty.
type NetworkInputs = deployment.NodeNetworkInputs

// Plan is a public Introduction deployment declaration, not runtime authority.
type Plan = deployment.IntroductionPlan

// DecodePlan retains this transition's established refusal identity.
func DecodePlan(raw []byte) (Plan, error) {
	p, err := deployment.DecodeIntroductionPlan(raw)
	if err != nil {
		return Plan{}, ErrInput
	}
	return p, nil
}

// MatchPlan compares only declarations. It opens no State, key or mutable root;
// full class coverage and actual protection/authority remain runtime decisions.
func (r Request) MatchPlan(p Plan) error {
	if p.Network.Root != r.StateRoot || p.Network.ClockObservationFile != r.Clock || p.ProcessProfile != r.Profile || p.ParticipationRoot != r.ParticipationRoot || p.SpendRoot != r.SpendRoot || p.HostingRoot != r.HostingRoot || p.IntroductionRoot != r.IntroductionRoot ||
		p.Certificate != posixpath.Join(r.CredentialDirectory, "certificate.pem") || p.PrivateKey != posixpath.Join(r.CredentialDirectory, "private-key.pem") {
		return ErrBinding
	}
	return nil
}

// CheckPlanPreparationOutput admits only the request's exact future plan leaf
// inside this fresh output. Every other declared input, root and retained
// journal remains outside; the public request's original file is independent.
// This checks path declarations only, not their native provenance or authority.
func (r Request) CheckPlanPreparationOutput(output, requestFile string) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if _, err := DecodeRequest(append(raw, '\n')); err != nil {
		return err
	}
	paths := append(r.paths(), r.reservedSiblingPaths()...)
	if !deployment.PlanOutputIndependent(output, requestFile, r.Plan, "introduction-plan.json", paths) {
		return ErrBinding
	}
	return nil
}
