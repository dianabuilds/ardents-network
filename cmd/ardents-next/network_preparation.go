package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

const maximumNetworkPreparationPlan = 8 << 20

var errNetworkPreparation = errors.New("network public preparation unavailable")

// Public plans use exact lowercase hex, never Go's truncating fixed-array JSON
// decoder. No private key, root, authority selection or signing input is read.
type preparationID [32]byte

func (v *preparationID) UnmarshalJSON(raw []byte) error {
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return errNetworkPreparation
	}
	b, err := admissionHex(text, 32)
	if err != nil {
		return errNetworkPreparation
	}
	copy(v[:], b)
	return nil
}

type preparationBytes []byte

func (v *preparationBytes) UnmarshalJSON(raw []byte) error {
	var text string
	if json.Unmarshal(raw, &text) != nil || len(text) == 0 || len(text) > 2*epoch.MaxRecordBytes {
		return errNetworkPreparation
	}
	b, err := hex.DecodeString(text)
	if err != nil || hex.EncodeToString(b) != text {
		return errNetworkPreparation
	}
	*v = b
	return nil
}

type recordPreparationPlan struct {
	Schema    string        `json:"schema"`
	Network   preparationID `json:"network_id"`
	Node      preparationID `json:"node_id"`
	From      string        `json:"valid_from"`
	Until     string        `json:"valid_until"`
	Family    string        `json:"family"`
	Endpoint  string        `json:"endpoint"`
	Carrier   string        `json:"carrier"`
	Capacity  uint16        `json:"capacity"`
	PublicKey preparationID `json:"public_key"`
}

type epochPreparationPlan struct {
	Schema      string             `json:"schema"`
	Network     preparationID      `json:"network_id"`
	Seed        preparationID      `json:"assignment_seed"`
	From        string             `json:"valid_from"`
	Until       string             `json:"valid_until"`
	Domains     []string           `json:"domains"`
	Records     []preparationBytes `json:"records"`
	Authorities []preparationID    `json:"authority_keys"`
}

type profilePreparationNode struct {
	Node    preparationID `json:"node_id"`
	Record  preparationID `json:"record_digest"`
	Domain  byte          `json:"role_domain"`
	Subrole byte          `json:"subrole"`
	Duty    uint64        `json:"duty_generation"`
}

type profilePreparationKey struct {
	Window string           `json:"window_start"`
	Class  byte             `json:"class"`
	SPKI   preparationBytes `json:"spki"`
}

type profilePreparationPlan struct {
	Schema     string                   `json:"schema"`
	Network    preparationID            `json:"network_id"`
	Generation preparationID            `json:"state_generation"`
	Digest     preparationID            `json:"epoch_digest"`
	Issuer     preparationID            `json:"issuer_node_id"`
	Authority  preparationID            `json:"issuance_authority_key"`
	Epoch      uint64                   `json:"epoch"`
	From       string                   `json:"not_before"`
	Until      string                   `json:"not_after"`
	Nodes      []profilePreparationNode `json:"nodes"`
	Keys       []profilePreparationKey  `json:"token_keys"`
}

func isNetworkPreparation(operation string) bool {
	return operation == "prepare-initial-node-record" || operation == "prepare-initial-epoch" || operation == "prepare-closed-profile"
}

func preparationObject(raw []byte, value any, keys ...string) error {
	if !requiredAdmissionObject(raw, keys...) || decodeAdmissionObject(raw, value) != nil {
		return errNetworkPreparation
	}
	return nil
}

