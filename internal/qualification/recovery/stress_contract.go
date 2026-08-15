package recovery

import "encoding/json"

const stressAttemptManifestSchema = "ardents-h3-s43-attempt-manifest-v1"

type stressAttemptManifest struct {
	Schema, SourceCommit, ImageID, ToolImageID, TopologyDigest string
	Topology                                                   []byte
	HostScope                                                  json.RawMessage
	RouteCase                                                  routeCase
	Candidates                                                 []replacementCandidate
	RouteManifest                                              [32]byte
	Prerequisites                                              []replacementPrerequisite
	Cells                                                      []replacementAttemptCell
}

type impairedCell struct {
	Direction, Mode, CellManifestDigest                                        string
	HostProcesses                                                              map[string]processObservationEvidence
	Seed, ExpectedDigest, ObservedDigest                                       [32]byte
	Bytes, MeasurementDelivered                                                uint32
	HostStartedAtNanos, ActiveStartedAtNanos                                   int64
	MeasurementCompletedAtNanos, TerminalNanos                                 int64
	ClientRouteGeneration, PublisherRouteGeneration                            uint64
	ClientRecoveryCount, PublisherRecoveryCount                                uint32
	ClientApplicationAccepts, PublisherApplicationAccepts                      uint32
	ClientRouteAccepts, PublisherRouteAccepts                                  uint32
	ClientAcceptedBytes, ClientAcknowledgedBytes, ClientReceivedBytes          uint32
	PublisherAcceptedBytes, PublisherAcknowledgedBytes, PublisherReceivedBytes uint32
	ClientQueueHighWater, PublisherQueueHighWater                              uint32
	ClientContinuity, PublisherContinuity                                      [32]byte
	Ordered, Unique, SameConnection, ApplicationReconnected                    bool
	TerminalClean                                                              bool
	Progress                                                                   []progressSample
	ResourceSamples                                                            []ResourceSample
	TrafficStart, TrafficEnd                                                   ResourceSample
	DirectBefore, DirectAfter                                                  directBaseline
	Shapers                                                                    []shaperEvidence
}

type progressSample struct {
	AtNanos   int64
	Delivered uint32
}

type directBaseline struct {
	Direction                                string
	Seed, ExpectedDigest, ObservedDigest     [32]byte
	Bytes, MeasurementDelivered              uint32
	ActiveStartedAtNanos, ActiveEndedAtNanos int64
	TerminalNanos                            int64
	Progress                                 []progressSample
	Processes                                map[string]processObservationEvidence
	Shapers                                  []shaperEvidence
}

type shaperEvidence struct {
	Role, ContainerID, TargetContainer, ToolImageID, ConfigDigest string
	ReadyObservedAtNanos, CompletedAtNanos                        int64
	Observer                                                      ObserverProcess
	Config                                                        json.RawMessage
	Result                                                        json.RawMessage
	Removed                                                       bool
}
