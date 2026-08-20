package stage6verify

import (
	"bytes"
	"crypto/sha256"
	"strings"
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
	NamespaceClaim             claimEvidence
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

func verifyResolutionTrace(trace traceRecord) bool {
	var evidence resolutionCellEvidence
	if decodeNestedJSON(trace.Auxiliary, &evidence) != nil || len(trace.Input) != 0 || len(trace.Output) != 0 {
		return false
	}
	if trace.Cell == "D4" {
		return len(evidence.Envelopes) == 0 && equalStrings(evidence.Classes, []string{"private resolution unavailable"}) &&
			evidence.Contacts == 1 && evidence.Alternate == 0 &&
			equalStrings(trace.Fields, []string{"private resolution unavailable", "one-selected-contact", "no-fallback"})
	}
	if !equalStrings(evidence.Classes, []string{"resolved", "resolved"}) || len(evidence.Envelopes) != 2 ||
		len(evidence.Envelopes[0]) == 0 || len(evidence.Envelopes[0]) != len(evidence.Envelopes[1]) ||
		bytes.Equal(evidence.Envelopes[0], evidence.Envelopes[1]) || evidence.RelayRequests != 2 ||
		evidence.GatewayRequests != 2 || evidence.Resolved != 2 || evidence.Rejected != 0 ||
		evidence.Exclusions != expectedResolutionExclusions() || !verifyResolutionRoles(evidence) ||
		!verifyNamespaceMaterialization(evidence) || !verifyDeepNamespaceMeasurement(evidence) ||
		!equalStrings(trace.Fields, []string{"relay-opaque", "gateway-name-only", "contexts-unlinked"}) {
		return false
	}
	contexts := [][32]byte{{51}, {52}}
	for index, envelope := range evidence.Envelopes {
		target := [32]byte{1}
		if bytes.Contains(envelope, []byte("alice")) || bytes.Contains(envelope, target[:]) ||
			bytes.Contains(envelope, contexts[index][:]) {
			return false
		}
	}
	return true
}

func verifyResolutionRoles(evidence resolutionCellEvidence) bool {
	if len(evidence.Resolvers) != 2 || len(evidence.Gateways) != 2 || len(evidence.Relays) != 2 {
		return false
	}
	deadline := int64(1_800_000_015_000_000_000)
	for index := range evidence.Resolvers {
		resolver, gateway, relay := evidence.Resolvers[index], evidence.Gateways[index], evidence.Relays[index]
		isolation := [32]byte{byte(51 + index)}
		if resolver.Operation != "resolve" || resolver.Name != "alice" || resolver.Isolation != isolation ||
			resolver.Network != [32]byte{9} || resolver.Nonce == [32]byte{} || resolver.Target != [32]byte{1} ||
			resolver.Relay != [32]byte{1} || resolver.Gateway != [32]byte{2} || resolver.Rendezvous != [32]byte{3} ||
			resolver.Deadline != deadline || resolver.Result != "resolved" || resolver.Generation != 1 || resolver.Revision != 2 {
			return false
		}
		if gateway.Operation != "resolve" || gateway.Name != "alice" || gateway.Network != resolver.Network ||
			gateway.Nonce != resolver.Nonce || gateway.Target != resolver.Target || gateway.Deadline != deadline ||
			gateway.Result != "resolved" || gateway.Generation != 1 || gateway.Revision != 2 {
			return false
		}
		envelope := evidence.Envelopes[index]
		if relay.Origin == "" || !strings.HasPrefix(relay.Gateway, "https://") ||
			relay.Request != sha256.Sum256(envelope) || relay.Response == [32]byte{} ||
			relay.RequestBytes != uint64(len(envelope)) || relay.ResponseBytes == 0 || relay.KeyID != envelope[0] {
			return false
		}
	}
	return evidence.Resolvers[0].Nonce != evidence.Resolvers[1].Nonce &&
		evidence.Relays[0].Origin != evidence.Relays[1].Origin &&
		evidence.Relays[0].Gateway == evidence.Relays[1].Gateway
}

func expectedResolutionExclusions() [32]byte {
	out := []byte("ardents-stage-6-resolution-exclusions-v1\x00")
	for _, identity := range [][32]byte{{1}, {2}} {
		out = append(out, identity[:]...)
	}
	for _, family := range []string{"relay-family", "gateway-family"} {
		out = append(out, family...)
		out = append(out, 0)
	}
	rendezvous := [32]byte{3}
	return sha256.Sum256(append(out, rendezvous[:]...))
}
