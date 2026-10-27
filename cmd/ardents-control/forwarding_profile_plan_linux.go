package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

const maximumForwardingProfilePlan = 2 << 20

type forwardingProfileDuty struct {
	Node    string `json:"node_id"`
	Domain  uint8  `json:"role_domain"`
	Subrole uint8  `json:"subrole"`
}

// Only public facts are frozen here. Role refinement is explicit; Epoch
// assignment, membership, grammar and acceptance remain Network decisions.
type forwardingProfilePlan struct {
	Schema              string                   `json:"schema"`
	Reference           time.Time                `json:"reference_time"`
	Network             string                   `json:"network_id"`
	Authority           string                   `json:"state_authority_key"`
	PermissionAuthority string                   `json:"issuance_authority_key"`
	Bundle              []byte                   `json:"epoch_bundle"`
	Tokens              forwardingTokenInventory `json:"tokens"`
	Duties              []forwardingProfileDuty  `json:"duties"`
}

func (p forwardingProfilePlan) current(now time.Time) error {
	if p.Schema != "ardents-forwarding-profile-plan-v1" || p.Reference.IsZero() || p.Reference.Location() != time.UTC || p.Reference.Nanosecond() != 0 || p.Reference.Before(now.Add(-5*time.Minute)) || p.Reference.After(now.Add(time.Minute)) || now.Before(p.Tokens.Inventory.Binding.Start) || !now.Before(p.Tokens.Inventory.Binding.End) || p.Reference.Before(p.Tokens.Inventory.Binding.Start) || !p.Reference.Before(p.Tokens.Inventory.Binding.End) {
		return errSourceControlInput
	}
	return nil
}

type forwardingProfilePreparation struct {
	input    closedprofile.Input
	decision epoch.Decision
	context  closedprofile.Context
	body     []byte
}

func (p forwardingProfilePlan) prepare(now time.Time) (prepared forwardingProfilePreparation, returned error) {
	if err := p.current(now); err != nil {
		return prepared, err
	}
	network, err := forwardingPublic32(p.Network)
	if err != nil {
		return prepared, err
	}
	authority, err := forwardingPublic32(p.Authority)
	if err != nil {
		return prepared, err
	}
	permission, err := forwardingPublic32(p.PermissionAuthority)
	if err != nil || authority == permission || p.Tokens.Schema != "ardents-forwarding-token-inventory-v1" || p.Tokens.Inventory.Binding.Network != network || len(p.Bundle) == 0 || len(p.Bundle) > 1<<20 || len(p.Duties) == 0 || len(p.Duties) > closedprofile.MaxNodes {
		return prepared, errSourceControlInput
	}
	bundle, err := source.DecodeBundle(p.Bundle)
	if err != nil || len(bundle.Materials) != 1 {
		return prepared, errors.Join(errSourceControlInput, err)
	}
	index, err := epoch.InspectMaterializationIndex(bundle.Materials[0])
	if err != nil {
		return prepared, err
	}
	decision, err := epoch.Verify(epoch.Policy{NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authority[:]): ed25519.PublicKey(authority[:])}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: now, MaterializationIndex: index}, bundle.Epoch, bundle.Inputs, bundle.Materials, true)
	if err != nil || decision.Header.Number != 1 || !decision.Snapshot.RecordPresent {
		return prepared, errors.Join(errSourceControlInput, err)
	}
	generation, err := forwardingPublic32(decision.Snapshot.Generation)
	if err != nil {
		return prepared, err
	}
	binding := p.Tokens.Inventory.Binding
	if err := issuerprofile.ValidateCohorts(binding.Start, binding.End, p.Tokens.Inventory.Keys); err != nil {
		return prepared, err
	}
	input := closedprofile.Input{NetworkID: network, StateGeneration: generation, EpochDigest: decision.Header.Digest, Epoch: decision.Header.Number, IssuerNodeID: binding.Issuer, IssuanceAuthorityKey: permission, NotBefore: binding.Start, NotAfter: binding.End}
	issuerMatches := false
	for _, candidate := range decision.Candidates {
		if candidate.PublicKey == permission {
			return prepared, errSourceControlInput
		}
		if candidate.NodeID == binding.Issuer {
			issuerMatches = candidate.PublicKey == binding.Signer
		}
	}
	if !issuerMatches {
		return prepared, errSourceControlInput
	}
	for _, duty := range p.Duties {
		node, err := forwardingPublic32(duty.Node)
		if err != nil {
			return prepared, err
		}
		found := false
		for _, candidate := range decision.Candidates {
			if candidate.NodeID == node {
				input.Nodes = append(input.Nodes, closedprofile.NodeInput{NodeID: node, RecordDigest: candidate.RecordDigest, RoleDomain: duty.Domain, Subrole: duty.Subrole, DutyGeneration: candidate.RecordGeneration})
				found = true
				break
			}
		}
		if !found {
			return prepared, errSourceControlInput
		}
	}
	for _, key := range p.Tokens.Inventory.Keys {
		input.TokenKeys = append(input.TokenKeys, closedprofile.TokenKeyInput{WindowStart: time.Unix(int64(key.Window), 0).UTC(), Class: key.Class, SPKI: bytes.Clone(key.SPKI)})
	}
	body, err := closedprofile.Prepare(input)
	if err != nil {
		return prepared, err
	}
	context := closedprofile.Context{NetworkID: network, StateGeneration: generation, EpochDigest: decision.Header.Digest, Epoch: decision.Header.Number, Authority: ed25519.PublicKey(authority[:]), Now: now}
	if _, err := closedprofile.SigningTranscript(body, context); err != nil {
		return prepared, err
	}
	prepared = forwardingProfilePreparation{input, decision, context, body}
	// The unsigned-body digest names only this detached consistency check;
	// it is not a signed profile identity or accepted State.
	return prepared, prepared.bind(sha256.Sum256(body))
}

