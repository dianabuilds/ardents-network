package main

import (
	"encoding/json"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	introductioninstallation "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction"
)

// The reviewed public pins and fixed request become one immutable local plan.
// This producer opens neither private keys nor State/participation/Hosting roots.
func introductionPlanArguments(args []string, inputs *sourceControlInputs) ([]byte, string, error) {
	if inputs == nil || len(args) > 64 {
		return nil, "", introductioninstallation.ErrInput
	}
	var p deployment.IntroductionPlan
	values, err := nodePlanFlags(args, "introduction", introductioninstallation.ErrInput, &p.Network.Threshold, &p.Work.Tx, &p.Work.Rx, &p.Termination.Tx, &p.Termination.Rx)
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
	r, err := introductioninstallation.DecodeRequest(raw)
	if err != nil || r.Plan != filepath.Join(destination, "introduction-plan.json") {
		return nil, "", introductioninstallation.ErrBinding
	}
	if err := r.CheckPlanPreparationOutput(destination, requestPath); err != nil {
		return nil, "", err
	}
	p.Network.NetworkID, p.NodeID, p.Network.ProfileAuthority, p.Network.Authorities, err = nodePlanKeys(networkID, nodeID, profileAuthority, pins, introductioninstallation.ErrInput)
	if err != nil {
		return nil, "", err
	}

	p.Schema, p.ProcessProfile = "ardents-introduction-runtime-v1", r.Profile
	p.Network.Root, p.Network.ClockObservationFile = r.StateRoot, r.Clock
	p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.IntroductionRoot = r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.IntroductionRoot
	p.Certificate, p.PrivateKey = filepath.Join(r.CredentialDirectory, "certificate.pem"), filepath.Join(r.CredentialDirectory, "private-key.pem")
	p.RetirementWaitMS = uint32(wait)
	raw, err = json.Marshal(p)
	if err != nil {
		return nil, "", err
	}
	raw = append(raw, '\n')
	p, err = deployment.DecodeIntroductionPlan(raw)
	if err != nil {
		return nil, "", introductioninstallation.ErrInput
	}
	if err := r.MatchPlan(p); err != nil {
		return nil, "", err
	}
	return raw, destination, nil
}
