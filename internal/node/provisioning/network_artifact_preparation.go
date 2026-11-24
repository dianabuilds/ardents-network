package provisioning

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

var errNetworkPreparation = errors.New("network public preparation unavailable")

// Public plans use exact lowercase hex, never Go's truncating fixed-array JSON
// decoder. No private key, root, authority selection or signing input is read.
type preparationID [32]byte

func (v *preparationID) UnmarshalJSON(raw []byte) error {
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return errNetworkPreparation
	}
	id, ok := canonicaldigest.Decode(text)
	if !ok {
		return errNetworkPreparation
	}
	*v = id
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
	Domains     []*string          `json:"domains"`
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

func (p *profilePreparationNode) UnmarshalJSON(raw []byte) error {
	type fields profilePreparationNode
	return artifacts.DecodeDeclaration(raw, (*fields)(p), "node_id", "record_digest", "role_domain", "subrole", "duty_generation")
}

func (p *profilePreparationKey) UnmarshalJSON(raw []byte) error {
	type fields profilePreparationKey
	return artifacts.DecodeDeclaration(raw, (*fields)(p), "window_start", "class", "spki")
}

func preparationTime(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil || t.Format(time.RFC3339) != value || t.Location() != time.UTC || t.Nanosecond() != 0 {
		return time.Time{}, errNetworkPreparation
	}
	return t, nil
}

// PrepareInitialNodeRecord returns the validated public facts and exact unsigned Record.
func PrepareInitialNodeRecord(raw []byte) (epoch.InitialClosedRecord, []byte, error) {
	var p recordPreparationPlan
	if artifacts.DecodeDeclaration(raw, &p, "schema", "network_id", "node_id", "valid_from", "valid_until", "family", "endpoint", "carrier", "capacity", "public_key") != nil || p.Schema != "ardents-initial-node-record-plan-v1" || p.Carrier != epoch.CarrierClosedTCP {
		return epoch.InitialClosedRecord{}, nil, errNetworkPreparation
	}
	from, e1 := preparationTime(p.From)
	until, e2 := preparationTime(p.Until)
	if errors.Join(e1, e2) != nil {
		return epoch.InitialClosedRecord{}, nil, errNetworkPreparation
	}
	input := epoch.InitialClosedRecord{NetworkID: p.Network, NodeID: p.Node, ValidFrom: from, ValidUntil: until, Family: p.Family, Endpoint: p.Endpoint, Carrier: p.Carrier, Capacity: p.Capacity, PublicKey: append(ed25519.PublicKey(nil), p.PublicKey[:]...)}
	message, err := epoch.PrepareInitialClosedRecord(input)
	return input, message, err
}

// PrepareInitialEpoch returns the validated public facts and exact unsigned Epoch.
func PrepareInitialEpoch(raw []byte) (epoch.InitialClosedEpoch, []byte, error) {
	var p epochPreparationPlan
	if artifacts.DecodeDeclaration(raw, &p, "schema", "network_id", "assignment_seed", "valid_from", "valid_until", "domains", "records", "authority_keys") != nil || p.Schema != "ardents-initial-epoch-plan-v1" || len(p.Records) == 0 || len(p.Records) > 64 || len(p.Authorities) == 0 || len(p.Authorities) > 16 || len(p.Domains) == 0 || len(p.Domains) > 16 {
		return epoch.InitialClosedEpoch{}, nil, errNetworkPreparation
	}
	from, e1 := preparationTime(p.From)
	until, e2 := preparationTime(p.Until)
	if errors.Join(e1, e2) != nil {
		return epoch.InitialClosedEpoch{}, nil, errNetworkPreparation
	}
	input := epoch.InitialClosedEpoch{NetworkID: p.Network, AssignmentSeed: p.Seed, ValidFrom: from, ValidUntil: until, Carrier: epoch.CarrierClosedTCP}
	for _, domain := range p.Domains {
		if domain == nil {
			return epoch.InitialClosedEpoch{}, nil, errNetworkPreparation
		}
		input.Domains = append(input.Domains, *domain)
	}
	for _, record := range p.Records {
		input.Records = append(input.Records, record)
	}
	for _, key := range p.Authorities {
		input.AuthorityKeys = append(input.AuthorityKeys, append(ed25519.PublicKey(nil), key[:]...))
	}
	message, err := epoch.PrepareInitialClosedEpoch(input)
	return input, message, err
}

// PrepareClosedNetworkProfile returns the validated public facts and exact unsigned profile.
func PrepareClosedNetworkProfile(raw []byte) (closedprofile.Input, []byte, error) {
	var p profilePreparationPlan
	if artifacts.DecodeDeclaration(raw, &p, "schema", "network_id", "state_generation", "epoch_digest", "issuer_node_id", "issuance_authority_key", "epoch", "not_before", "not_after", "nodes", "token_keys") != nil || p.Schema != "ardents-closed-profile-plan-v1" || len(p.Nodes) == 0 || len(p.Nodes) > closedprofile.MaxNodes || len(p.Keys) == 0 || len(p.Keys) > closedprofile.MaxKeys {
		return closedprofile.Input{}, nil, errNetworkPreparation
	}
	from, e1 := preparationTime(p.From)
	until, e2 := preparationTime(p.Until)
	if errors.Join(e1, e2) != nil {
		return closedprofile.Input{}, nil, errNetworkPreparation
	}
	input := closedprofile.Input{NetworkID: p.Network, StateGeneration: p.Generation, EpochDigest: p.Digest, IssuerNodeID: p.Issuer, IssuanceAuthorityKey: p.Authority, Epoch: p.Epoch, NotBefore: from, NotAfter: until}
	for _, n := range p.Nodes {
		input.Nodes = append(input.Nodes, closedprofile.NodeInput{NodeID: n.Node, RecordDigest: n.Record, RoleDomain: n.Domain, Subrole: n.Subrole, DutyGeneration: n.Duty})
	}
	for _, k := range p.Keys {
		window, e := preparationTime(k.Window)
		if e != nil {
			return closedprofile.Input{}, nil, errNetworkPreparation
		}
		input.TokenKeys = append(input.TokenKeys, closedprofile.TokenKeyInput{WindowStart: window, Class: k.Class, SPKI: k.SPKI})
	}
	message, err := closedprofile.Prepare(input)
	return input, message, err
}