// Adapt detached verified facts to the actual Network join. No role mapping,
// membership policy, trusted clock, durable root or runtime right lives here.
func (p forwardingProfilePreparation) bind(digest [32]byte) error {
	i := p.input
	binding := networkdomain.ProfileBinding{Network: i.NetworkID, Generation: i.StateGeneration, EpochDigest: i.EpochDigest, Digest: digest, Epoch: i.Epoch, NotBefore: i.NotBefore, NotAfter: i.NotAfter}
	facts := networkdomain.ProfileFacts{ProfileBinding: binding, IssuanceAuthorityKey: i.IssuanceAuthorityKey, IssuerNodeID: i.IssuerNodeID}
	var assignments []networkdomain.Assignment
	for _, node := range i.Nodes {
		assignments = append(assignments, networkdomain.Assignment{NodeID: node.NodeID, RecordDigest: node.RecordDigest, Generation: node.DutyGeneration, RoleDomain: node.RoleDomain, Subrole: node.Subrole})
		if node.NodeID == i.IssuerNodeID {
			facts.IssuerDutyGeneration = node.DutyGeneration
		}
	}
	var records []networkdomain.NodeRecord
	for _, record := range p.decision.Candidates {
		records = append(records, networkdomain.NodeRecord{NodeID: record.NodeID, RecordDigest: record.RecordDigest, Generation: record.RecordGeneration, PublicKey: record.PublicKey, FamilyID: record.FamilyID, KeyID: record.KeyID, DomainProofDigest: sha256.Sum256(record.DomainProof), Endpoint: record.Endpoint, CarrierProfile: record.CarrierProfile, Capacity: record.Capacity, Assignment: record.Domain, ValidFrom: record.ValidFrom, ValidUntil: record.ValidUntil, AssignmentNotAfter: record.AssignmentNotAfter})
	}
	for _, key := range i.TokenKeys {
		public := networkdomain.TokenKey{WindowStart: key.WindowStart, Class: key.Class}
		copy(public.SPKI[:], key.SPKI)
		facts.TokenKeys = append(facts.TokenKeys, public)
	}
	membership, err := networkdomain.BindMembership(binding, assignments, records)
	if err != nil {
		return err
	}
	header := p.decision.Header
	_, err = networkdomain.BindAcceptedState(networkdomain.EpochFacts{Network: header.NetworkID, Number: header.Number, Digest: header.Digest, Previous: header.Previous, ValidFrom: header.ValidFrom, ValidUntil: header.ValidUntil}, i.StateGeneration, facts, membership)
	return err
}

