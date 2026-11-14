package descriptor

import (
	"bytes"
	"encoding/json"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

// NetworkInputs declares independent State trust and paths, never a signed
// observation or an accepting duty. State alone authenticates these at runtime.
type NetworkInputs struct {
	Root                 string     `json:"root"`
	NetworkID            [32]byte   `json:"network_id"`
	Authorities          [][32]byte `json:"authorities"`
	Threshold            int        `json:"threshold"`
	ProfileAuthority     [32]byte   `json:"profile_authority"`
	ClockObservationFile string     `json:"clock_observation_file"`
}

// Plan is the closed installed Descriptor input. Its immutable declaration
// grants no State, Node, Admission or Hosting permission and selects no peer.
type Plan struct {
	Schema            string          `json:"schema"`
	Network           NetworkInputs   `json:"network"`
	NodeID            [32]byte        `json:"node_id"`
	ProcessProfile    string          `json:"process_profile"`
	ParticipationRoot string          `json:"participation_root"`
	SpendRoot         string          `json:"spend_root"`
	HostingRoot       string          `json:"hosting_root"`
	DescriptorRoot    string          `json:"descriptor_root"`
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
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || p.Schema != "ardents-descriptor-runtime-v1" || p.ProcessProfile != "ardents-descriptor-process-v1" || p.NodeID == [32]byte{} ||
		n.NetworkID == [32]byte{} || n.ProfileAuthority == [32]byte{} || len(n.Authorities) == 0 || len(n.Authorities) > 16 || n.Threshold <= 0 || n.Threshold > len(n.Authorities) || p.RetirementWaitMS == 0 || p.RetirementWaitMS > 5000 {
		return Plan{}, ErrInput
	}
	seen := map[[32]byte]bool{}
	for _, key := range n.Authorities {
		if key == [32]byte{} || seen[key] {
			return Plan{}, ErrInput
		}
		seen[key] = true
	}
	for _, path := range []string{n.Root, n.ClockObservationFile, p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.DescriptorRoot, p.Certificate, p.PrivateKey} {
		if !validPath(path) {
			return Plan{}, ErrInput
		}
	}
	return p, nil
}

// MatchPlan compares only declarations. It opens no State, key or mutable root;
// full class coverage and actual protection/authority remain runtime decisions.
func (r Request) MatchPlan(p Plan) error {
	if p.Network.Root != r.StateRoot || p.Network.ClockObservationFile != r.Clock || p.ProcessProfile != r.Profile || p.ParticipationRoot != r.ParticipationRoot || p.SpendRoot != r.SpendRoot || p.HostingRoot != r.HostingRoot || p.DescriptorRoot != r.DescriptorRoot ||
		p.Certificate != filepath.Join(r.CredentialDirectory, "certificate.pem") || p.PrivateKey != filepath.Join(r.CredentialDirectory, "private-key.pem") {
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
	if !validPath(output) || !validPath(requestFile) || r.Plan != filepath.Join(output, "descriptor-plan.json") {
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
