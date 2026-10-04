package network

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
)

// ClosedNode supplies a signing key and the intended profile role. An empty
// Family is deterministically selected to produce that Epoch-assigned domain.
type ClosedNode struct {
	RecordSpec
	RoleDomain, Subrole byte
}

type ClosedTokenKey struct {
	WindowStart time.Time
	Class       byte
	SPKI        []byte
}

// ClosedSpec is signed input data only. The receiving test must open State and
// accept these bytes; this builder cannot create trusted runtime projections.
type ClosedSpec struct {
	NetworkID, Seed, IssuanceAuthority [32]byte
	Previous                           [32]byte
	Number                             uint64
	NotBefore, NotAfter                time.Time
	Authority                          ed25519.PrivateKey
	Nodes                              []ClosedNode
	Keys                               []ClosedTokenKey
}

type Closed struct {
	Epoch   Epoch
	Profile []byte
	Nodes   []ClosedNode
}

func BuildClosed(spec ClosedSpec) (Closed, error) {
	if len(spec.Authority) != ed25519.PrivateKeySize || len(spec.Nodes) == 0 || len(spec.Nodes) > 64 {
		return Closed{}, errors.New("closed fixture specification is invalid")
	}
	roleDomains := []string{"initiator", "rendezvous", "responder", "introduction"}
	domains := append([]string(nil), roleDomains...)
	sort.Strings(domains)
	nodes := append([]ClosedNode(nil), spec.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return bytes.Compare(nodes[i].NodeID[:], nodes[j].NodeID[:]) < 0 })
	var records []Record
	var inputs [][]byte
	var issuer [32]byte
	for index := range nodes {
		node := &nodes[index]
		if node.RoleDomain < 1 || node.RoleDomain > 4 {
			return Closed{}, errors.New("invalid fixture role")
		}
		node.NetworkID = spec.NetworkID
		if node.Family == "" {
			for attempt := 0; attempt < 1024; attempt++ {
				family := fmt.Sprintf("closed-%d-%d", index, attempt)
				domain, err := assignment.Select(spec.NetworkID, spec.Number, spec.Seed, family, domains)
				if err != nil {
					return Closed{}, err
				}
				if domain == roleDomains[node.RoleDomain-1] {
					node.Family = family
					break
				}
			}
		}
		record, err := BuildRecord(node.RecordSpec)
		if err != nil {
			return Closed{}, err
		}
		records = append(records, record)
		inputs = append(inputs, record.Raw)
		if node.RoleDomain == 2 && node.Subrole == 6 {
			issuer = node.NodeID
		}
	}
	epoch, err := BuildEpoch(EpochSpec{NetworkID: spec.NetworkID, Number: spec.Number, Previous: spec.Previous, ValidFrom: spec.NotBefore, ValidUntil: spec.NotAfter,
		Inputs: inputs, Accepted: records, AssignmentSeed: spec.Seed, Domains: domains, Authorities: []ed25519.PrivateKey{spec.Authority}, Profile: "ardents-route-v3", Version: 3})
	if err != nil {
		return Closed{}, err
	}
	keys := append([]ClosedTokenKey(nil), spec.Keys...)
	if len(keys) == 0 {
		spki, err := closedFixtureSPKI()
		if err != nil {
			return Closed{}, err
		}
		keys = []ClosedTokenKey{{WindowStart: spec.NotBefore.Truncate(time.Hour), Class: 1, SPKI: spki}}
	}
	var body bytes.Buffer
	body.WriteString("ARDCPR03")
	u16(&body, 3)
	body.Write(spec.NetworkID[:])
	body.Write(epoch.Digest[:])
	u64(&body, spec.Number)
	body.Write(epoch.Digest[:])
	i64(&body, spec.NotBefore.Unix())
	i64(&body, spec.NotAfter.Unix())
	body.Write(issuer[:])
	body.Write(spec.IssuanceAuthority[:])
	u16(&body, uint16(len(nodes)))
	for index, node := range nodes {
		digest := sha256.Sum256(records[index].Raw)
		body.Write(node.NodeID[:])
		body.Write(digest[:])
		body.WriteByte(node.RoleDomain)
		body.WriteByte(node.Subrole)
		u64(&body, node.Generation)
	}
	u16(&body, uint16(len(keys)))
	for _, key := range keys {
		i64(&body, key.WindowStart.Unix())
		body.WriteByte(key.Class)
		u16(&body, uint16(len(key.SPKI)))
		body.Write(key.SPKI)
	}
	raw := body.Bytes()
	raw = append(raw, ed25519.Sign(spec.Authority, append([]byte("ardents-closed-profile-v3\x00"), raw...))...)
	return Closed{Epoch: epoch, Profile: raw, Nodes: nodes}, nil
}
