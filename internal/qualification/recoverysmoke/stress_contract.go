package recoverysmoke

import (
	"encoding/json"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

type impairedCell struct {
	Direction, Mode, CellManifestDigest                               string
	HostProcesses                                                     map[string]processObservationEvidence
	Seed, ExpectedDigest, ObservedDigest                              [32]byte
	Bytes, MeasurementDelivered                                       uint32
	HostStartedAtNanos, ActiveStartedAtNanos                          int64
	MeasurementCompletedAtNanos, TerminalNanos                        int64
	ClientRouteGeneration, PublisherRouteGeneration                   uint64
	ClientRecoveryCount, PublisherRecoveryCount                       uint32
	ClientApplicationAccepts, PublisherApplicationAccepts             uint32
	ClientRouteAccepts, PublisherRouteAccepts                         uint32
	ClientAcceptedBytes, ClientAcknowledgedBytes, ClientReceivedBytes uint32
	PublisherAcceptedBytes, PublisherAcknowledgedBytes                uint32
	PublisherReceivedBytes                                            uint32
	ClientQueueHighWater, PublisherQueueHighWater                     uint32
	ClientContinuity, PublisherContinuity                             [32]byte
	Ordered, Unique, SameConnection, ApplicationReconnected           bool
	TerminalClean                                                     bool
	Progress                                                          []progressSample
	ResourceSamples                                                   []recovery.ResourceSample
	TrafficStart, TrafficEnd                                          recovery.ResourceSample
	DirectBefore, DirectAfter                                         directBaseline
	Shapers                                                           []shaperEvidence
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
	Observer                                                      recovery.ObserverProcess
	Config                                                        json.RawMessage
	Result                                                        json.RawMessage
	Removed                                                       bool
}
