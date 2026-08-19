package blockedverify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

type finalRawObserverSet struct {
	Observers []finalRawObserver `json:"observers"`
}

type finalRawObserver struct {
	Boundary string                  `json:"boundary"`
	Role     string                  `json:"role"`
	Path     finalRawPathObservation `json:"path"`
	DNS      finalRawDNSObservation  `json:"dns"`
}

type finalRawPathObservation struct {
	Phase              string                    `json:"phase"`
	Counts             map[string]int64          `json:"counts"`
	UnexpectedExternal int64                     `json:"unexpected_external"`
	UnexpectedFlows    map[string]int64          `json:"unexpected_flows,omitempty"`
	DynamicBindings    map[string]finalRawTarget `json:"dynamic_bindings,omitempty"`
	Packets            int64                     `json:"packets"`
	Passed             bool                      `json:"passed"`
}

type finalRawTarget struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
}

type finalRawDNSObservation struct {
	Packets          int64                         `json:"Packets"`
	Controls         int64                         `json:"Controls"`
	Ambiguous        int64                         `json:"Ambiguous"`
	IPv4UDPControls  int64                         `json:"IPv4UDPControls"`
	IPv6UDPControls  int64                         `json:"IPv6UDPControls"`
	IPv4TCPControls  int64                         `json:"IPv4TCPControls"`
	BoundaryControls map[string]finalRawDNSControl `json:"BoundaryControls"`
}

type finalRawDNSControl struct {
	IPv4UDP int64  `json:"IPv4UDP"`
	IPv6UDP int64  `json:"IPv6UDP"`
	IPv4TCP int64  `json:"IPv4TCP"`
	IfIndex int    `json:"IfIndex"`
	Token   string `json:"Token"`
}

type finalRawObserverEvidence struct {
	Schema   string                `json:"schema"`
	CellID   string                `json:"cell_id"`
	Sets     []finalRawObserverSet `json:"sets"`
	Exercise *finalFaultExercise   `json:"exercise,omitempty"`
}

type finalFaultExercise struct {
	Schema         string `json:"schema"`
	CellID         string `json:"cell_id"`
	Group          string `json:"group"`
	Variant        string `json:"variant"`
	Episode        string `json:"episode"`
	InjectionClass string `json:"injection_class"`
	Subject        string `json:"subject"`
	BeforeSHA256   string `json:"before_sha256"`
	AfterSHA256    string `json:"after_sha256"`
	Relation       string `json:"relation"`
	ReceiptSHA256  string `json:"receipt_sha256"`
	Receipt        []byte `json:"receipt"`
	ActorSHA256    string `json:"actor_sha256"`
	SeedSHA256     string `json:"seed_sha256"`
	OffsetMillis   uint64 `json:"offset_millis"`
	External       bool   `json:"external"`
}

var finalObserverBoundaries = []string{
	"endpoint-adapter", "tls-front", "webtunnel-server", "bridge-next-leg", "publisher-endpoint",
	"ordinary-initiator", "ordinary-introduction", "ordinary-rendezvous", "ordinary-responder",
}

func verifyFinalObserverEvidence(root string, cells []finalCellObservation) []string {
	for _, cell := range cells {
		expected := finalObserverEvidencePath(cell.ID)
		artifact := cell.ObserverEvidence
		path, safe := safeArtifactPath(root, expected)
		if !safe || artifact.Path != expected || artifact.Bytes < 1 || !isHexDigest(artifact.SHA256, 32) {
			return []string{"final cell raw observer evidence commitment is invalid"}
		}
		hash, size, err := hashFile(path)
		if err != nil || hash != artifact.SHA256 || size != artifact.Bytes {
			return []string{"final cell raw observer evidence commitment mismatch"}
		}
		var raw finalRawObserverEvidence
		input, err := readStableFile(path)
		if err != nil || decodeCanonicalSnapshot(input, &raw) != nil ||
			raw.Schema != "ardents-h3-final-raw-observers-v1" || raw.CellID != cell.ID ||
			!validFinalRawObserverEvidence(cell.ID, raw.Sets) || !validFinalFaultExercise(cell.ID, cell.Seed, raw.Exercise) {
			return []string{"final cell raw observer evidence is incomplete or invalid"}
		}
	}
	return nil
}

