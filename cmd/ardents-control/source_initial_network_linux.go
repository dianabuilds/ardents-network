package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

type sourceInitialID [32]byte

func (id sourceInitialID) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(id[:]))
}
func (id *sourceInitialID) UnmarshalJSON(raw []byte) error {
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return errSourceControlInput
	}
	value, err := hex.DecodeString(text)
	if err != nil || len(value) != 32 || hex.EncodeToString(value) != text {
		return errSourceControlInput
	}
	copy(id[:], value)
	return nil
}

// One honest family/Record and genesis Epoch, not a full deployment, accepted
// Route profile or candidate/duty operator. Node/State keys remain distinct.
type sourceInitialNetworkPlan struct {
	Schema    string          `json:"schema"`
	Reference time.Time       `json:"reference_time"`
	Network   sourceInitialID `json:"network_id"`
	Node      sourceInitialID `json:"node_id"`
	NodeKey   sourceInitialID `json:"node_public_key"`
	StateKey  sourceInitialID `json:"state_public_key"`
	Family    string          `json:"family"`
	Endpoint  string          `json:"endpoint"`
	From      time.Time       `json:"valid_from"`
	Until     time.Time       `json:"valid_until"`
	Seed      sourceInitialID `json:"assignment_seed"`
	Domains   []string        `json:"domains"`
}

func decodeSourceInitialNetwork(raw []byte, now time.Time) (sourceInitialNetworkPlan, error) {
	var p sourceInitialNetworkPlan
	if len(raw) == 0 || len(raw) > 16<<10 || sourceDeclarationObject(raw, &p, "schema", "reference_time", "network_id", "node_id", "node_public_key", "state_public_key", "family", "endpoint", "valid_from", "valid_until", "assignment_seed", "domains") != nil || p.Schema != "ardents-source-initial-network-plan-v1" ||
		p.Network == (sourceInitialID{}) || p.Node == (sourceInitialID{}) || p.Seed == (sourceInitialID{}) || p.NodeKey == (sourceInitialID{}) || p.StateKey == (sourceInitialID{}) || p.NodeKey == p.StateKey || len(p.Domains) < 1 || len(p.Domains) > 16 ||
		p.Reference.IsZero() || p.Reference.Before(now.Add(-5*time.Minute)) || p.Reference.After(now.Add(time.Minute)) || now.Before(p.From) || !now.Before(p.Until) || p.Reference.Before(p.From) || !p.Reference.Before(p.Until) || p.Until.Sub(p.From) > 7*24*time.Hour {
		return sourceInitialNetworkPlan{}, errSourceControlInput
	}
	host, _, err := net.SplitHostPort(p.Endpoint)
	if err != nil || net.ParseIP(host) == nil {
		return sourceInitialNetworkPlan{}, errSourceControlInput
	}
	// Network owns exact grammar, intervals, capacity and canonical text. No
	// Source composition reimplements eligibility/assignment or signs raw input.
	_, err = epoch.PrepareInitialClosedRecord(epoch.InitialClosedRecord{NetworkID: p.Network, NodeID: p.Node, PublicKey: ed25519.PublicKey(p.NodeKey[:]), Family: p.Family, Endpoint: p.Endpoint,
		ValidFrom: p.From, ValidUntil: p.Until, Carrier: epoch.CarrierClosedTCP, Capacity: 1})
	return p, err
}

func runSourceInitialNetwork(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errSourceControlInput
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	artifact, err := prepareSourceInitialNetwork(ctx, args, nil)
	if err != nil {
		return &sourceControlFailure{err}
	}
	digest := sha256.Sum256(artifact)
	raw, err := json.Marshal(struct {
		Operation   string `json:"operation"`
		Artifact    string `json:"artifact_sha256"`
		Accepted    bool   `json:"state_accepted"`
		Independent bool   `json:"independent_families"`
	}{
		"source." + args[0], hex.EncodeToString(digest[:]), false, false})
	if err != nil {
		return err
	}
	return writeSourceControlBytes(ctx, out, append(raw, '\n'))
}

