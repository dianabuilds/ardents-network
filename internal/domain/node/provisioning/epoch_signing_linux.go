package provisioning

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
)

type nodeEpochResult struct {
	bundle []byte
	plan   nodeEpochPlan
}

// This fixed genesis signature accepts only its original rebuilt AREP-v3
// preparation and one explicit State authority. It cannot sign an arbitrary
// message, successor, role profile or Source declaration.
func signNodeEpoch(ctx context.Context, role nodeProvisioningRole, args []string, observe func(string, *os.File)) (result *nodeEpochResult, returned error) {
	if !role.valid() || ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) != 5 {
		return nil, artifacts.ErrInput
	}
	for _, index := range []int{0, 1, 3, 4} {
		if !artifacts.AbsolutePath(args[index]) {
			return nil, artifacts.ErrInput
		}
		for _, other := range []int{0, 1, 3, 4} {
			if index != other && (args[index] == args[other] || index == 4 && strings.HasPrefix(args[other], args[index]+"/")) {
				return nil, artifacts.ErrInput
			}
		}
	}
	if _, ok := canonicaldigest.Decode(args[2]); !ok {
		return nil, artifacts.ErrInput
	}
	inputs := artifacts.NewInputs(ctx)
	var admitted *nodeEpochPlan
	defer func() {
		returned = errors.Join(returned, inputs.Close(), ctx.Err())
		if admitted != nil {
			returned = errors.Join(returned, admitted.current(role, time.Now().UTC()))
		}
		if returned != nil {
			result = nil
		}
	}()
	raw, err := inputs.Read(args[0], maximumNodeEpochPlan)
	if err != nil {
		return nil, err
	}
	p, message, err := decodeNodeEpochPlan(role, raw, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	admitted = &p
	prepared, err := inputs.Read(args[1], epoch.MaxEpochBytes)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(message)
	if !bytes.Equal(prepared, message) || hex.EncodeToString(sum[:]) != args[2] {
		return nil, artifacts.ErrInput
	}
	check := func() error { return errors.Join(p.current(role, time.Now().UTC()), inputs.CheckProtected()) }
	if err := check(); err != nil {
		return nil, err
	}
	keyRaw, err := inputs.Read(args[3], 4096)
	if err != nil {
		return nil, err
	}
	private := inputs.Last()
	if private.Info().Mode() != 0600 || private.Info().Sys().(*syscall.Stat_t).Uid != 0 {
		return nil, artifacts.ErrInput
	}
	key, err := artifacts.ParseEd25519Key(keyRaw)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	authority, err := controlPublic32(p.Authority)
	if err != nil || !bytes.Equal(key[32:], authority[:]) {
		return nil, artifacts.ErrInput
	}
	if err := check(); err != nil {
		return nil, err
	}
	id := sha256.Sum256(authority[:])
	signed := append(bytes.Clone(message), 1)
	signed = append(signed, id[:]...)
	signed = append(signed, ed25519.Sign(key, sum[:])...)
	network, _ := controlPublic32(p.Network)
	policy := epoch.Policy{NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{id: ed25519.PublicKey(authority[:])}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: time.Now().UTC()}
	records := make([][]byte, len(p.Records))
	for index, record := range p.Records {
		records[index] = record.Signed
	}
	decision, err := epoch.Verify(policy, signed, records, nil, false)
	if err != nil || len(decision.Candidates) != len(records) {
		return nil, errors.Join(artifacts.ErrInput, err)
	}
	node, _ := controlPublic32(p.Node)
	selected := -1
	for index, candidate := range decision.Candidates {
		if candidate.NodeID == node {
			selected = index
		}
	}
	if selected < 0 {
		return nil, artifacts.ErrInput
	}
	material, err := decision.Materialization(uint32(selected))
	if err != nil {
		return nil, err
	}
	policy.MaterializationIndex = uint32(selected)
	verified, err := epoch.Verify(policy, signed, records, [][]byte{material}, true)
	if err != nil || !verified.Snapshot.RecordPresent || verified.Snapshot.NodeID != node {
		return nil, errors.Join(artifacts.ErrInput, err)
	}
	bundle, err := source.EncodeBundle(source.Bundle{Epoch: signed, Inputs: records, Materials: [][]byte{material}})
	if err != nil {
		return nil, err
	}
	if observe != nil {
		observe("signed", private.Descriptor())
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := artifacts.Publish(ctx, args[4], []artifacts.Content{{Name: "epoch.bin", Data: signed}, {Name: "material.bin", Data: material}, {Name: "state-bundle.bin", Data: bundle}}, check, observe); err != nil {
		return nil, err
	}
	return &nodeEpochResult{bytes.Clone(bundle), p}, check()
}
