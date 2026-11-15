package join

import (
	"bytes"
	"encoding/json"
	posixpath "path"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

// NetworkInputs is a public declaration, not authenticated State or duty.
type NetworkInputs = deployment.NodeNetworkInputs

// Plan is the closed installed JOIN input. Its immutable declaration
// grants no State, Node, Admission or Hosting permission and selects no peer.
type Plan struct {
	Schema            string          `json:"schema"`
	Network           NetworkInputs   `json:"network"`
	NodeID            [32]byte        `json:"node_id"`
	ProcessProfile    string          `json:"process_profile"`
	ParticipationRoot string          `json:"participation_root"`
	SpendRoot         string          `json:"spend_root"`
	HostingRoot       string          `json:"hosting_root"`
	Certificate       string          `json:"certificate"`
	PrivateKey        string          `json:"private_key"`
	Work              hosting.Traffic `json:"work"`
	Termination       hosting.Traffic `json:"termination"`
	RetirementWaitMS  uint32          `json:"retirement_wait_ms"`
}

func DecodePlan(raw []byte) (Plan, error) {
	var p Plan
	if len(raw) == 0 || len(raw) > 16<<10 || json.Unmarshal(raw, &p) != nil {
		return Plan{}, ErrInput
	}
	canonical, err := json.Marshal(p)
	n := p.Network
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || p.Schema != "ardents-join-runtime-v1" || p.ProcessProfile != "ardents-join-process-v1" || p.NodeID == [32]byte{} ||
		!deployment.ValidNodeTrustDeclaration(n.NetworkID, n.ProfileAuthority, n.Authorities, n.Threshold) || p.RetirementWaitMS == 0 || p.RetirementWaitMS > 5000 {
		return Plan{}, ErrInput
	}
	if !deployment.IndependentNodePaths(n.Root, n.ClockObservationFile, p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.Certificate, p.PrivateKey) {
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
	if !validPath(output) || !validPath(requestFile) || r.Plan != posixpath.Join(output, "join-plan.json") {
		return ErrBinding
	}
	paths := append(r.paths(), r.reservedSiblingPaths()...)
	for _, path := range paths {
		if contains(requestFile, path) || contains(path, requestFile) {
			return ErrBinding
		}
	}
	paths = append(paths, requestFile)
	for _, path := range paths {
		if path != r.Plan && (contains(output, path) || contains(path, output)) {
			return ErrBinding
		}
	}
	return nil
}