func prepareSourceInitialNetwork(ctx context.Context, args []string, observe func(string, *os.File)) (result []byte, returned error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) < 3 {
		return nil, errSourceControlInput
	}
	operation := args[0]
	recordPurpose := operation == "prepare-initial-record" || operation == "sign-initial-record"
	signing := operation == "sign-initial-record" || operation == "sign-initial-epoch"
	if operation != "prepare-initial-record" && operation != "prepare-initial-epoch" && !signing {
		return nil, errSourceControlInput
	}
	wanted := 3
	if !recordPurpose {
		wanted++
	}
	if signing {
		wanted += 3
	}
	if len(args) != wanted {
		return nil, errSourceControlInput
	}
	// SHA is a reviewed public commitment, not a path or an accepting flag.
	var pin string
	if signing {
		pin = args[len(args)-3]
		decoded, err := hex.DecodeString(pin)
		if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != pin {
			return nil, errSourceControlInput
		}
	}
	var paths []string
	for i, path := range args[1:] {
		if signing && i+1 == len(args)-3 {
			continue
		}
		if !sourceControlPath(path) {
			return nil, errSourceControlInput
		}
		for _, previous := range paths {
			if path == previous {
				return nil, errSourceControlInput
			}
		}
		paths = append(paths, path)
	}
	inputs := &sourceControlInputs{ctx: ctx}
	defer func() {
		returned = errors.Join(returned, inputs.close(), ctx.Err())
		if returned != nil {
			result = nil
		}
	}()
	raw, err := inputs.read(args[1], 16<<10)
	if err != nil {
		return nil, err
	}
	p, err := decodeSourceInitialNetwork(raw, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	check := func() error {
		if _, err := decodeSourceInitialNetwork(raw, time.Now().UTC()); err != nil {
			return err
		}
		return inputs.checkProtected()
	}
	if err := check(); err != nil {
		return nil, err
	}
	message, err := epoch.PrepareInitialClosedRecord(epoch.InitialClosedRecord{NetworkID: p.Network, NodeID: p.Node, ValidFrom: p.From, ValidUntil: p.Until,
		Family: p.Family, Endpoint: p.Endpoint, Carrier: epoch.CarrierClosedTCP, Capacity: 1, PublicKey: ed25519.PublicKey(p.NodeKey[:])})
	if err != nil {
		return nil, err
	}
	name := "node-record.signing-input"
	var record []byte
	if !recordPurpose {
		record, err = inputs.read(args[2], epoch.MaxRecordBytes)
		if err != nil {
			return nil, err
		}
		if _, err := epoch.VerifyInitialClosedRecord(message, record); err != nil {
			return nil, err
		}
		message, err = epoch.PrepareInitialClosedEpoch(epoch.InitialClosedEpoch{NetworkID: p.Network, AssignmentSeed: p.Seed, ValidFrom: p.From, ValidUntil: p.Until, Domains: p.Domains,
			Records: [][]byte{record}, AuthorityKeys: []ed25519.PublicKey{ed25519.PublicKey(p.StateKey[:])}, Carrier: epoch.CarrierClosedTCP})
		if err != nil {
			return nil, err
		}
		name = "epoch.unsigned"
	}
	if !signing {
		if err := persistSourceArtifacts(ctx, args[len(args)-1], []sourceArtifactFile{{name, message}}, check, observe); err != nil {
			return nil, err
		}
		return bytes.Clone(message), check()
	}
	prepared, err := inputs.read(args[len(args)-4], 1<<20)
	if err != nil {
		return nil, err
	}
	preparedDigest := sha256.Sum256(message)
	if !bytes.Equal(prepared, message) || hex.EncodeToString(preparedDigest[:]) != pin {
		return nil, errSourceControlInput
	}
	keyRaw, err := inputs.read(args[len(args)-2], 4096)
	if err != nil {
		return nil, err
	}
	info := inputs.files[len(inputs.files)-1].info
	if info.Mode() != 0600 || info.Sys().(*syscall.Stat_t).Uid != 0 {
		return nil, errSourceControlInput
	}
	key, err := parseSourceSigningKey(keyRaw)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	expected := p.NodeKey
	if !recordPurpose {
		expected = p.StateKey
	}
	if !bytes.Equal(key.Public().(ed25519.PublicKey), expected[:]) {
		return nil, errSourceControlInput
	}
	if err := check(); err != nil {
		return nil, err
	}
	if recordPurpose {
		record = append(bytes.Clone(message), ed25519.Sign(key, message)...)
		if _, err := epoch.VerifyInitialClosedRecord(message, record); err != nil {
			return nil, err
		}
		if observe != nil {
			observe("signed", inputs.files[len(inputs.files)-1].file)
		}
		if err := check(); err != nil {
			return nil, err
		}
		if err := persistSourceArtifacts(ctx, args[len(args)-1], []sourceArtifactFile{{"node-record.bin", record}}, check, observe); err != nil {
			return nil, err
		}
		return bytes.Clone(record), check()
	}
	digest, id := sha256.Sum256(message), sha256.Sum256(p.StateKey[:])
	signed := append(bytes.Clone(message), 1)
	signed = append(signed, id[:]...)
	signed = append(signed, ed25519.Sign(key, digest[:])...)
	policy := epoch.Policy{NetworkID: p.Network, Authorities: map[[32]byte]ed25519.PublicKey{id: ed25519.PublicKey(p.StateKey[:])}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: time.Now().UTC()}
	decision, err := epoch.Verify(policy, signed, [][]byte{record}, nil, false)
	if err != nil || len(decision.Candidates) != 1 {
		return nil, errors.Join(errSourceControlInput, err)
	}
	material, err := decision.Materialization(0)
	if err != nil {
		return nil, err
	}
	if _, err := epoch.Verify(policy, signed, [][]byte{record}, [][]byte{material}, true); err != nil {
		return nil, err
	}
	bundle, err := source.EncodeBundle(source.Bundle{Epoch: signed, Inputs: [][]byte{record}, Materials: [][]byte{material}})
	if err != nil {
		return nil, err
	}
	if observe != nil {
		observe("signed", inputs.files[len(inputs.files)-1].file)
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := persistSourceArtifacts(ctx, args[len(args)-1], []sourceArtifactFile{{"epoch.bin", signed}, {"material-00.bin", material}, {"source-bundle.bin", bundle}}, check, observe); err != nil {
		return nil, err
	}
	return bytes.Clone(bundle), check()
}
