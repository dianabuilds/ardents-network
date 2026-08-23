package stage6evidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
	nameresolution "github.com/dianabuilds/ardents-network/internal/naming/resolution"
)

type resolutionCellEvidence struct {
	Envelopes                  [][]byte
	Classes                    []string
	RelayRequests              uint32
	GatewayRequests            uint32
	Resolved                   uint32
	Rejected                   uint32
	Exclusions                 [32]byte
	Contacts                   uint32
	Alternate                  uint32
	Resolvers                  []resolutionResolverView
	Relays                     []resolutionRelayView
	Gateways                   []resolutionGatewayView
	NamespaceProof             []byte
	NamespaceRecords           [][]byte
	DeepName                   string
	DeepNamespaceProof         []byte
	DeepNamespaceRecords       [][]byte
	DeepProofBytes             uint32
	NamespaceTransitions       [][]byte
	NamespaceClaim             namespace.ClaimProof
	NamespaceClaimInputs       []claimInputEvidence
	NamespaceClaimRejections   []claimRejectionEvidence
	NamespaceClaimAuthorityIDs [][32]byte
	NamespaceClaimPublicKeys   [][32]byte
	NamespaceClaimThreshold    uint8
	NamespaceClaimMaximum      uint32
	EpochSignerIDs             [][32]byte
	EpochPublicKeys            [][32]byte
	EpochThreshold             uint8
}

type resolutionResolverView struct {
	Operation  string   `json:"operation"`
	Name       string   `json:"name"`
	Isolation  [32]byte `json:"isolation"`
	Network    [32]byte `json:"network"`
	Nonce      [32]byte `json:"nonce"`
	Target     [32]byte `json:"target"`
	Relay      [32]byte `json:"relay"`
	Gateway    [32]byte `json:"gateway"`
	Rendezvous [32]byte `json:"rendezvous"`
	Deadline   int64    `json:"deadline"`
	Result     string   `json:"result"`
	Generation uint64   `json:"generation"`
	Revision   uint64   `json:"revision"`
}

type resolutionGatewayView struct {
	Operation  string   `json:"operation"`
	Name       string   `json:"name"`
	Network    [32]byte `json:"network"`
	Nonce      [32]byte `json:"nonce"`
	Target     [32]byte `json:"target"`
	Deadline   int64    `json:"deadline"`
	Result     string   `json:"result"`
	Generation uint64   `json:"generation"`
	Revision   uint64   `json:"revision"`
}

type resolutionRelayView struct {
	Origin        string   `json:"origin"`
	Gateway       string   `json:"gateway"`
	Request       [32]byte `json:"request"`
	Response      [32]byte `json:"response"`
	RequestBytes  uint64   `json:"request_bytes"`
	ResponseBytes uint64   `json:"response_bytes"`
	KeyID         byte     `json:"key_id"`
	Deadline      int64    `json:"deadline"`
}

