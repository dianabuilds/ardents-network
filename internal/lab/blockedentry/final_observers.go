package blockedentry

import (
	"errors"
	"fmt"
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

func admitFinalObserverEvidence(secretRoot string, output cellObservation) (artifactCommitment, error) {
	var raw finalRawObserverEvidence
	if err := readFinalHandoffArtifact(secretRoot, "final-observers", output.CellID,
		output.ObserverEvidence, &raw); err != nil || raw.Schema != "ardents-h3-final-raw-observers-v1" ||
		raw.CellID != output.CellID || !validFinalRawObserverEvidence(output.CellID, raw.Sets) {
		return artifactCommitment{}, errors.New("final cell raw observer evidence is incomplete")
	}
	return output.ObserverEvidence, nil
}

func validFinalRawObserverEvidence(cellID string, sets []finalRawObserverSet) bool {
	wantSets := 1
	if cellID == "pressure/P4" {
		wantSets = 10
	}
	if len(sets) != wantSets {
		return false
	}
	for _, set := range sets {
		if len(set.Observers) < len(boundaries) {
			return false
		}
		seen := make(map[string]bool, len(set.Observers))
		for _, observed := range set.Observers {
			if observed.Boundary == "" || observed.Role == "" || seen[observed.Boundary+"/"+observed.Role] ||
				observed.Path.Phase == "" || !observed.Path.Passed || observed.Path.UnexpectedExternal != 0 ||
				len(observed.Path.UnexpectedFlows) != 0 || observed.DNS.Packets != 0 || observed.DNS.Ambiguous != 0 {
				return false
			}
			seen[observed.Boundary+"/"+observed.Role] = true
		}
		for index := range finalObserverEndpointCount(cellID) {
			role := "endpoint"
			if finalObserverEndpointCount(cellID) > 1 {
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
