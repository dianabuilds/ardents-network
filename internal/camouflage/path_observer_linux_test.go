//go:build linux

package camouflage

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type pathBoundary struct {
	Name    string `json:"name"`
	Source  string `json:"source,omitempty"`
	Address string `json:"address"`
	Port    uint16 `json:"port"`
}

type pathTarget struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
}

type pathManifest struct {
	Phase           string         `json:"phase"`
	Required        []pathBoundary `json:"required"`
	Forbidden       []pathBoundary `json:"forbidden"`
	AllowedExternal []pathTarget   `json:"allowed_external"`
	DynamicLoopback []string       `json:"dynamic_loopback"`
}

type pathResult struct {
	Phase              string                `json:"phase"`
	Counts             map[string]int64      `json:"counts"`
	UnexpectedExternal int64                 `json:"unexpected_external"`
	UnexpectedFlows    map[string]int64      `json:"unexpected_flows,omitempty"`
	DynamicBindings    map[string]pathTarget `json:"dynamic_bindings,omitempty"`
	Packets            int64                 `json:"packets"`
	Passed             bool                  `json:"passed"`
}

type pathObserver struct {
	manifest      pathManifest
	counts        map[string]int64
	active        bool
	collecting    bool
	completed     bool
	external      int64
	unexpected    map[string]int64
	bindings      map[string]pathTarget
	bindingCounts map[string]int64
	packets       int64
	doneAt        time.Time
	activated     time.Time
}

func (observer *pathObserver) poll(root string) error {
	manifestPath := filepath.Join(root, "path-manifest.json")
	if observer.completed {
		if !exists(manifestPath) {
			observer.completed = false
		}
		return nil
	}
	if !observer.active {
		raw, err := os.ReadFile(manifestPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("path observer cannot read manifest: %w", err)
		}
		if err := json.Unmarshal(raw, &observer.manifest); err != nil {
			return fmt.Errorf("path observer cannot decode manifest: %w", err)
		}
		if !validPathManifest(observer.manifest) {
			return fmt.Errorf("path observer manifest is invalid: %+v", observer.manifest)
		}
		observer.counts = make(map[string]int64)
		observer.unexpected = make(map[string]int64)
		observer.bindings = make(map[string]pathTarget)
		observer.bindingCounts = make(map[string]int64)
		observer.active = true
		observer.activated = time.Now()
		return nil
	}
	if !observer.collecting {
		if time.Since(observer.activated) < 250*time.Millisecond {
			return nil
		}
		observer.collecting = true
		return os.WriteFile(filepath.Join(root, "path-ready"), []byte("ready\n"), 0o666)
	}
	if !exists(filepath.Join(root, "path-done")) {
		return nil
	}
	if observer.doneAt.IsZero() {
		observer.doneAt = time.Now()
		return nil
	}
	if time.Since(observer.doneAt) < time.Second {
		return nil
	}
	result := pathResult{Phase: observer.manifest.Phase, Counts: observer.counts,
		UnexpectedExternal: observer.external,
		UnexpectedFlows:    observer.unexpected, DynamicBindings: observer.bindings,
		Packets: observer.packets, Passed: true}
	for _, boundary := range observer.manifest.Required {
		if observer.counts[boundary.Name] == 0 {
			result.Passed = false
		}
	}
	for _, boundary := range observer.manifest.Forbidden {
		if observer.counts[boundary.Name] != 0 {
			result.Passed = false
		}
	}
	for _, name := range observer.manifest.DynamicLoopback {
		if _, ok := observer.bindings[name]; !ok || observer.bindingCounts[name] == 0 {
			result.Passed = false
		}
	}
	if observer.external != 0 {
		result.Passed = false
	}
	raw, _ := json.Marshal(result)
	observer.active, observer.collecting, observer.completed = false, false, true
	observer.manifest, observer.counts = pathManifest{}, nil
	observer.unexpected = nil
	observer.bindings, observer.bindingCounts = nil, nil
	observer.external, observer.packets = 0, 0
	observer.doneAt = time.Time{}
	observer.activated = time.Time{}
	return os.WriteFile(filepath.Join(root, "path-result.json"), raw, 0o666)
}

