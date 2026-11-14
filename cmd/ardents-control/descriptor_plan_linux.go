package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"path/filepath"
	"strings"

	descriptorinstallation "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor"
)

type descriptorAuthorityFlags []string

func (pins *descriptorAuthorityFlags) String() string { return strings.Join(*pins, ",") }
func (pins *descriptorAuthorityFlags) Set(value string) error {
	if len(*pins) >= 16 || len(value) != 64 {
		return descriptorinstallation.ErrInput
	}
	*pins = append(*pins, value)
	return nil
}

// The reviewed public pins and fixed request become one immutable local plan.
// This producer opens neither private keys nor State/participation/Hosting roots.
func descriptorPlanArguments(args []string, inputs *sourceControlInputs) ([]byte, string, error) {
	if inputs == nil || len(args) > 64 {
		return nil, "", descriptorinstallation.ErrInput
	}
	flags := flag.NewFlagSet("descriptor prepare-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var requestPath, destination, networkID, nodeID, profileAuthority string
	var pins descriptorAuthorityFlags
	var p descriptorinstallation.Plan
	var wait uint64
	flags.StringVar(&requestPath, "request", "", "original canonical request")
	flags.StringVar(&destination, "output", "", "fresh output directory")
	flags.StringVar(&networkID, "network-id", "", "exact Network ID")
	flags.StringVar(&nodeID, "node-id", "", "exact Node ID")
	flags.StringVar(&profileAuthority, "profile-authority", "", "independently approved profile key")
	flags.Var(&pins, "authority", "independently approved State key; at most sixteen")
	flags.IntVar(&p.Network.Threshold, "threshold", 0, "State signature threshold")
	flags.Uint64Var(&p.Work.Tx, "work-tx", 0, "full Descriptor work Tx coverage")
	flags.Uint64Var(&p.Work.Rx, "work-rx", 0, "full Descriptor work Rx coverage")
	flags.Uint64Var(&p.Termination.Tx, "termination-tx", 0, "termination Tx coverage")
	flags.Uint64Var(&p.Termination.Rx, "termination-rx", 0, "termination Rx coverage")
	flags.Uint64Var(&wait, "retirement-wait-ms", 0, "bounded notice wait, not a descriptor proof")
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 12 || !forwardingUniqueOptions(args, "authority") || !sourceControlPath(destination) || wait == 0 || wait > 5000 {
		return nil, "", descriptorinstallation.ErrInput
	}
	raw, err := inputs.read(requestPath, 16<<10)
	if err != nil {
		return nil, "", err
	}
	r, err := descriptorinstallation.DecodeRequest(raw)
	if err != nil || r.Plan != filepath.Join(destination, "descriptor-plan.json") {
		return nil, "", descriptorinstallation.ErrBinding
	}
	if err := r.CheckPlanPreparationOutput(destination, requestPath); err != nil {
		return nil, "", err
	}
	decode := func(value string) ([32]byte, error) {
		var result [32]byte
		raw, err := hex.DecodeString(value)
		if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != value {
			return result, descriptorinstallation.ErrInput
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
	p.Schema, p.ProcessProfile = "ardents-descriptor-runtime-v1", r.Profile
	p.Network.Root, p.Network.ClockObservationFile = r.StateRoot, r.Clock
	p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.DescriptorRoot = r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.DescriptorRoot
	p.Certificate, p.PrivateKey = filepath.Join(r.CredentialDirectory, "certificate.pem"), filepath.Join(r.CredentialDirectory, "private-key.pem")
	p.RetirementWaitMS = uint32(wait)
	raw, err = json.Marshal(p)
	if err != nil {
		return nil, "", err
	}
	raw = append(raw, '\n')
	p, err = descriptorinstallation.DecodePlan(raw)
	if err != nil {
		return nil, "", err
	}
	if err := r.MatchPlan(p); err != nil {
		return nil, "", err
	}
	return raw, destination, nil
}
