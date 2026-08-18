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
	Schema string                `json:"schema"`
	CellID string                `json:"cell_id"`
	Sets   []finalRawObserverSet `json:"sets"`
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
			!validFinalRawObserverEvidence(cell.ID, raw.Sets) {
			return []string{"final cell raw observer evidence is incomplete or invalid"}
		}
	}
	return nil
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
