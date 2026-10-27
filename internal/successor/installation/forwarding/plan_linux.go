package forwarding

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

// Plan is the closed installed Forwarding input. Its immutable declaration
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
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || p.Schema != "ardents-forwarding-runtime-v1" || p.ProcessProfile != "ardents-forwarding-process-v1" || p.NodeID == [32]byte{} ||
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
	for _, path := range []string{n.Root, n.ClockObservationFile, p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.Certificate, p.PrivateKey} {
		if !validPath(path) {
			return Plan{}, ErrInput
		}
	}
	return p, nil
}

// MatchPlan compares only declarations. It opens no State, key or mutable root;
// full class coverage and actual protection/authority remain runtime decisions.
func (r Request) MatchPlan(p Plan) error {
	if p.Network.Root != r.StateRoot || p.Network.ClockObservationFile != r.Clock || p.ProcessProfile != r.Profile || p.ParticipationRoot != r.ParticipationRoot || p.SpendRoot != r.SpendRoot || p.HostingRoot != r.HostingRoot ||
		p.Certificate != filepath.Join(r.CredentialDirectory, "certificate.pem") || p.PrivateKey != filepath.Join(r.CredentialDirectory, "private-key.pem") {
		return ErrBinding
	}
	return nil
}
