//go:build live

package network_test

import "encoding/json"

type finalRunnerPlan struct {
	Schema           string `json:"schema"`
	EventID          string `json:"event_id"`
	Group            string `json:"group"`
	Variant          string `json:"variant"`
	Episode          int    `json:"episode"`
	ExpectedTerminal string `json:"expected_terminal"`
	CellID           string `json:"cell_id"`
	Seed             string `json:"seed"`
}

type finalRunnerSchedule struct {
	Schema           string                    `json:"schema"`
	RepositoryCommit string                    `json:"repository_commit"`
	SourceSHA256     string                    `json:"source_sha256"`
	LinuxImage       string                    `json:"linux_image"`
	ImageSHA256      string                    `json:"image_sha256"`
	ProductImageID   string                    `json:"product_image_id"`
	ToolImageID      string                    `json:"tool_image_id"`
	GoBuilderImageID string                    `json:"go_builder_image_id"`
	GoBuilderVersion string                    `json:"go_builder_version"`
	SupplyLock       finalRunnerArtifact       `json:"supply_lock"`
	RuntimeCompose   finalRunnerArtifact       `json:"runtime_compose"`
	ProductReceipt   finalRunnerProductReceipt `json:"product_receipt"`
	ToolReceipt      finalRunnerToolReceipt    `json:"tool_receipt"`
	Kernel           string                    `json:"kernel"`
	ClientSHA256     string                    `json:"client_sha256"`
	ServerSHA256     string                    `json:"server_sha256"`
	Endpoint         json.RawMessage           `json:"endpoint"`
	ReferenceBridge  json.RawMessage           `json:"reference_bridge"`
	StrongerBridge   json.RawMessage           `json:"stronger_bridge"`
	Collector        json.RawMessage           `json:"collector"`
	Network          json.RawMessage           `json:"network"`
	Clocks           json.RawMessage           `json:"clocks"`
	CellOrder        []string                  `json:"cell_order"`
	Seeds            []string                  `json:"seeds"`
	Configurations   json.RawMessage           `json:"configurations"`
}

type finalRunnerProductReceipt struct {
	SourceSHA256    string `json:"source_sha256"`
	GoArchiveSHA256 string `json:"go_archive_sha256"`
	GoRecipeSHA256  string `json:"go_builder_recipe_sha256"`
	GoModuleSHA256  string `json:"go_module_cache_sha256"`
	RouteSHA256     string `json:"route_sha256"`
	BridgeSHA256    string `json:"bridge_sha256"`
	ServiceSHA256   string `json:"service_sha256"`
	StreamSHA256    string `json:"stream_sha256"`
	PublishSHA256   string `json:"publish_sha256"`
	NetworkSHA256   string `json:"network_test_sha256"`
	AdapterSHA256   string `json:"adapter_test_sha256"`
}

type finalRunnerToolReceipt struct {
	BaseDigest     string `json:"base_digest"`
	ToolLockSHA256 string `json:"tool_lock_sha256"`
	SourceSHA256   string `json:"source_sha256"`
	CarrierSHA256  string `json:"carrier_sha256"`
}

type finalRunnerArtifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type finalWorkerResult struct {
	Schema               string                 `json:"schema"`
	CellID               string                 `json:"cell_id"`
	Terminal             string                 `json:"terminal"`
	EvidenceComplete     bool                   `json:"evidence_complete"`
	StartedOffsetMillis  uint64                 `json:"started_offset_millis"`
	TerminalOffsetMillis uint64                 `json:"terminal_offset_millis"`
	CleanupOffsetMillis  uint64                 `json:"cleanup_offset_millis"`
	Observers            []finalRunnerObserver  `json:"observers"`
	ObserverEvidence     finalRunnerArtifact    `json:"observer_evidence"`
	TelemetryEvidence    finalRunnerArtifact    `json:"telemetry_evidence"`
	Sustained            *finalWorkerSustained  `json:"sustained,omitempty"`
	Pressure             *finalPressureEvidence `json:"pressure,omitempty"`
	Residuals            []finalRunnerResidual  `json:"residuals"`
	ObserverSets         uint16                 `json:"observer_sets"`
}

// finalWorkerSustained is one manifest-bound sustained measurement. The runner
// combines the two direct records and five carrier runs for each direction.
type finalWorkerSustained struct {
	Direction             string                     `json:"direction"`
	Kind                  string                     `json:"kind"`
	DirectMbit            float64                    `json:"direct_mbit,omitempty"`
	Direct                *finalDirectRunEvidence    `json:"direct,omitempty"`
	Run                   *finalSustainedRunEvidence `json:"run,omitempty"`
	EndpointCarrierBytes  uint64                     `json:"endpoint_carrier_bytes,omitempty"`
	PublisherCarrierBytes uint64                     `json:"publisher_carrier_bytes,omitempty"`
}

type finalRunnerClosed struct {
	Schema       string              `json:"schema"`
	FinalSummary *finalRunnerSummary `json:"final_summary,omitempty"`
}

type finalRunnerSummary struct {
	Schema     string                       `json:"schema"`
	Cells      []finalRunnerCell            `json:"cells"`
	Profiles   []finalRunnerProfile         `json:"profiles,omitempty"`
	Sustained  []finalSustainedCellEvidence `json:"sustained,omitempty"`
	Pressure   []finalRunnerPressure        `json:"pressure,omitempty"`
	ImageHash  string                       `json:"image_hash"`
	ClientHash string                       `json:"client_hash"`
	ServerHash string                       `json:"server_hash"`
}

type finalRunnerProfile struct {
	ID         string `json:"id"`
	Terminal   string `json:"terminal"`
	Attempts   uint16 `json:"attempts"`
	Successful uint16 `json:"successful"`
}

