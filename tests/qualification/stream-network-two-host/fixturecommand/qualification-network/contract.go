//go:build linux

package main

import "time"

type fixtureConfig struct {
	Output, ReaderHost, PublisherHost string
	Carrier, Cell, Profile, Seed      string
	At                                time.Time
}

type fixtureNode struct {
	ID, PublicKey, Family, Endpoint, Host string
	RoleDomain, Subrole                   uint8
	DutyGeneration, MaterializationIndex  uint64
	Key, Certificate                      string
	RecordSHA256                          string
}

type fixtureParticipant struct {
	Name, Host, ID       string
	MaterializationIndex uint64
	EntryRoot            string
}

type fixtureSource struct {
	Name, Host, ID, StateNodeID, Endpoint, ServerName string
	MaterializationIndex                              uint64
	ServerCertificate, ServerKey, ServerRootCA        string
	ClientCertificate, ClientKey, ClientRootCA        string
	ServerLeafKeyDigest, ClientKeyDigest              string
}

type fixtureBundle struct {
	Schema, NetworkID, AuthorityPublic, AuthorityKey, Carrier, Cell, Profile, Seed string
	At, NotAfter                                                                   string
	Epoch, Inputs                                                                  string
	EpochSHA256                                                                    string
	Nodes                                                                          []fixtureNode
	Participants                                                                   []fixtureParticipant
	Sources                                                                        []fixtureSource
	ReaderEntry, ReaderInterior, PublisherEntry, PublisherInterior                 string
	DataJoin                                                                       string
	NetworkManifest                                                                string
	RemoteRoot, NodeInventory                                                      string
	NodePlans, SourcePlans                                                         []string
	HostingRoot, ClockObservationFile                                              string
	ReaderPlanTemplate, PublisherPlan, Net32Plan, ClosedProfilePlan                string
	IssuerInitialization, PreparationInventory                                     string
	ServiceInitializationPlans                                                     []string
}

type networkManifest struct {
	Version      int
	Cell         string
	Carrier      string
	Generator    networkGenerator
	SeedSchedule []string
	Paths        []networkPath
	Relays       []networkRelay
	Failures     []networkFailure
}
type networkGenerator struct{ Algorithm, Version string }
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
type networkRelay struct {
	ID, Host, Container, Network, Listen, PublicEndpoint, Target string
	Mode, UpstreamSegment, ClientSegment                         string
	UpstreamRate, ClientRate                                     uint64
}
type networkFailure struct {
	Episode, SegmentID       string
	AtMillis, DurationMillis uint64
}
