//go:build live

package network_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

var finalObserverBindings = []struct {
	boundary string
	role     string
	flow     string
}{
	{"endpoint-adapter", "endpoint", "E-to-B-front"},
	{"tls-front", "bridge", "E-to-B-front"},
	{"webtunnel-server", "bridge", "dynamic"},
	{"bridge-next-leg", "bridge", "B-to-Initiator"},
	{"publisher-endpoint", "publisher", "Responder-to-Publisher"},
	{"ordinary-initiator", "initiator", "Initiator-to-Introduction"},
	{"ordinary-introduction", "introduction", "Introduction-to-Rendezvous"},
	{"ordinary-rendezvous", "rendezvous", "Rendezvous-to-Responder"},
	{"ordinary-responder", "responder", "Responder-to-Publisher"},
}

type finalPathObservation struct {
	Phase              string                 `json:"phase"`
	Counts             map[string]int64       `json:"counts"`
	UnexpectedExternal int64                  `json:"unexpected_external"`
	UnexpectedFlows    map[string]int64       `json:"unexpected_flows,omitempty"`
	DynamicBindings    map[string]finalTarget `json:"dynamic_bindings,omitempty"`
	Packets            int64                  `json:"packets"`
	Passed             bool                   `json:"passed"`
}

type finalTarget struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
}

type finalDNSObservation struct {
	Packets          int64                      `json:"Packets"`
	Controls         int64                      `json:"Controls"`
	Ambiguous        int64                      `json:"Ambiguous"`
	IPv4UDPControls  int64                      `json:"IPv4UDPControls"`
	IPv6UDPControls  int64                      `json:"IPv6UDPControls"`
	IPv4TCPControls  int64                      `json:"IPv4TCPControls"`
	BoundaryControls map[string]finalDNSControl `json:"BoundaryControls"`
}

type finalDNSControl struct {
	IPv4UDP, IPv6UDP, IPv4TCP int64
	IfIndex                   int
	Token                     string
}

func collectFinalWorkerObservers(identity string, roots []string) ([]finalRunnerObserver, []finalRawObserverSet) {
	if len(roots) == 0 {
		return nil, nil
	}
	var aggregate []finalRunnerObserver
	rawSets := make([]finalRawObserverSet, 0, len(roots))
	for _, root := range roots {
		observed, raw := collectFinalRootObservers(identity, root)
		if observed == nil {
			return nil, nil
		}
		if aggregate == nil {
			aggregate = observed
		} else if !reflect.DeepEqual(aggregate, observed) {
			return nil, nil
		}
		rawSets = append(rawSets, finalRawObserverSet{Observers: raw})
	}
	return aggregate, rawSets
}

func collectFinalRootObservers(identity, root string) ([]finalRunnerObserver, []finalRawObserver) {
	if root == "" {
		return nil, nil
	}
	result := make([]finalRunnerObserver, 0, len(finalObserverBindings))
	raw := make([]finalRawObserver, 0, len(finalObserverBindings))
	for _, binding := range finalObserverBindings {
		if binding.boundary == "endpoint-adapter" {
			endpointRaw, ok := collectFinalEndpointObserver(identity, root)
			if !ok {
				return nil, nil
			}
			result = append(result, cleanFinalObserver(binding.boundary))
			raw = append(raw, endpointRaw...)
			continue
		}
		role := binding.role
		path, dns, ok := readFinalRoleObservation(root, role)
		if !validFinalRoleObservation(identity, path, dns, binding.role, binding.flow, ok) {
			return nil, nil
		}
		result = append(result, cleanFinalObserver(binding.boundary))
		raw = append(raw, finalRawObserver{Boundary: binding.boundary, Role: role, Path: path, DNS: dns})
	}
	return result, raw
}

func collectFinalEndpointObserver(identity, root string) ([]finalRawObserver, bool) {
	roles := []string{"endpoint"}
	if !finalObservationExists(root, "endpoint") {
		entries, err := os.ReadDir(filepath.Join(root, "sync"))
		if err != nil {
			return nil, false
		}
		roles = roles[:0]
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "capacity-") && finalObservationExists(root, entry.Name()) {
				roles = append(roles, entry.Name())
			}
		}
		sort.Strings(roles)
	}
	expected := 1
	if strings.HasPrefix(identity, "capacity/h3-s5-b1-v1-strong/") {
		expected = 16
	} else if strings.HasPrefix(identity, "capacity/") || strings.HasPrefix(identity, "pressure/P0") ||
		strings.HasPrefix(identity, "pressure/P1") || strings.HasPrefix(identity, "pressure/P4") {
		expected = 4
	}
	if len(roles) != expected {
		return nil, false
	}
	retained := make([]finalRawObserver, 0, len(roles))
	for index, role := range roles {
		if expected > 1 && role != fmt.Sprintf("capacity-%02d", index) {
			return nil, false
		}
		path, dns, ok := readFinalRoleObservation(root, role)
		if !validFinalRoleObservation(identity, path, dns, "endpoint", "E-to-B-front", ok) {
			return nil, false
		}
		retained = append(retained, finalRawObserver{Boundary: "endpoint-adapter", Role: role, Path: path, DNS: dns})
	}
	return retained, true
}

