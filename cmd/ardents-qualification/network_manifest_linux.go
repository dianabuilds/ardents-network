//go:build linux

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
)

const (
	networkManifestVersion = 1
	userEndpoint           = "user"
	publisherEndpoint      = "publisher"
)

type qualificationNetworkManifest struct {
	Version      int
	Cell         string
	Carrier      string
	Generator    networkGenerator
	SeedSchedule []string
	Paths        []networkPath
	Relays       []networkRelay
	Failures     []networkFailure
}

type networkGenerator struct {
	Algorithm string
	Version   string
}

type networkPath struct {
	Name        string
	CapBitsPerS uint64
	Segments    []networkSegment
}

type networkSegment struct {
	ID, From, To                 string
	DelayMicros, JitterP95Micros uint64
	LossPartsPerMillion          uint64
	ReorderPartsPerMillion       uint64
}

type networkFailure struct {
	Episode, SegmentID string
	AtMillis           uint64
	DurationMillis     uint64
}

type networkRelay struct {
	ID, Host, Container, Network, Listen, PublicEndpoint, Target string
	Mode                                                         string
	UpstreamSegment, ClientSegment                               string
	UpstreamRate, ClientRate                                     uint64
}

type networkManifestEvidence struct {
	Kind, SHA256, Cell, Carrier      string
	Version, Paths, Segments, Relays int
	Seeds, Failures                  int
}

func verifyNetworkManifest(arguments []string, output io.Writer) error {
	if len(arguments) != 1 {
		return errors.New("usage: ardents-qualification verify-network-manifest <manifest.json>")
	}
	body, err := os.ReadFile(arguments[0])
	if err != nil {
		return err
	}
	if len(body) == 0 || len(body) > 256<<10 {
		return errors.New("network manifest size is invalid")
	}
	var manifest qualificationNetworkManifest
	if err := decodeExact(body, &manifest); err != nil {
		return err
	}
	segments, err := validateNetworkManifest(manifest)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	return json.NewEncoder(output).Encode(networkManifestEvidence{
		Kind: "network-manifest", SHA256: hex.EncodeToString(sum[:]), Cell: manifest.Cell,
		Carrier: manifest.Carrier, Version: manifest.Version, Paths: len(manifest.Paths),
		Segments: segments, Relays: len(manifest.Relays), Seeds: len(manifest.SeedSchedule), Failures: len(manifest.Failures),
	})
}

func validateNetworkManifest(manifest qualificationNetworkManifest) (int, error) {
	if manifest.Version != networkManifestVersion || manifest.Generator.Algorithm != "linux-netem" || manifest.Generator.Version == "" {
		return 0, errors.New("network manifest generator identity is invalid")
	}
	if manifest.Carrier != "tcp-tls" && manifest.Carrier != "quic" {
		return 0, errors.New("network manifest carrier is invalid")
	}
	if len(manifest.SeedSchedule) == 0 {
		return 0, errors.New("network manifest seed schedule is empty")
	}
	seenSeeds := make(map[string]bool, len(manifest.SeedSchedule))
	for _, seed := range manifest.SeedSchedule {
		if _, err := decodeIdentity(seed); err != nil || seenSeeds[seed] {
			return 0, errors.New("network manifest seed schedule is invalid")
		}
		seenSeeds[seed] = true
	}
	if len(manifest.Paths) != 2 {
		return 0, errors.New("network manifest must contain two directional paths")
	}
	segments := make(map[string]bool, 12)
	paths := make(map[string]bool, 2)
	for _, path := range manifest.Paths {
		if paths[path.Name] {
			return 0, errors.New("network manifest directional path is duplicated")
		}
		paths[path.Name] = true
		if err := validateNetworkPath(manifest.Cell, path, segments); err != nil {
			return 0, err
		}
	}
	if len(segments) != 12 {
		return 0, errors.New("network manifest must contain twelve directions over six physical links")
	}
	if _, err := manifestNodeIDs(manifest); err != nil {
		return 0, err
	}
	if err := validateNetworkRelays(manifest, segments); err != nil {
		return 0, err
	}
	if err := validateNetworkFailures(manifest.Cell, manifest.Failures, segments); err != nil {
		return 0, err
	}
	return len(segments), nil
}

