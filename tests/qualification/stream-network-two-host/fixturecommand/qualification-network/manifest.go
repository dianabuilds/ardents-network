//go:build linux

package main

import (
	"errors"
	"fmt"
)

func makeNetworkManifest(config fixtureConfig, nodes []fixtureNode, readerEntry, readerInterior,
	publisherInterior, publisherEntry [32]byte, dataJoin string) (networkManifest, error) {
	ids := []string{hex32(readerEntry), hex32(readerInterior), dataJoin, hex32(publisherInterior), hex32(publisherEntry)}
	forwardPoints := append(append([]string{"user"}, ids...), "publisher")
	reversePoints := []string{"publisher", ids[4], ids[3], ids[2], ids[1], ids[0], "user"}
	forward := makeNetworkPath(config.Cell, "user-to-publisher", forwardPoints, false)
	reverse := makeNetworkPath(config.Cell, "publisher-to-user", reversePoints, true)
	manifest := networkManifest{Version: 1, Cell: config.Cell, Carrier: config.Carrier,
		Generator:    networkGenerator{Algorithm: "linux-netem", Version: "issue60-fixture-v1"},
		SeedSchedule: []string{config.Seed}, Paths: []networkPath{forward, reverse}}
	pathNodes := []string{ids[0], ids[1], ids[2], ids[2], ids[3], ids[4]}
	targetPorts := [...]int{49000, 49001, 49002, 49002, 49003, 49004}
	for index := 0; index < 6; index++ {
		host := "reader"
		publicHost := config.ReaderHost
		if index >= 4 {
			host, publicHost = "publisher", config.PublisherHost
		}
		port, err := nodeEndpointPort(nodes, pathNodes[index])
		if err != nil {
			return networkManifest{}, err
		}
		if index == 3 {
			port = 48500
		}
		network := "tcp"
		if config.Carrier == "quic" {
			network = "udp"
		}
		mode := "net14ad-delay"
		if config.Cell == "net14s" {
			mode = "net14s-delay"
		}
		if forward.Segments[index].LossPartsPerMillion != 0 {
			if config.Cell == "net14s" {
				mode = "net14s-loss"
			} else {
				mode = "net14ad-loss"
			}
		}
		upstreamRate := uint64(100_000_000)
		if index == 0 {
			upstreamRate = 20_000_000
		}
		manifest.Relays = append(manifest.Relays, networkRelay{
			ID: fmt.Sprintf("issue60-relay-%d", index), Host: host,
			Container: fmt.Sprintf("ardents-issue60-relay-%d", index), Network: network,
			Listen: fmt.Sprintf(":%d", port), PublicEndpoint: fmt.Sprintf("%s:%d", publicHost, port),
			Target: fmt.Sprintf("172.17.0.1:%d", targetPorts[index]), Mode: mode,
			UpstreamSegment: forward.Segments[index].ID, ClientSegment: reverse.Segments[5-index].ID,
			UpstreamRate: upstreamRate, ClientRate: 100_000_000,
		})
	}
	if config.Cell == "net14-recovery" {
		manifest.Failures = []networkFailure{
			{Episode: "reader-entry-loss", SegmentID: forward.Segments[1].ID, AtMillis: 60_000, DurationMillis: 5_000},
			{Episode: "data-join-loss", SegmentID: forward.Segments[3].ID, AtMillis: 180_000, DurationMillis: 5_000},
			{Episode: "publisher-entry-loss", SegmentID: forward.Segments[4].ID, AtMillis: 360_000, DurationMillis: 5_000},
		}
	}
	return manifest, nil
}

func makeNetworkPath(cell, name string, points []string, reverse bool) networkPath {
	segments := make([]networkSegment, 0, 6)
	for index := 0; index < 6; index++ {
		physical := index
		if reverse {
			physical = 5 - index
		}
		delay := uint64(6666)
		loss := uint64(0)
		if cell == "net14s" {
			delay = 25_000
		}
		if physical == 5 && cell != "net14s" {
			delay = 6670
		}
		if physical == 2 {
			if cell == "net14s" {
				loss = 50_000
			} else {
				loss = 1_000
			}
		}
		segments = append(segments, networkSegment{ID: fmt.Sprintf("%s-%d", name, index),
			From: points[index], To: points[index+1], DelayMicros: delay, LossPartsPerMillion: loss})
	}
	capacity := uint64(20_000_000)
	if reverse {
		capacity = 100_000_000
	}
	return networkPath{Name: name, CapBitsPerS: capacity, Segments: segments}
}

func nodeEndpointPort(nodes []fixtureNode, id string) (int, error) {
	for index := range nodes {
		if nodes[index].ID == id {
			return 50000 + index, nil
		}
	}
	return 0, errors.New("fixture Route references an absent Node")
}