// finalRunnerPressure deliberately omits the worker-only schema marker. Its
// JSON shape matches the final evidence schema consumed by blockedentry.
type finalRunnerPressure struct {
	ID                   string                      `json:"id"`
	Terminal             string                      `json:"terminal"`
	BaselineSockets      uint16                      `json:"baseline_sockets"`
	Injected             uint16                      `json:"injected"`
	PeakSockets          uint16                      `json:"peak_sockets"`
	Offers               uint16                      `json:"offers"`
	Refused              uint16                      `json:"refused"`
	HighSamples          uint16                      `json:"high_samples"`
	LowSamples           uint16                      `json:"low_samples"`
	Batches              uint16                      `json:"batches"`
	Units                uint16                      `json:"units"`
	StreamMbit           uint16                      `json:"stream_mbit"`
	DurationMillis       uint32                      `json:"duration_millis"`
	CadenceMillis        uint32                      `json:"cadence_millis"`
	PartialBytes         uint16                      `json:"partial_bytes"`
	RatePerSecond        uint16                      `json:"rate_per_second"`
	MaximumRefusalMillis uint32                      `json:"maximum_refusal_millis"`
	ExitMillis           uint32                      `json:"exit_millis"`
	Progress             bool                        `json:"progress"`
	Protect              bool                        `json:"protect"`
	Drain                bool                        `json:"drain"`
	Normal               bool                        `json:"normal"`
	Cleanup              bool                        `json:"cleanup"`
	OOMEvents            uint16                      `json:"oom_events"`
	Residuals            uint16                      `json:"residuals"`
	UpwardTrend          bool                        `json:"upward_trend"`
	Reconciliations      []finalRunnerReconciliation `json:"reconciliations,omitempty"`
}

type finalRunnerReconciliation struct {
	Batch                uint16 `json:"batch"`
	AllocationDelta      int32  `json:"allocation_delta"`
	FDDelta              int32  `json:"fd_delta"`
	SocketDelta          int32  `json:"socket_delta"`
	GoroutineDelta       int32  `json:"goroutine_delta"`
	TimerDelta           int32  `json:"timer_delta"`
	StateBytesDelta      int64  `json:"state_bytes_delta"`
	EvidenceRecordsDelta int32  `json:"evidence_records_delta"`
	CleanupSockets       uint16 `json:"cleanup_sockets"`
	CleanupDescendants   uint16 `json:"cleanup_descendants"`
	CleanupStateBytes    uint64 `json:"cleanup_state_bytes"`
	Residuals            uint16 `json:"residuals"`
}

type finalRunnerCell struct {
	ID                   string `json:"id"`
	Seed                 string `json:"seed"`
	Terminal             string `json:"terminal"`
	StartedOffsetMillis  uint64 `json:"started_offset_millis"`
	TerminalOffsetMillis uint64 `json:"terminal_offset_millis"`
	CleanupOffsetMillis  uint64 `json:"cleanup_offset_millis"`
}

// finalRawObserverSet retains one role-bound parser result per worker root.
// It is sent only to the parent harness, which stores it in the secret tree.
type finalRawObserverSet struct {
	Observers []finalRawObserver `json:"observers"`
}

type finalRawObserver struct {
	Boundary string               `json:"boundary"`
	Role     string               `json:"role"`
	Path     finalPathObservation `json:"path"`
	DNS      finalDNSObservation  `json:"dns"`
}

type finalRawTelemetry struct {
	Root uint16 `json:"root"`
	Role string `json:"role"`
	Kind string `json:"kind"`
	Data []byte `json:"data"`
}

type finalRunnerObservation struct {
	Schema               string                `json:"schema"`
	EventID              string                `json:"event_id"`
	CellID               string                `json:"cell_id"`
	Seed                 string                `json:"seed"`
	ObservedTerminal     string                `json:"observed_terminal"`
	ProductStarted       bool                  `json:"product_started"`
	FaultInjected        bool                  `json:"fault_injected"`
	FaultOwner           string                `json:"fault_owner"`
	Attribution          string                `json:"attribution"`
	AttributionEvidence  string                `json:"attribution_evidence"`
	Diagnostic           string                `json:"diagnostic"`
	StartedOffsetMillis  uint64                `json:"started_offset_millis"`
	TerminalOffsetMillis uint64                `json:"terminal_offset_millis"`
	CleanupOffsetMillis  uint64                `json:"cleanup_offset_millis"`
	AdapterCleanupMillis uint64                `json:"adapter_cleanup_millis"`
	Observers            []finalRunnerObserver `json:"observers"`
	ObserverEvidence     finalRunnerArtifact   `json:"observer_evidence,omitempty"`
	TelemetryEvidence    finalRunnerArtifact   `json:"telemetry_evidence,omitempty"`
	Residuals            []finalRunnerResidual `json:"residuals"`
}

type finalRunnerObserver struct {
	Boundary             string `json:"boundary"`
	IPv4UDPControl       bool   `json:"ipv4_udp_control"`
	IPv6UDPControl       bool   `json:"ipv6_udp_control"`
	IPv4TCPControl       bool   `json:"ipv4_tcp_control"`
	Attribution          string `json:"attribution"`
	ForbiddenPackets     uint64 `json:"forbidden_packets"`
	ForbiddenOwner       string `json:"forbidden_owner"`
	UnclassifiedPackets  uint64 `json:"unclassified_packets"`
	ObservationCompleted bool   `json:"observation_completed"`
}

type finalRunnerResidual struct {
	Kind                string `json:"kind"`
	Count               uint64 `json:"count"`
	Owner               string `json:"owner"`
	AttributionEvidence string `json:"attribution_evidence"`
}
