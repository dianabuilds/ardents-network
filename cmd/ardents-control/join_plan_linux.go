package main

import (
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
	p.Network.NetworkID, p.NodeID, p.Network.ProfileAuthority, p.Network.Authorities, err = nodePlanKeys(networkID, nodeID, profileAuthority, pins, joininstallation.ErrInput)
	if err != nil {
		return nil, "", err
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
