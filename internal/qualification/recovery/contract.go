package recovery

// Evidence is the bounded, public-only S4.1 observer record.
type Evidence struct {
	Schema, SourceCommit, ImageID, TopologyDigest, ManifestDigest string
	VerifierImageID, Claim                                        string
	Target, Instance, NetworkID, CandidateView                    [32]byte
	IsolationContext, DestinationBinding, AuthorityPublic         [32]byte
	ClientPrincipal, PublisherPrincipal                           [32]byte
	RouteProfile                                                  string
	CredentialGeneration                                          uint64
	CredentialNotBefore, CredentialNotAfter                       int64
	WorkSafetyNotAfter, WorkSafetyMaximum, NoNewRecoveryAfter     int64
	BinaryDigests                                                 map[string]string
	Topology                                                      []byte
	Manifest                                                      PublicManifest
	RequestedNanos, CampaignNanos                                 int64
	Cells                                                         []Cell
	Negatives                                                     map[string]Negative
	Cleanup                                                       cleanup
}

// PublicManifest contains every public input to the S4.1 connection binding.
type PublicManifest struct {
	RouteManifest, NetworkID, AuthorityPublic, IntroductionPublic [32]byte
	Target, InstancePublic, ClientPrincipal, PublisherPrincipal   [32]byte
	CredentialSignature                                           [64]byte
	CredentialGeneration                                          uint64
	CredentialNotBefore, CredentialNotAfter                       int64
	CredentialCapabilities                                        uint32
	RouteProfile                                                  string
	WorkSafetyNotAfter, WorkSafetyMaximum, NoNewRecoveryAfter     int64
}

// Cell records one externally observed directional Carrier recovery.
type Cell struct {
	Direction, ClientProcess, PublisherProcess                     string
	ClientApplicationProcess, PublisherApplicationProcess          string
	InitialCarrier, ReplacementCarrier                             string
	InitialCarrierLocal, InitialCarrierRemote                      string
	ReplacementCarrierLocal, ReplacementCarrierRemote              string
	FaultedCarrier, RetiredCarrier                                 string
	InitialCarrierInode, ReplacementCarrierInode                   uint32
	InitialCarrierInterface, ReplacementCarrierInterface           string
	InitialCarrierInterfaceIndex, ReplacementCarrierInterfaceIndex int
	CellManifestDigest                                             string
	FaultService, FaultContainer, FaultNetwork, FaultController    string
	FaultControllerRemoved                                         bool
	ReplacementObserver                                            ObserverProcess
	InitialRouteContainers, RecoveredRouteContainers               map[string]string
	InitialRoutePIDs, RecoveredRoutePIDs                           map[string]uint32
	Seed                                                           [32]byte
	ExpectedDigest, ObservedDigest, Canary                         [32]byte
	Bytes, PlannedFaultOffset, FaultOffset                         uint32
	DeliveredBeforeFault, CanaryOffset                             uint32
	LastDeliveryNanos, CarrierObservedNanos, FaultAtNanos          int64
	FaultCompletedNanos                                            int64
	CarrierCutAfterNanos, AbsenceAfterNanos                        int64
	CarrierAttachmentDeadlineNanos, OldCarrierRetiredNanos         int64
	CanaryAtNanos, ReplacementObservedNanos                        int64
	TerminalAtNanos                                                int64
	ClientRouteGeneration, PublisherRouteGeneration                uint64
	ClientRecoveryCount, PublisherRecoveryCount                    uint32
	ClientApplicationAccepts, PublisherApplicationAccepts          uint32
	ClientRouteAccepts, PublisherRouteAccepts                      uint32
	ClientContinuity, PublisherContinuity                          [32]byte
	Ordered, Unique, SameConnection, ApplicationReconnected        bool
	OldCarrierReused, OldCarrierRetired, TerminalClean             bool
	FaultResourceAbsent, FailedResourceUnavailable                 bool
	QueueHighWater                                                 uint32
	MemoryHighWater, CarrierForwardBytes, CarrierReverseBytes      uint64
	CPUSeconds                                                     float64
	ExternalCPUPercent                                             float64
	ExternalStatsObserved                                          bool
	OpenFilesHighWater, GoroutinesHighWater, TimerHighWater        uint32
	ResourceSamples                                                []ResourceSample
	BaselineClientTraffic, BaselinePublisherTraffic                uint64
}

// ObserverProcess is the host-inspected public confinement projection of a transient Carrier observer.
type ObserverProcess struct {
	ContainerID, ImageID, NetworkMode, User string
	PIDMode, IPCMode                        string
	Command, CapAdd, CapDrop, SecurityOpt   []string
	ReadOnly, Privileged, Removed           bool
	MountCount                              uint32
	PidsLimit                               int64
	MemoryLimit, NanoCPUs                   int64
}

// ResourceSample is one host-observed endpoint-tree sample.
type ResourceSample struct {
	AtNanos                                                      int64
	ClientRSS, PublisherRSS                                      uint64
	ClientCPUPercent, PublisherCPUPercent                        float64
	ClientReceived, ClientSent, PublisherReceived, PublisherSent uint64
}

// Negative records one isolated fail-closed S4.1 case.
type Negative struct {
	TerminalCount                                              uint32
	Class                                                      string
	WithinNanos                                                int64
	Passed                                                     bool
	ContainerID, InjectedResource, BeforeProcess, AfterProcess string
	InjectionKind, InjectionDigest                             string
	AttackAttempts, RecoveryCount                              uint32
	RouteGeneration                                            uint64
}

type cleanup struct {
	DockerEmpty, FixtureAbsent, PrivateMaterialAbsent bool
}

// Result is exactly one independent pass, fail, or invalid verdict.
type Result struct {
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
}
