package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

type forwardingEpochResult struct {
	bundle []byte
	plan   forwardingEpochPlan
}

// This fixed genesis signature accepts only its original rebuilt AREP-v3
// preparation and one explicit State authority. It cannot sign an arbitrary
// message, successor, role profile or Source declaration.
func signForwardingEpoch(ctx context.Context, args []string, observe func(string, *os.File)) (result *forwardingEpochResult, returned error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) != 5 {
		return nil, errSourceControlInput
	}
	for _, index := range []int{0, 1, 3, 4} {
		if !sourceControlPath(args[index]) {
			return nil, errSourceControlInput
		}
		for _, other := range []int{0, 1, 3, 4} {
			if index != other && (args[index] == args[other] || index == 4 && strings.HasPrefix(args[other], args[index]+"/")) {
				return nil, errSourceControlInput
			}
		}
	}
	pin, err := hex.DecodeString(args[2])
	if err != nil || len(pin) != 32 || hex.EncodeToString(pin) != args[2] {
		return nil, errSourceControlInput
	}
	inputs := &sourceControlInputs{ctx: ctx}
	var admitted *forwardingEpochPlan
	defer func() {
		returned = errors.Join(returned, inputs.close(), ctx.Err())
		if admitted != nil {
			returned = errors.Join(returned, admitted.current(time.Now().UTC()))
		}
		if returned != nil {
			result = nil
		}
	}()
	raw, err := inputs.read(args[0], maximumForwardingEpochPlan)
	if err != nil {
		return nil, err
	}
	p, message, err := decodeForwardingEpochPlan(raw, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	admitted = &p
	prepared, err := inputs.read(args[1], epoch.MaxEpochBytes)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(message)
	if !bytes.Equal(prepared, message) || hex.EncodeToString(sum[:]) != args[2] {
		return nil, errSourceControlInput
	}
	check := func() error { return errors.Join(p.current(time.Now().UTC()), inputs.checkProtected()) }
	if err := check(); err != nil {
		return nil, err
	}
	keyRaw, err := inputs.read(args[3], 4096)
	if err != nil {
		return nil, err
	}
	private := inputs.files[len(inputs.files)-1]
	if private.info.Mode() != 0600 || private.info.Sys().(*syscall.Stat_t).Uid != 0 {
		return nil, errSourceControlInput
	}
	key, err := parseSourceSigningKey(keyRaw)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	authority, err := forwardingPublic32(p.Authority)
	if err != nil || !bytes.Equal(key[32:], authority[:]) {
		return nil, errSourceControlInput
	}
	if err := check(); err != nil {
		return nil, err
	}
	id := sha256.Sum256(authority[:])
	signed := append(bytes.Clone(message), 1)
	signed = append(signed, id[:]...)
	signed = append(signed, ed25519.Sign(key, sum[:])...)
	network, _ := forwardingPublic32(p.Network)
	policy := epoch.Policy{NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{id: ed25519.PublicKey(authority[:])}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: time.Now().UTC()}
	records := make([][]byte, len(p.Records))
	for index, record := range p.Records {
		records[index] = record.Signed
	}
	decision, err := epoch.Verify(policy, signed, records, nil, false)
	if err != nil || len(decision.Candidates) != len(records) {
		return nil, errors.Join(errSourceControlInput, err)
	}
	node, _ := forwardingPublic32(p.Node)
	selected := -1
	for index, candidate := range decision.Candidates {
		if candidate.NodeID == node {
			selected = index
		}
	}
	if selected < 0 {
		return nil, errSourceControlInput
	}
	material, err := decision.Materialization(uint32(selected))
	if err != nil {
		return nil, err
	}
	policy.MaterializationIndex = uint32(selected)
	verified, err := epoch.Verify(policy, signed, records, [][]byte{material}, true)
	if err != nil || !verified.Snapshot.RecordPresent || verified.Snapshot.NodeID != node {
		return nil, errors.Join(errSourceControlInput, err)
	}
	bundle, err := source.EncodeBundle(source.Bundle{Epoch: signed, Inputs: records, Materials: [][]byte{material}})
	if err != nil {
		return nil, err
	}
	if observe != nil {
		observe("signed", private.file)
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := persistSourceArtifacts(ctx, args[4], []sourceArtifactFile{{"epoch.bin", signed}, {"material.bin", material}, {"state-bundle.bin", bundle}}, check, observe); err != nil {
		return nil, err
	}
	return &forwardingEpochResult{bytes.Clone(bundle), p}, check()
}