func validFinalFaultExercise(cell, seed string, value *finalFaultExercise) bool {
	if !strings.HasPrefix(cell, "hostile/") {
		return value == nil
	}
	parts := strings.Split(cell, "/")
	return value != nil && len(parts) == 4 && value.Schema == "ardents-h3-final-fault-exercise-v4" &&
		value.CellID == cell && value.Group == parts[1] && value.Variant == parts[2] &&
		value.Episode == parts[3] && value.Subject == parts[1]+"/"+parts[2] && value.External &&
		value.OffsetMillis > 0 && validFinalFaultRelation(value) &&
		isHexDigest(value.BeforeSHA256, 32) && isHexDigest(value.AfterSHA256, 32) &&
		validFinalFaultReceipt(value, parts[1], parts[2]) && isHexDigest(value.ActorSHA256, 32) &&
		value.SeedSHA256 == finalFaultSeedDigest(seed) &&
		finalFaultClass(parts[1]) == value.InjectionClass
}

func finalFaultSeedDigest(seed string) string {
	raw, err := hex.DecodeString(seed)
	if err != nil || len(raw) != 32 {
		return ""
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func validFinalFaultRelation(value *finalFaultExercise) bool {
	if finalFaultRequiresStableState(value.Group, value.Variant) {
		return value.Relation == "same" && value.BeforeSHA256 == value.AfterSHA256
	}
	return value.Relation == "same" && value.BeforeSHA256 == value.AfterSHA256 ||
		value.Relation == "different" && value.BeforeSHA256 != value.AfterSHA256
}

func finalFaultRequiresStableState(group, variant string) bool {
	return group == "G9-ledger-leakage" && variantIn(variant, "unknown-invite-field", "regime-oscillation", "slot1-before-slot0",
		"retry-before-initial", "duplicate-ordinal", "ledger-reset-restart", "ledger-reset-new-operation")
}

func finalFaultClass(group string) string {
	return map[string]string{"G1-invite": "input-mutation", "G2-domain-collision": "state-mutation",
		"G3-replay-replacement": "state-mutation", "G4-restart": "lifecycle-fault",
		"G5-adapter-fault": "adapter-fault", "G6-substitution": "binding-substitution",
		"G7-forbidden-path": "forbidden-path", "G8-lifecycle": "lifecycle-fault",
		"G9-ledger-leakage": "ledger-mutation"}[group]
}

func finalObserverEvidencePath(cell string) string {
	digest := sha256.Sum256([]byte(cell))
	return filepath.ToSlash(filepath.Join("final-observers", hex.EncodeToString(digest[:])+".json"))
}

func validFinalRawObserverEvidence(cell string, sets []finalRawObserverSet) bool {
	wantSets := 1
	if cell == "pressure/P4" {
		wantSets = 10
	}
	if len(sets) != wantSets {
		return false
	}
	for _, set := range sets {
		seen := make(map[string]bool, len(finalObserverBoundaries))
		for _, observed := range set.Observers {
			if observed.Boundary == "" || observed.Role == "" || observed.Path.Phase == "" ||
				!observed.Path.Passed || observed.Path.UnexpectedExternal != 0 ||
				len(observed.Path.UnexpectedFlows) != 0 || observed.DNS.Packets != 0 ||
				observed.DNS.Ambiguous != 0 || !validFinalRawControls(observed.DNS) {
				return false
			}
			seen[observed.Boundary] = true
			seen[observed.Boundary+"/"+observed.Role] = true
		}
		for _, boundary := range finalObserverBoundaries {
			if !seen[boundary] {
				return false
			}
		}
		for index := range finalObserverEndpointCount(cell) {
			role := "endpoint"
			if finalObserverEndpointCount(cell) > 1 {
				role = fmt.Sprintf("capacity-%02d", index)
			}
			if !seen["endpoint-adapter/"+role] {
				return false
			}
		}
	}
	return true
}

func finalObserverEndpointCount(cell string) int {
	if strings.HasPrefix(cell, "capacity/h3-s5-b1-v1-strong/") {
		return 16
	}
	if strings.HasPrefix(cell, "capacity/") || strings.HasPrefix(cell, "pressure/P0") ||
		strings.HasPrefix(cell, "pressure/P1") || strings.HasPrefix(cell, "pressure/P4") {
		return 4
	}
	return 1
}

func validFinalRawControls(value finalRawDNSObservation) bool {
	if value.Controls < 6 || value.IPv4UDPControls < 2 || value.IPv6UDPControls < 2 || value.IPv4TCPControls < 2 {
		return false
	}
	for _, control := range value.BoundaryControls {
		if control.IfIndex > 0 && len(control.Token) == 32 && control.IPv4UDP >= 2 &&
			control.IPv6UDP >= 2 && control.IPv4TCP >= 2 {
			return true
		}
	}
	return false
}
