//go:build linux

package main

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

type relaySegmentTraffic struct {
	ID, From, To string
	Bytes        uint64
	Samples      []relaySegmentSample
}

type relaySegmentSample struct {
	At    time.Time
	Bytes uint64
}

type relayCounterSample struct {
	At                         time.Time
	UpstreamBytes, ClientBytes uint64
}
type relayNodeTraffic struct {
	ID     string
	Tx, Rx uint64
}

type relayTrafficVerdict struct {
	ManifestSHA256, BinarySHA256 string
	Segments                     []relaySegmentTraffic
	Nodes                        []relayNodeTraffic
	Endpoints                    []relayNodeTraffic
}

type relayResultInput struct {
	ID, Host, Container, UpstreamSegment, ClientSegment, BinarySHA256 string
	UpstreamBytes, ClientBytes                                        uint64
	TrafficControl                                                    []string
	Samples                                                           []relayCounterSample
}

type trafficControlRecord struct {
	Kind, Handle, Parent string
	Bytes                uint64
	Stats                struct{ Bytes uint64 }
}

func readRelayResults(path string, manifest qualificationNetworkManifest, manifestHash string) (relayTrafficVerdict, []streamqualification.Criterion, error) {
	body, err := os.ReadFile(path)
	if err != nil || len(body) == 0 || len(body) > 4<<20 {
		return relayTrafficVerdict{}, nil, errors.Join(err, errors.New("relay result input is invalid"))
	}
	var inputs []relayResultInput
	if err := decodeExact(body, &inputs); err != nil {
		return relayTrafficVerdict{}, nil, err
	}
	criteria := make([]streamqualification.Criterion, 0, 3)
	add := func(name string, observed float64, relation string, bound float64, passed bool) {
		criteria = append(criteria, streamqualification.Criterion{Name: name, Observed: observed, Relation: relation, Bound: bound, Passed: passed})
	}
	binaryIdentity := ""
	byID := make(map[string]relayResultInput, len(inputs))
	for _, input := range inputs {
		if input.ID == "" || byID[input.ID].ID != "" {
			return relayTrafficVerdict{}, criteria, errors.New("relay result identity is duplicated or absent")
		}
		if _, digestErr := decodeIdentity(input.BinarySHA256); digestErr != nil || binaryIdentity != "" && binaryIdentity != input.BinarySHA256 {
			return relayTrafficVerdict{}, criteria, errors.New("relay candidate identity is invalid or inconsistent")
		}
		binaryIdentity = input.BinarySHA256
		byID[input.ID] = input
	}
	segmentDetails := make(map[string]networkSegment, 12)
	for _, path := range manifest.Paths {
		for _, segment := range path.Segments {
			segmentDetails[segment.ID] = segment
		}
	}
	verdict := relayTrafficVerdict{ManifestSHA256: manifestHash, BinarySHA256: binaryIdentity}
	nodes := make(map[string]*relayNodeTraffic)
	clean := len(inputs) == len(manifest.Relays)
	for _, relay := range manifest.Relays {
		input, present := byID[relay.ID]
		if !present || input.Host != relay.Host || input.Container != relay.Container || input.UpstreamSegment != relay.UpstreamSegment || input.ClientSegment != relay.ClientSegment {
			clean = false
			continue
		}
		classes, parseErr := relayClassBytes(input.TrafficControl)
		upstream, upstreamOK := classes["1:10"]
		client, clientOK := classes["1:20"]
		sampleClean := len(input.Samples) >= 598
		for index, sample := range input.Samples {
			if sample.At.IsZero() || sample.UpstreamBytes > upstream || sample.ClientBytes > client || index > 0 && (sample.At.Sub(input.Samples[index-1].At) <= 0 || sample.At.Sub(input.Samples[index-1].At) > 1500*time.Millisecond || sample.UpstreamBytes < input.Samples[index-1].UpstreamBytes || sample.ClientBytes < input.Samples[index-1].ClientBytes) {
				sampleClean = false
			}
		}
		if parseErr != nil || !upstreamOK || !clientOK || input.UpstreamBytes > upstream || input.ClientBytes > client || !sampleClean {
			clean = false
			continue
		}
		for _, observed := range []struct {
			id      string
			bytes   uint64
			samples []relaySegmentSample
		}{{relay.UpstreamSegment, upstream, relaySegmentSamples(input.Samples, true)}, {relay.ClientSegment, client, relaySegmentSamples(input.Samples, false)}} {
			segment := segmentDetails[observed.id]
			verdict.Segments = append(verdict.Segments, relaySegmentTraffic{ID: segment.ID, From: segment.From, To: segment.To, Bytes: observed.bytes, Samples: observed.samples})
			if nodes[segment.From] == nil {
				nodes[segment.From] = &relayNodeTraffic{ID: segment.From}
			}
			if nodes[segment.To] == nil {
				nodes[segment.To] = &relayNodeTraffic{ID: segment.To}
			}
			if observed.bytes > math.MaxUint64-nodes[segment.From].Tx || observed.bytes > math.MaxUint64-nodes[segment.To].Rx {
				clean = false
				continue
			}
			nodes[segment.From].Tx += observed.bytes
			nodes[segment.To].Rx += observed.bytes
		}
	}
	for id, node := range nodes {
		if id == userEndpoint || id == publisherEndpoint {
			verdict.Endpoints = append(verdict.Endpoints, *node)
			continue
		}
		verdict.Nodes = append(verdict.Nodes, *node)
	}
	sort.Slice(verdict.Segments, func(i, j int) bool { return verdict.Segments[i].ID < verdict.Segments[j].ID })
	sort.Slice(verdict.Endpoints, func(i, j int) bool { return verdict.Endpoints[i].ID < verdict.Endpoints[j].ID })
	sort.Slice(verdict.Nodes, func(i, j int) bool { return verdict.Nodes[i].ID < verdict.Nodes[j].ID })
	add("relay-terminal-receipts", float64(len(verdict.Segments)), "=", 12, clean && len(verdict.Segments) == 12)
	add("forwarding-node-traffic-owners", float64(len(verdict.Nodes)), "=", 5, clean && len(verdict.Nodes) == 5)
	add("endpoint-traffic-owners", float64(len(verdict.Endpoints)), "=", 2, clean && len(verdict.Endpoints) == 2)
	return verdict, criteria, nil
}

