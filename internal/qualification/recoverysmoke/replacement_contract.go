package recoverysmoke

import (
	"encoding/json"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

type replacementEvidence struct {
	RouteCase  json.RawMessage
	Candidates []replacementCandidate
	Cells      []replacementCell
}

type replacementCandidate struct {
	Role, Family, Endpoint string
	NodeID, PublicKey      [32]byte
}

type replacementCell struct {
	Direction, Mode                                                            string
	CellManifestDigest, FaultFamily                                            string
	FailureRoles                                                               []string
	FaultOffsets                                                               []uint32
	HostProcesses                                                              map[string]processObservationEvidence
	Seed, ExpectedDigest, ObservedDigest                                       [32]byte
	Bytes, ChunkBytes, CanaryBytes                                             uint32
	ChunkDelayNanos, SetupDeadlineNanos, LifetimeNanos, HostStartedAtNanos     int64
	ResourceStartedAtNanos                                                     int64
	TerminalNanos                                                              int64
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
	Routes                                                                     []routeGeneration
	Proposals                                                                  []replacementProposal
	Events                                                                     []replacementEvent
	BaselineClientTrafficObserver, BaselinePublisherTrafficObserver            recovery.ObserverProcess
	ClientTrafficObserver, PublisherTrafficObserver                            recovery.ObserverProcess
	BaselineClientRoute, BaselinePublisherRoute                                string
	ClientRoute, PublisherRoute                                                string
	ResourceSamples                                                            []recovery.ResourceSample
	FinalTraffic, BaselineFinalTraffic                                         recovery.ResourceSample
	BaselineTerminalNanos                                                      int64
	BaselineClientTraffic, BaselinePublisherTraffic                            uint64
	FinalCanaryOffset                                                          uint32
	FinalCanary                                                                [32]byte
	FinalCanaryObservedNanos                                                   int64
}

type replacementProposal struct {
	Attachment          uint32
	NodeIDs             [4][32]byte
	PublicKeys          [4][32]byte
	ExcludedIdentities  [][32]byte
	Terminal            string
	Committed           bool
	IntroductionReceipt [32]byte
	IntroductionProof   introductionProof
	Processes           [4]candidateProcess
	Stopped             [4]failedResourceReceipt
	PlanTimings         map[string]routePlanTiming
}

type routePlanTiming struct {
	Process                    processEvidenceRef
	Attachment, DeadlineMillis uint32
	LifetimeMillis             uint32
}

type introductionProof struct {
	ManifestDigest, NetworkID, EpochDigest, ViewRoot         [32]byte
	ProfileDigest, CapabilitiesDigest                        [32]byte
	IntroductionNode, RendezvousNode, RendezvousReachability [32]byte
	JoinHandle, EndpointHandshake, TranscriptContext, Reply  [32]byte
	ExpiresAtNanos                                           int64
}

type routeGeneration struct {
	Generation uint64
	Processes  map[string]candidateProcess
}

type candidateProcess struct {
	Service, ContainerID, Incarnation string
	PID                               uint32
	ObservedAtNanos                   int64
	HostObservation                   [32]byte
	AdapterProjection                 string
	NodeID, PublicKey                 [32]byte
	Host                              processEvidenceRef
}

type processObservationEvidence struct {
	Host              processEvidenceRef
	PID               uint32
	ObservedAtNanos   int64
	HostObservation   [32]byte
	AdapterProjection string
}

type replacementEvent struct {
	Role, Layer                                  string
	GenerationBefore, GenerationAfter            uint64
	Failed, Replacement                          candidateProcess
	RendezvousBefore, RendezvousAfter            candidateProcess
	Introduction                                 candidateProcess
	IntroductionAttachment                       uint32
	IntroductionSetupAttachment                  uint32
	IntroductionSetupReceipt                     [32]byte
	IntroductionOpaqueDigest                     [32]byte
	IntroductionOpaqueBytes                      uint64
	FaultOffset, CanaryOffset                    uint32
	Canary                                       [32]byte
	LastDeliveryNanos, FaultAtNanos, CanaryNanos int64
	FailedResource                               failedResourceReceipt
}

type failedResourceReceipt struct {
	ContainerID     string
	ObservedAtNanos int64
	Running         bool
	Fault           processFaultEvidence
	State           processStateEvidence
}