func runResolutionCell(trace *traceRecord) error {
	fixture, err := newResolutionFixture()
	if err != nil {
		return err
	}
	defer fixture.close()
	evidence := resolutionCellEvidence{}
	if trace.Cell == "D4" {
		return runUnavailableResolution(trace, fixture, &evidence)
	}
	contexts := [][32]byte{{51}, {52}}
	for index, isolation := range contexts {
		selection, selectionErr := fixture.admitted("alice", isolation, byte(index+1))
		if selectionErr != nil {
			return selectionErr
		}
		resolver, openErr := nameresolution.OpenEvidence(fixture.view, selection, fixture.profile(), isolation,
			fixture.relay.Client().Transport.(*http.Transport))
		if openErr != nil {
			return openErr
		}
		result, resolveErr := resolver.Resolve(context.Background(), "alice", fixture.now)
		if resolveErr != nil || result.Class != "resolved" || result.Binding.Target != [32]byte{1} {
			return errors.New("private resolution did not return the authenticated binding")
		}
		evidence.Classes = append(evidence.Classes, result.Class)
		role := resolver.RoleEvidence()
		evidence.Resolvers = append(evidence.Resolvers, resolutionResolverView{Operation: role.Operation,
			Name: role.Name, Isolation: role.Isolation, Network: role.Network, Nonce: role.Nonce,
			Target: role.Target, Relay: role.Relay, Gateway: role.Gateway, Rendezvous: role.Rendezvous,
			Deadline: role.Deadline, Result: role.Result, Generation: role.Generation, Revision: role.Revision})
		identities, families, rendezvous := resolver.ConnectionExclusions()
		if len(identities) != 2 || len(families) != 2 || rendezvous != [32]byte{3} {
			return errors.New("resolution roles were not excluded from the connection")
		}
		if index == 0 {
			evidence.Exclusions = exclusionCommitment(identities, families, rendezvous)
		}
	}
	evidence.Envelopes = fixture.capture.evidence()
	evidence.Gateways, evidence.Relays = fixture.roleEvidence()
	evidence.NamespaceProof, err = fixture.store.Lookup("alice", 1)
	if err != nil {
		return err
	}
	evidence.DeepName, evidence.DeepNamespaceProof, evidence.DeepNamespaceRecords, err =
		deepNamespaceEvidence(fixture.materialization, fixture.now)
	if err != nil {
		return err
	}
	evidence.DeepProofBytes = uint32(len(evidence.DeepNamespaceProof))
	for _, record := range fixture.records {
		evidence.NamespaceRecords = append(evidence.NamespaceRecords, append([]byte(nil), record...))
	}
	for _, transition := range fixture.transitions {
		evidence.NamespaceTransitions = append(evidence.NamespaceTransitions, append([]byte(nil), transition...))
	}
	evidence.NamespaceClaim = fixture.claimProof
	evidence.NamespaceClaimInputs = claimInputs(fixture.claimProof)
	evidence.NamespaceClaimRejections = claimRejections(fixture.claimProof)
	evidence.NamespaceClaimThreshold = uint8(fixture.claimOrder.Threshold)
	evidence.NamespaceClaimMaximum = fixture.claimOrder.MaximumClaims
	for _, id := range namespaceClaimPolicyIDs(fixture.claimOrder) {
		evidence.NamespaceClaimAuthorityIDs = append(evidence.NamespaceClaimAuthorityIDs, id)
		var public [32]byte
		copy(public[:], fixture.claimOrder.Authorities[id])
		evidence.NamespaceClaimPublicKeys = append(evidence.NamespaceClaimPublicKeys, public)
	}
	for _, id := range namespacePolicyIDs(fixture.materialization.policy) {
		evidence.EpochSignerIDs = append(evidence.EpochSignerIDs, id)
		var public [32]byte
		copy(public[:], fixture.materialization.policy.Authorities[id])
		evidence.EpochPublicKeys = append(evidence.EpochPublicKeys, public)
	}
	evidence.EpochThreshold = uint8(fixture.materialization.policy.Threshold)
	evidence.RelayRequests, evidence.GatewayRequests, evidence.Resolved, evidence.Rejected = fixture.observations()
	if len(evidence.Envelopes) != 2 || len(evidence.Envelopes[0]) != len(evidence.Envelopes[1]) ||
		bytes.Equal(evidence.Envelopes[0], evidence.Envelopes[1]) {
		return errors.New("private resolution envelopes are not fixed and fresh")
	}
	for index, envelope := range evidence.Envelopes {
		target := [32]byte{1}
		if bytes.Contains(envelope, []byte("alice")) || bytes.Contains(envelope, target[:]) ||
			bytes.Contains(envelope, contexts[index][:]) {
			return errors.New("relay observed a forbidden resolution field")
		}
	}
	trace.Fields = []string{"relay-opaque", "gateway-name-only", "contexts-unlinked"}
	return retainResolutionEvidence(trace, evidence)
}

func runUnavailableResolution(trace *traceRecord, fixture resolutionFixture, evidence *resolutionCellEvidence) error {
	selection, err := fixture.admitted("alice", [32]byte{53}, 3)
	if err != nil {
		return err
	}
	selected := fixture.view.Candidates[0].Endpoint
	transport := &http.Transport{DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
		if address == selected {
			evidence.Contacts++
		} else {
			evidence.Alternate++
		}
		return nil, errors.New("selected private path unavailable")
	}}
	resolver, err := nameresolution.OpenEvidence(fixture.view, selection, fixture.profile(), [32]byte{53}, transport)
	if err != nil {
		return err
	}
	result, resolveErr := resolver.Resolve(context.Background(), "alice", fixture.now)
	if resolveErr == nil || result.Class != "private resolution unavailable" || evidence.Contacts != 1 || evidence.Alternate != 0 {
		return errors.New("private resolution used a forbidden fallback")
	}
	evidence.Classes = []string{result.Class}
	trace.Fields = []string{"private resolution unavailable", "one-selected-contact", "no-fallback"}
	return retainResolutionEvidence(trace, *evidence)
}

func retainResolutionEvidence(trace *traceRecord, evidence resolutionCellEvidence) error {
	raw, err := json.Marshal(evidence)
	if err == nil {
		trace.Auxiliary = raw
	}
	return err
}

func exclusionCommitment(identities [][32]byte, families []string, rendezvous [32]byte) [32]byte {
	out := []byte("ardents-stage-6-resolution-exclusions-v1\x00")
	for _, identity := range identities {
		out = append(out, identity[:]...)
	}
	for _, family := range families {
		out = append(out, family...)
		out = append(out, 0)
	}
	return sha256.Sum256(append(out, rendezvous[:]...))
}