// Only adaptation lives here. Network owns bytes, eligibility, assignment and
// profile/SPKI rules; the command neither redraws a seed nor invents membership.
func prepareNetworkPublic(operation string, raw []byte) (name string, message []byte, err error) {
	switch operation {
	case "prepare-initial-node-record":
		var p recordPreparationPlan
		if preparationObject(raw, &p, "schema", "network_id", "node_id", "valid_from", "valid_until", "family", "endpoint", "carrier", "capacity", "public_key") != nil || p.Schema != "ardents-initial-node-record-plan-v1" || p.Carrier != epoch.CarrierClosedTCP {
			return "", nil, errNetworkPreparation
		}
		from, e1 := admissionTime(p.From)
		until, e2 := admissionTime(p.Until)
		if errors.Join(e1, e2) != nil {
			return "", nil, errNetworkPreparation
		}
		message, err = epoch.PrepareInitialClosedRecord(epoch.InitialClosedRecord{NetworkID: p.Network, NodeID: p.Node, ValidFrom: from, ValidUntil: until, Family: p.Family, Endpoint: p.Endpoint, Carrier: p.Carrier, Capacity: p.Capacity, PublicKey: append(ed25519.PublicKey(nil), p.PublicKey[:]...)})
		name = "node-record.signing-input"
	case "prepare-initial-epoch":
		var p epochPreparationPlan
		if preparationObject(raw, &p, "schema", "network_id", "assignment_seed", "valid_from", "valid_until", "domains", "records", "authority_keys") != nil || p.Schema != "ardents-initial-epoch-plan-v1" || len(p.Records) == 0 || len(p.Records) > 64 || len(p.Authorities) == 0 || len(p.Authorities) > 16 || len(p.Domains) == 0 || len(p.Domains) > 16 {
			return "", nil, errNetworkPreparation
		}
		from, e1 := admissionTime(p.From)
		until, e2 := admissionTime(p.Until)
		if errors.Join(e1, e2) != nil {
			return "", nil, errNetworkPreparation
		}
		input := epoch.InitialClosedEpoch{NetworkID: p.Network, AssignmentSeed: p.Seed, ValidFrom: from, ValidUntil: until, Domains: p.Domains, Carrier: epoch.CarrierClosedTCP}
		for _, record := range p.Records {
			input.Records = append(input.Records, record)
		}
		for _, key := range p.Authorities {
			input.AuthorityKeys = append(input.AuthorityKeys, append(ed25519.PublicKey(nil), key[:]...))
		}
		message, err = epoch.PrepareInitialClosedEpoch(input)
		name = "epoch.unsigned"
	case "prepare-closed-profile":
		var p profilePreparationPlan
		if preparationObject(raw, &p, "schema", "network_id", "state_generation", "epoch_digest", "issuer_node_id", "issuance_authority_key", "epoch", "not_before", "not_after", "nodes", "token_keys") != nil || p.Schema != "ardents-closed-profile-plan-v1" || len(p.Nodes) == 0 || len(p.Nodes) > closedprofile.MaxNodes || len(p.Keys) == 0 || len(p.Keys) > closedprofile.MaxKeys {
			return "", nil, errNetworkPreparation
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		for field, keys := range map[string][]string{"nodes": {"node_id", "record_digest", "role_domain", "subrole", "duty_generation"}, "token_keys": {"window_start", "class", "spki"}} {
			var entries []json.RawMessage
			if json.Unmarshal(fields[field], &entries) != nil {
				return "", nil, errNetworkPreparation
			}
			for _, entry := range entries {
				if !requiredAdmissionObject(entry, keys...) {
					return "", nil, errNetworkPreparation
				}
			}
		}
		from, e1 := admissionTime(p.From)
		until, e2 := admissionTime(p.Until)
		if errors.Join(e1, e2) != nil {
			return "", nil, errNetworkPreparation
		}
		input := closedprofile.Input{NetworkID: p.Network, StateGeneration: p.Generation, EpochDigest: p.Digest, IssuerNodeID: p.Issuer, IssuanceAuthorityKey: p.Authority, Epoch: p.Epoch, NotBefore: from, NotAfter: until}
		for _, n := range p.Nodes {
			input.Nodes = append(input.Nodes, closedprofile.NodeInput{NodeID: n.Node, RecordDigest: n.Record, RoleDomain: n.Domain, Subrole: n.Subrole, DutyGeneration: n.Duty})
		}
		for _, k := range p.Keys {
			window, e := admissionTime(k.Window)
			if e != nil {
				return "", nil, errNetworkPreparation
			}
			input.TokenKeys = append(input.TokenKeys, closedprofile.TokenKeyInput{WindowStart: window, Class: k.Class, SPKI: k.SPKI})
		}
		message, err = closedprofile.Prepare(input)
		name = "profile.unsigned"
	default:
		return "", nil, errNetworkPreparation
	}
	return
}

func runNetworkPreparation(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 3 || !isNetworkPreparation(args[0]) || !filepath.IsAbs(args[1]) || filepath.Clean(args[1]) != args[1] || !filepath.IsAbs(args[2]) || filepath.Clean(args[2]) != args[2] {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	raw, err := readNetworkPreparation(ctx, args[1])
	if err != nil || len(raw) > maximumNetworkPreparationPlan {
		return 2
	}
	name, message, err := prepareNetworkPublic(args[0], raw)
	if err != nil {
		return 2
	}
	if err = writeNetworkPreparation(ctx, args[2], name, message); err != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	digest := sha256.Sum256(message)
	// Grammar preparation is deliberately weaker than signed State acceptance.
	receipt, err := json.Marshal(struct {
		Schema        string `json:"schema"`
		Operation     string `json:"operation"`
		Artifact      string `json:"artifact"`
		SHA256        string `json:"unsigned_sha256"`
		Signed        bool   `json:"signed"`
		StateAccepted bool   `json:"state_accepted"`
		Validation    string `json:"validation"`
	}{"ardents-network-prepared-v1", args[0], name, hex.EncodeToString(digest[:]), false, false, "public-grammar-only"})
	if err != nil {
		return 1
	}
	if writeNetworkPreparationReceipt(ctx, out, append(receipt, '\n')) != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	return 0
}