func (observer *pathObserver) observe(packet []byte) {
	if !observer.active || !observer.collecting {
		return
	}
	sourceIP, destinationIP, addressesOK := packetAddresses(packet)
	if !addressesOK {
		return
	}
	offset, protocol, ambiguous, ok := packetTransport(packet)
	if ok && !ambiguous && protocol == 58 && ipv6LinkControl(sourceIP, destinationIP) {
		return
	}
	if !ok || ambiguous || protocol != 6 && protocol != 17 || len(packet) < offset+4 {
		observer.external++
		observer.recordUnexpected(protocol, sourceIP, destinationIP, 0, 0)
		return
	}
	observer.packets++
	sourcePort := binary.BigEndian.Uint16(packet[offset : offset+2])
	destinationPort := binary.BigEndian.Uint16(packet[offset+2 : offset+4])
	for _, boundary := range append(observer.manifest.Required, observer.manifest.Forbidden...) {
		if matchesBoundary(boundary, sourceIP, destinationIP, sourcePort, destinationPort) {
			observer.counts[boundary.Name]++
			return
		}
	}
	for name, target := range observer.bindings {
		if matchesTarget(target, sourceIP, destinationIP, sourcePort, destinationPort) {
			observer.bindingCounts[name]++
			return
		}
	}
	if protocol == 6 && sourceIP.IsLoopback() && destinationIP.IsLoopback() &&
		observer.bindDynamicLoopback(sourcePort, destinationPort) {
		return
	}
	for _, allowed := range observer.manifest.AllowedExternal {
		if matchesTarget(allowed, sourceIP, destinationIP, sourcePort, destinationPort) {
			return
		}
	}
	observer.external++
	observer.recordUnexpected(protocol, sourceIP, destinationIP, sourcePort, destinationPort)
}

func ipv6LinkControl(source, destination net.IP) bool {
	return destination.IsMulticast() && destination.To4() == nil &&
		(source.IsUnspecified() || source.IsLinkLocalUnicast())
}

func validPathManifest(manifest pathManifest) bool {
	if manifest.Phase == "" || len(manifest.Required) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, boundary := range append(manifest.Required, manifest.Forbidden...) {
		if boundary.Name == "" || boundary.Port == 0 || net.ParseIP(boundary.Address) == nil ||
			boundary.Source != "" && net.ParseIP(boundary.Source) == nil || seen[boundary.Name] {
			return false
		}
		seen[boundary.Name] = true
	}
	for _, target := range manifest.AllowedExternal {
		if target.Port == 0 || net.ParseIP(target.Address) == nil {
			return false
		}
	}
	for _, name := range manifest.DynamicLoopback {
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
	}
	return true
}

func (observer *pathObserver) bindDynamicLoopback(sourcePort, destinationPort uint16) bool {
	port := uint16(0)
	if loopbackTCPListener(destinationPort) {
		port = destinationPort
	} else if loopbackTCPListener(sourcePort) {
		port = sourcePort
	}
	if port == 0 {
		return false
	}
	for _, name := range observer.manifest.DynamicLoopback {
		if _, exists := observer.bindings[name]; !exists {
			observer.bindings[name] = pathTarget{Address: "127.0.0.1", Port: port}
			observer.bindingCounts[name] = 1
			return true
		}
	}
	return false
}

func loopbackTCPListener(port uint16) bool {
	input, err := os.Open("/proc/net/tcp")
	if err != nil {
		return false
	}
	defer input.Close()
	wanted := strings.ToUpper(strconv.FormatUint(uint64(port), 16))
	wanted = strings.Repeat("0", 4-len(wanted)) + wanted
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 4 && fields[1] == "0100007F:"+wanted && fields[3] == "0A" {
			return true
		}
	}
	return false
}

func packetAddresses(packet []byte) (net.IP, net.IP, bool) {
	offset := 14
	if len(packet) < offset {
		return nil, nil, false
	}
	if binary.BigEndian.Uint16(packet[12:14]) == 0x8100 {
		offset = 18
	}
	if len(packet) < offset+20 {
		return nil, nil, false
	}
	switch packet[offset] >> 4 {
	case 4:
		return net.IP(packet[offset+12 : offset+16]), net.IP(packet[offset+16 : offset+20]), true
	case 6:
		if len(packet) < offset+40 {
			return nil, nil, false
		}
		return net.IP(packet[offset+8 : offset+24]), net.IP(packet[offset+24 : offset+40]), true
	default:
		return nil, nil, false
	}
}

func matchesBoundary(boundary pathBoundary, source, destination net.IP, sourcePort, destinationPort uint16) bool {
	target := net.ParseIP(boundary.Address)
	origin := net.ParseIP(boundary.Source)
	forward := destination.Equal(target) && destinationPort == boundary.Port &&
		(origin == nil || source.Equal(origin))
	reverse := source.Equal(target) && sourcePort == boundary.Port &&
		(origin == nil || destination.Equal(origin))
	return forward || reverse
}

func matchesTarget(target pathTarget, source, destination net.IP, sourcePort, destinationPort uint16) bool {
	address := net.ParseIP(target.Address)
	return destination.Equal(address) && destinationPort == target.Port ||
		source.Equal(address) && sourcePort == target.Port
}

func (observer *pathObserver) recordUnexpected(protocol byte, source, destination net.IP,
	sourcePort, destinationPort uint16) {
	if len(observer.unexpected) >= 16 {
		return
	}
	key := fmt.Sprintf("%d:%s:%d>%s:%d", protocol, source, sourcePort, destination, destinationPort)
	observer.unexpected[key]++
}
