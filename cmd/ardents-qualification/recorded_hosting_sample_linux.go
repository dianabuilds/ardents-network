//go:build linux

package main

import "time"

// recordedHostingSample preserves the Hosting fields of qualification journals,
// including fields whose types are checked during decoding but are unused by verdicts.
type recordedHostingSample struct {
	At          time.Time
	Policy      recordedHostingPolicy
	Boot        string
	Interfaces  []recordedInterfaceCounters
	Observation recordedHostingObservation
}

type recordedInterfaceCounters struct {
	Name  string
	Index uint64
	Tx    uint64
	Rx    uint64
}

type recordedHostingPolicy struct {
	Provider          string    `json:"provider"`
	Start             time.Time `json:"start"`
	End               time.Time `json:"end"`
	Unit              string    `json:"unit"`
	Quantity          uint64    `json:"quantity"`
	Directions        string    `json:"directions"`
	Interfaces        []string  `json:"interfaces"`
	InitialUsedBytes  uint64    `json:"initial_used_bytes"`
	LowWatermarkBytes uint64    `json:"low_watermark_bytes"`
}

type recordedHostingObservation struct {
	UsedBytes      uint64
	ReservedBytes  uint64
	RemainingBytes uint64
	Protect        bool
	Drain          bool
}