func decodeForwardingProfilePlan(raw []byte, now time.Time) (p forwardingProfilePlan, prepared forwardingProfilePreparation, returned error) {
	if len(raw) == 0 || len(raw) > maximumForwardingProfilePlan || json.Unmarshal(raw, &p) != nil {
		return p, prepared, errSourceControlInput
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return p, prepared, errSourceControlInput
	}
	prepared, err = p.prepare(now)
	return p, prepared, err
}

func prepareForwardingProfile(ctx context.Context, args []string, observe func(string, *os.File)) (p forwardingProfilePlan, body []byte, returned error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) > 80 {
		return p, nil, errSourceControlInput
	}
	p = forwardingProfilePlan{Schema: "ardents-forwarding-profile-plan-v1", Reference: time.Now().UTC().Truncate(time.Second)}
	flags := flag.NewFlagSet("forwarding prepare-profile", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var bundlePath, inventoryPath, output string
	duties := forwardingEpochFlags{maximum: closedprofile.MaxNodes}
	flags.StringVar(&p.Network, "network-id", "", "explicit Network ID")
	flags.StringVar(&p.Authority, "authority-key", "", "independent State authority public pin")
	flags.StringVar(&p.PermissionAuthority, "issuance-authority-key", "", "separate Admission permission authority public pin")
	flags.StringVar(&bundlePath, "bundle", "", "original verified public Epoch bundle")
	flags.StringVar(&inventoryPath, "token-inventory", "", "original public Admission key inventory")
	flags.Var(&duties, "duty", "Node-ID:Role-Domain:subrole in canonical Node order; repeat at most32")
	flags.StringVar(&output, "output", "", "fresh public profile preparation directory")
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 7 || !forwardingUniqueOptions(args, "duty") || !sourceControlPath(output) {
		return p, nil, errSourceControlInput
	}
	inputs := &sourceControlInputs{ctx: ctx}
	defer func() {
		returned = errors.Join(returned, inputs.close(), p.current(time.Now().UTC()), ctx.Err())
		if returned != nil {
			body = nil
		}
	}()
	for _, path := range []string{bundlePath, inventoryPath} {
		if !sourceControlPath(path) || path == output || strings.HasPrefix(path, output+"/") {
			return p, nil, errSourceControlInput
		}
	}
	if bundlePath == inventoryPath {
		return p, nil, errSourceControlInput
	}
	var err error
	p.Bundle, err = inputs.read(bundlePath, 1<<20)
	if err != nil {
		return p, nil, err
	}
	raw, err := inputs.read(inventoryPath, 64<<10)
	if err != nil {
		return p, nil, err
	}
	if json.Unmarshal(raw, &p.Tokens) != nil {
		return p, nil, errSourceControlInput
	}
	canonical, err := json.Marshal(p.Tokens)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return p, nil, errSourceControlInput
	}
	for _, text := range duties.values {
		fields := strings.Split(text, ":")
		if len(fields) != 3 {
			return p, nil, errSourceControlInput
		}
		domain, e1 := strconv.ParseUint(fields[1], 10, 8)
		subrole, e2 := strconv.ParseUint(fields[2], 10, 8)
		if e1 != nil || e2 != nil || strconv.FormatUint(domain, 10) != fields[1] || strconv.FormatUint(subrole, 10) != fields[2] {
			return p, nil, errSourceControlInput
		}
		p.Duties = append(p.Duties, forwardingProfileDuty{fields[0], uint8(domain), uint8(subrole)})
	}
	prepared, err := p.prepare(time.Now().UTC())
	if err != nil {
		return p, nil, err
	}
	raw, err = json.Marshal(p)
	if err != nil || len(raw)+1 > maximumForwardingProfilePlan {
		return p, nil, errSourceControlInput
	}
	check := func() error { return errors.Join(p.current(time.Now().UTC()), inputs.checkProtected(), ctx.Err()) }
	if err := persistSourceArtifacts(ctx, output, []sourceArtifactFile{{"profile-plan.json", append(raw, '\n')}, {"profile.unsigned", prepared.body}}, check, observe); err != nil {
		return p, nil, err
	}
	return p, bytes.Clone(prepared.body), check()
}