func relayClassBytes(lines []string) (map[string]uint64, error) {
	classes := make(map[string]uint64, 2)
	qdiscs := make(map[string]uint64, 2)
	for _, line := range lines {
		var records []trafficControlRecord
		if json.Unmarshal([]byte(line), &records) != nil {
			continue
		}
		for _, record := range records {
			target, key := classes, record.Handle
			if record.Kind == "netem" {
				target, key = qdiscs, record.Parent
			} else if record.Kind != "htb" {
				continue
			}
			if key == "1:10" || key == "1:20" {
				if _, exists := target[key]; exists {
					return nil, errors.New("relay class counter is duplicated")
				}
				bytes := record.Bytes
				if bytes == 0 {
					bytes = record.Stats.Bytes
				}
				target[key] = bytes
			}
		}
	}
	if len(qdiscs) == 2 {
		return qdiscs, nil
	}
	if len(classes) != 2 {
		return nil, errors.New("relay class counters are incomplete")
	}
	return classes, nil
}

func relaySegmentSamples(samples []relayCounterSample, upstream bool) []relaySegmentSample {
	result := make([]relaySegmentSample, 0, len(samples))
	for _, sample := range samples {
		value := sample.ClientBytes
		if upstream {
			value = sample.UpstreamBytes
		}
		result = append(result, relaySegmentSample{At: sample.At, Bytes: value})
	}
	return result
}
