package main

import (
	"encoding/hex"
	"encoding/json"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	joininstallation "github.com/dianabuilds/ardents-network/internal/successor/installation/join"
)

// The reviewed public pins and fixed request become one immutable local plan.
// This producer opens neither private keys nor State/participation/Hosting roots.
func joinPlanArguments(args []string, inputs *sourceControlInputs) ([]byte, string, error) {
	if inputs == nil || len(args) > 64 {
		return nil, "", joininstallation.ErrInput
	}
	var p deployment.JoinPlan
	values, err := nodePlanFlags(args, "join", joininstallation.ErrInput, &p.Network.Threshold, &p.Work.Tx, &p.Work.Rx, &p.Termination.Tx, &p.Termination.Rx)
	if err != nil {
		return nil, "", err
	}
	requestPath, destination := values.requestPath, values.destination
	networkID, nodeID, profileAuthority := values.networkID, values.nodeID, values.profileAuthority
	pins, wait := values.pins.values, values.wait

	raw, err := inputs.read(requestPath, 16<<10)
	if err != nil {
		return nil, "", err
	}
	r, err := joininstallation.DecodeRequest(raw)
	if err != nil || r.Plan != filepath.Join(destination, "join-plan.json") {
		return nil, "", joininstallation.ErrBinding
	}
	if err := r.CheckPlanPreparationOutput(destination, requestPath); err != nil {
		return nil, "", err
	}
	decode := func(value string) ([32]byte, error) {
		var result [32]byte
		raw, err := hex.DecodeString(value)
		if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != value {
			return result, joininstallation.ErrInput
		}
		copy(result[:], raw)
		return result, nil
	}
	p.Network.NetworkID, err = decode(networkID)
	if err != nil {
		return nil, "", err
	}
	p.NodeID, err = decode(nodeID)
	if err != nil {
		return nil, "", err
	}
	p.Network.ProfileAuthority, err = decode(profileAuthority)
	if err != nil {
		return nil, "", err
	}
	for _, pin := range pins {
		key, err := decode(pin)
		if err != nil {
			return nil, "", err
		}
		p.Network.Authorities = append(p.Network.Authorities, key)
	}
	p.Schema, p.ProcessProfile = "ardents-join-runtime-v1", r.Profile
	p.Network.Root, p.Network.ClockObservationFile = r.StateRoot, r.Clock
	p.ParticipationRoot, p.SpendRoot, p.HostingRoot = r.ParticipationRoot, r.SpendRoot, r.HostingRoot
	p.Certificate, p.PrivateKey = filepath.Join(r.CredentialDirectory, "certificate.pem"), filepath.Join(r.CredentialDirectory, "private-key.pem")
	p.RetirementWaitMS = uint32(wait)
	raw, err = json.Marshal(p)
	if err != nil {
		return nil, "", err
	}
	raw = append(raw, '\n')
	p, err = deployment.DecodeJoinPlan(raw)
	if err != nil {
		return nil, "", joininstallation.ErrInput
	}
	if err := r.MatchPlan(p); err != nil {
		return nil, "", err
	}
	return raw, destination, nil
}