func validFinalRoleObservation(identity string, path finalPathObservation, dns finalDNSObservation,
	role, flow string, decoded bool,
) bool {
	controlNames := finalObservedControlNames(path, flow)
	return decoded && path.Phase == "s5.3-"+role && path.Passed && len(controlNames) > 0 &&
		path.UnexpectedExternal == 0 && len(path.UnexpectedFlows) == 0 &&
		(path.Packets > 0 || strings.HasPrefix(identity, "hostile/G1-invite/")) &&
		validFinalDNSControls(dns, controlNames) && dns.Controls >= 6 &&
		dns.IPv4UDPControls >= 2 && dns.IPv6UDPControls >= 2 &&
		dns.IPv4TCPControls >= 2 && dns.Packets == 0 && dns.Ambiguous == 0
}

func finalObservedControlNames(path finalPathObservation, flow string) []string {
	if strings.HasPrefix(path.Phase, "s5.3-") && path.Packets == 0 {
		if flow == "dynamic" {
			return []string{"front-to-WebTunnel-server"}
		}
		return []string{flow}
	}
	if flow == "dynamic" {
		if _, ok := path.DynamicBindings["front-to-WebTunnel-server"]; ok {
			return []string{"front-to-WebTunnel-server"}
		}
		return nil
	}
	if path.Counts[flow] > 0 {
		return []string{flow}
	}
	if flow != "E-to-B-front" {
		return nil
	}
	var result []string
	for name, count := range path.Counts {
		if strings.HasPrefix(name, "E-to-B-front-") && count > 0 {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

func validFinalDNSControls(value finalDNSObservation, names []string) bool {
	for _, name := range names {
		control, ok := value.BoundaryControls[name]
		if !ok || control.IfIndex <= 0 || len(control.Token) != 32 || control.IPv4UDP < 2 ||
			control.IPv6UDP < 2 || control.IPv4TCP < 2 {
			return false
		}
	}
	return true
}

func cleanFinalObserver(boundary string) finalRunnerObserver {
	return finalRunnerObserver{Boundary: boundary, IPv4UDPControl: true, IPv6UDPControl: true,
		IPv4TCPControl: true, Attribution: "exact", ForbiddenOwner: "none", ObservationCompleted: true}
}

func finalObservationExists(root, role string) bool {
	_, pathErr := os.Lstat(filepath.Join(root, "sync", role, "path-result.json"))
	_, dnsErr := os.Lstat(filepath.Join(root, "sync", role, "result.json"))
	return pathErr == nil && dnsErr == nil
}

func readFinalRoleObservation(root, role string) (finalPathObservation, finalDNSObservation, bool) {
	var path finalPathObservation
	var dns finalDNSObservation
	pathRaw, pathErr := readFinalEvidenceFile(filepath.Join(root, "sync", role, "path-result.json"))
	dnsRaw, dnsErr := readFinalEvidenceFile(filepath.Join(root, "sync", role, "result.json"))
	if pathErr != nil || dnsErr != nil || decodeFinalEvidence(pathRaw, &path) != nil ||
		decodeFinalEvidence(dnsRaw, &dns) != nil {
		return finalPathObservation{}, finalDNSObservation{}, false
	}
	return path, dns, true
}

func readFinalEvidenceFile(path string) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 1 || before.Size() > 64<<10 {
		return nil, errors.New("final observer evidence is not a bounded regular file")
	}
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(input, (64<<10)+1))
	after, statErr := input.Stat()
	closeErr := input.Close()
	if readErr != nil || statErr != nil || closeErr != nil || len(raw) > 64<<10 ||
		!os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return nil, errors.Join(readErr, statErr, closeErr, errors.New("final observer evidence changed while read"))
	}
	return raw, nil
}

func decodeFinalEvidence(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("final observer evidence is not one strict JSON object")
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(raw, canonical) {
		return errors.New("final observer evidence is not canonical")
	}
	return nil
}

func releaseFinalWorkerRoot(workerRoot string) error {
	entries, err := os.ReadDir(workerRoot)
	if err != nil || len(entries) != 0 {
		return errors.Join(err, errors.New("final worker retained files in its parent-owned root"))
	}
	if err := os.Remove(workerRoot); err != nil {
		return err
	}
	return os.Remove(filepath.Dir(workerRoot))
}

func completeFinalWorkerEvidence(values []finalWorkerResult,
	startedOffset, terminalOffset, cleanupOffset uint64,
) {
	for index := range values {
		value := &values[index]
		expectedSets := uint16(1)
		if value.CellID == "pressure/P4" {
			expectedSets = 10
		}
		if value.ObserverSets != expectedSets {
			continue
		}
		value.StartedOffsetMillis = startedOffset
		value.TerminalOffsetMillis = terminalOffset
		value.CleanupOffsetMillis = cleanupOffset
		if len(value.Observers) != len(finalObserverBindings) || cleanupOffset < value.TerminalOffsetMillis {
			continue
		}
		value.Residuals = verifiedFinalWorkerResiduals()
		value.EvidenceComplete = true
	}
}
