package main

import (
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
)

// The reviewed public pins and fixed request become one immutable local plan.
// This producer opens neither private keys nor State/participation/Hosting roots.
func forwardingPlanArguments(args []string, inputs *sourceControlInputs) ([]byte, string, error) {
	if inputs == nil || len(args) > 64 {
		return nil, "", forwarding.ErrInput
	}
	var p deployment.ForwardingPlan
	values, err := nodePlanFlags(args, "forwarding", forwarding.ErrInput, &p.Network.Threshold, &p.Work.Tx, &p.Work.Rx, &p.Termination.Tx, &p.Termination.Rx)
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
	r, err := forwarding.DecodeRequest(raw)
	if err != nil || r.Plan != filepath.Join(destination, "forwarding-plan.json") {
		return nil, "", forwarding.ErrBinding
	}
	if err := r.CheckPlanPreparationOutput(destination, requestPath); err != nil {
		return nil, "", err
	}

	decode := func(value string) ([32]byte, error) {
		var result [32]byte
		raw, err := hex.DecodeString(value)
		if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != value {
			return result, forwarding.ErrInput
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
	p.Schema, p.ProcessProfile = "ardents-forwarding-runtime-v1", r.Profile
	p.Network.Root, p.Network.ClockObservationFile = r.StateRoot, r.Clock
	p.ParticipationRoot, p.SpendRoot, p.HostingRoot = r.ParticipationRoot, r.SpendRoot, r.HostingRoot
	p.Certificate, p.PrivateKey = filepath.Join(r.CredentialDirectory, "certificate.pem"), filepath.Join(r.CredentialDirectory, "private-key.pem")
	p.RetirementWaitMS = uint32(wait)
	raw, err = json.Marshal(p)
	if err != nil {
		return nil, "", err
	}
	raw = append(raw, '\n')
	p, err = deployment.DecodeForwardingPlan(raw)
	if err != nil {
		return nil, "", forwarding.ErrInput
	}
	if err := r.MatchPlan(p); err != nil {
		return nil, "", err
	}
	return raw, destination, nil
}

// Both finite Forwarding flag producers reject implicit last-wins selection.
// Only the plan's exact State-authority option is intentionally repeatable.
func forwardingUniqueOptions(args []string, repeatable string) bool {
	seen := map[string]bool{}
	for _, argument := range args {
		if len(argument) < 2 || argument[0] != '-' {
			continue
		}
		name := strings.TrimLeft(argument, "-")
		name, _, _ = strings.Cut(name, "=")
		if seen[name] && name != repeatable {
			return false
		}
		seen[name] = true
	}
	return true
}