func validateNetworkRelays(manifest qualificationNetworkManifest, segments map[string]bool) error {
	if len(manifest.Relays) != 6 {
		return errors.New("network manifest must bind six isolated relays")
	}
	details := make(map[string]networkSegment, 12)
	pathName := make(map[string]string, 12)
	for _, path := range manifest.Paths {
		for _, segment := range path.Segments {
			details[segment.ID], pathName[segment.ID] = segment, path.Name
		}
	}
	mapped := make(map[string]bool, 12)
	identities := make(map[string]bool, 18)
	wantNetwork := "tcp"
	if manifest.Carrier == "quic" {
		wantNetwork = "udp"
	}
	for index, relay := range manifest.Relays {
		if identities["id:"+relay.ID] || identities["container:"+relay.Container] || identities["listen:"+relay.Host+":"+relay.Listen] {
			return errors.New("network manifest relay identity, container or listener is duplicated")
		}
		identities["id:"+relay.ID], identities["container:"+relay.Container], identities["listen:"+relay.Host+":"+relay.Listen] = true, true, true
		if !safeRelayName(relay.ID) || !safeRelayName(relay.Container) || relay.Host != "reader" && relay.Host != "publisher" || relay.Network != wantNetwork ||
			!validManifestEndpoint(relay.Listen, true) || !validManifestEndpoint(relay.PublicEndpoint, false) || !validManifestEndpoint(relay.Target, false) {
			return errors.New("network manifest relay identity or endpoint is invalid")
		}
		_, listenPort, _ := net.SplitHostPort(relay.Listen)
		_, publicPort, _ := net.SplitHostPort(relay.PublicEndpoint)
		if listenPort != publicPort {
			return errors.New("network manifest relay public endpoint differs from its listener")
		}
		upstream, upstreamOK := details[relay.UpstreamSegment]
		client, clientOK := details[relay.ClientSegment]
		if !upstreamOK || !clientOK || mapped[relay.UpstreamSegment] || mapped[relay.ClientSegment] ||
			pathName[relay.UpstreamSegment] != "user-to-publisher" || pathName[relay.ClientSegment] != "publisher-to-user" ||
			upstream.From != client.To || upstream.To != client.From ||
			upstream.DelayMicros != client.DelayMicros || upstream.JitterP95Micros != client.JitterP95Micros || upstream.LossPartsPerMillion != client.LossPartsPerMillion {
			return errors.New("network manifest relay directional segment binding is invalid")
		}
		mapped[relay.UpstreamSegment], mapped[relay.ClientSegment] = true, true
		wantUpstream, wantClient := uint64(100_000_000), uint64(100_000_000)
		if index == 0 {
			wantUpstream = 20_000_000
		}
		if relay.UpstreamRate != wantUpstream || relay.ClientRate != wantClient {
			return errors.New("network manifest relay directional rate is invalid")
		}
		prefix := "net14ad-"
		if manifest.Cell == "net14s" {
			prefix = "net14s-"
		}
		wantMode := prefix + "delay"
		if upstream.LossPartsPerMillion != 0 {
			wantMode = prefix + "loss"
		}
		if relay.Mode != wantMode {
			return errors.New("network manifest relay mode differs from its segments")
		}
	}
	if len(mapped) != len(segments) {
		return errors.New("network manifest does not map every directional segment")
	}
	return nil
}

func safeRelayName(value string) bool {
	if len(value) == 0 || len(value) > 63 {
		return false
	}
	for _, character := range value {
		letter := character >= 'a' && character <= 'z'
		digit := character >= '0' && character <= '9'
		if !letter && !digit && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func validManifestEndpoint(value string, allowEmptyHost bool) bool {
	host, portText, err := net.SplitHostPort(value)
	port, portErr := strconv.ParseUint(portText, 10, 16)
	if err != nil || portErr != nil || port == 0 {
		return false
	}
	if allowEmptyHost {
		return host == ""
	}
	address := net.ParseIP(host)
	return address != nil && address.To4() != nil
}

func validateNetworkPath(cell string, path networkPath, seen map[string]bool) error {
	wantDelay, wantLoss, maxJitter := uint64(40_000), uint64(1_000), uint64(10_000)
	if cell == "net14s" {
		wantDelay, wantLoss, maxJitter = 150_000, 50_000, 100_000
	} else if cell != "net14ad" && cell != "net14-recovery" {
		return errors.New("network manifest cell is invalid")
	}
	from, to, capBits := userEndpoint, publisherEndpoint, uint64(20_000_000)
	if path.Name == "publisher-to-user" {
		from, to, capBits = publisherEndpoint, userEndpoint, 100_000_000
	} else if path.Name != "user-to-publisher" {
		return errors.New("network manifest path name is invalid")
	}
	if path.CapBitsPerS != capBits || len(path.Segments) != 6 {
		return fmt.Errorf("network path %s cap or segment count is invalid", path.Name)
	}
	delay, jitter, lossInjectors := uint64(0), uint64(0), 0
	current := from
	for _, segment := range path.Segments {
		if segment.ID == "" || seen[segment.ID] || segment.From != current || segment.To == "" || segment.ReorderPartsPerMillion != 0 {
			return fmt.Errorf("network path %s segment chain is invalid", path.Name)
		}
		if segment.DelayMicros > wantDelay || segment.JitterP95Micros > maxJitter || segment.LossPartsPerMillion > wantLoss {
			return fmt.Errorf("network path %s segment impairment exceeds its bound", path.Name)
		}
		seen[segment.ID] = true
		current = segment.To
		delay += segment.DelayMicros
		jitter += segment.JitterP95Micros
		if segment.LossPartsPerMillion != 0 {
			lossInjectors++
		}
	}
	if current != to {
		return fmt.Errorf("network path %s endpoint is invalid", path.Name)
	}
	loss := uint64(0)
	for _, segment := range path.Segments {
		loss += segment.LossPartsPerMillion
	}
	if delay != wantDelay || jitter > maxJitter || loss != wantLoss || lossInjectors != 1 {
		return fmt.Errorf("network path %s end-to-end impairment is invalid", path.Name)
	}
	return nil
}

func validateNetworkFailures(cell string, failures []networkFailure, segments map[string]bool) error {
	if cell != "net14-recovery" {
		if len(failures) != 0 {
			return errors.New("non-recovery manifest cannot schedule failures")
		}
		return nil
	}
	if len(failures) < 3 {
		return errors.New("recovery manifest needs at least three scheduled failures")
	}
	last := uint64(0)
	seenEpisodes := make(map[string]bool, len(failures))
	for _, failure := range failures {
		if !safeRelayName(failure.Episode) || seenEpisodes[failure.Episode] || !segments[failure.SegmentID] ||
			failure.AtMillis < 15_000 || failure.AtMillis <= last || failure.AtMillis >= 600_000 || failure.DurationMillis == 0 || failure.DurationMillis > 600_000-failure.AtMillis {
			return errors.New("network recovery failure schedule is invalid")
		}
		seenEpisodes[failure.Episode] = true
		last = failure.AtMillis
	}
	return nil
}
