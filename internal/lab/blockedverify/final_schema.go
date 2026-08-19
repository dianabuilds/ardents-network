package blockedverify

type finalSpec struct {
	Schema            string                  `json:"schema"`
	RepositoryCommit  string                  `json:"repository_commit"`
	SourceSHA256      string                  `json:"source_sha256"`
	LinuxImage        string                  `json:"linux_image"`
	ImageSHA256       string                  `json:"image_sha256"`
	ProductImageID    string                  `json:"product_image_id"`
	ToolImageID       string                  `json:"tool_image_id"`
	GoBuilderImageID  string                  `json:"go_builder_image_id"`
	GoBuilderVersion  string                  `json:"go_builder_version"`
	SupplyLock        artifactCommitment      `json:"supply_lock"`
	RuntimeCompose    artifactCommitment      `json:"runtime_compose"`
	ProductReceipt    finalProductReceipt     `json:"product_receipt"`
	ToolReceipt       finalToolReceipt        `json:"tool_receipt"`
	Kernel            string                  `json:"kernel"`
	ClientSHA256      string                  `json:"client_sha256"`
	ServerSHA256      string                  `json:"server_sha256"`
	Endpoint          finalHostClass          `json:"endpoint"`
	ReferenceBridge   finalHostClass          `json:"reference_bridge"`
	StrongerBridge    finalHostClass          `json:"stronger_bridge"`
	Collector         finalHostClass          `json:"collector"`
	Network           finalNetwork            `json:"network"`
	Clocks            finalClocks             `json:"clocks"`
	CellOrder         []string                `json:"cell_order"`
	Seeds             []string                `json:"seeds"`
	MutationCampaigns []finalMutationCampaign `json:"mutation_campaigns"`
	Configurations    []artifactCommitment    `json:"configurations"`
}

type finalHostClass struct {
	ID               string  `json:"id"`
	OperatingSystem  string  `json:"operating_system"`
	Architecture     string  `json:"architecture"`
	StorageClass     string  `json:"storage_class"`
	Dedicated        bool    `json:"dedicated"`
	VCPU             uint16  `json:"vcpu"`
	MemoryMiB        uint32  `json:"memory_mib"`
	LinkDownMbit     uint16  `json:"link_down_mbit"`
	LinkUpMbit       uint16  `json:"link_up_mbit"`
	CPUMaxCores      float64 `json:"cpu_max_cores,omitempty"`
	CPUMeanCores     float64 `json:"cpu_mean_cores,omitempty"`
	CPUP95Cores      float64 `json:"cpu_p95_cores,omitempty"`
	MemoryMaxMiB     uint32  `json:"memory_max_mib,omitempty"`
	MemoryP95MiB     uint32  `json:"memory_p95_mib,omitempty"`
	HelperRSSP95MiB  uint32  `json:"helper_rss_p95_mib,omitempty"`
	HelperFDs        uint16  `json:"helper_fds,omitempty"`
	HelperSockets    uint16  `json:"helper_sockets,omitempty"`
	MinimumReservePC uint8   `json:"minimum_reserve_percent,omitempty"`
}

type finalNetwork struct {
	BaseRTTMillis   uint16 `json:"base_rtt_millis"`
	LossPPM         uint16 `json:"loss_ppm_each_direction"`
	JitterP95Millis uint16 `json:"jitter_p95_millis"`
	Interruption    bool   `json:"interruption"`
	Reordering      bool   `json:"reordering"`
}

type finalClocks struct {
	OrdinaryBlockedMillis uint32 `json:"ordinary_blocked_millis"`
	TransitionMillis      uint32 `json:"transition_millis"`
	AttemptMillis         uint32 `json:"attempt_millis"`
	ContactMillis         uint32 `json:"contact_millis"`
	StartupMillis         uint32 `json:"startup_millis"`
	InterContactMillis    uint32 `json:"inter_contact_millis"`
	AdapterCleanupMillis  uint32 `json:"adapter_cleanup_millis"`
	CellCleanupMillis     uint32 `json:"cell_cleanup_millis"`
}

type finalSummary struct {
	Schema            string                 `json:"schema"`
	Cells             []finalCellObservation `json:"cells"`
	Profiles          []finalProfileResult   `json:"profiles"`
	Capacity          []finalCapacityBatch   `json:"capacity"`
	Sustained         []finalSustainedCell   `json:"sustained"`
	Pressure          []finalPressureCell    `json:"pressure"`
	Recovery          finalRecovery          `json:"recovery"`
	Hosts             []finalObservedHost    `json:"hosts"`
	ImageHash         string                 `json:"image_hash"`
	ClientHash        string                 `json:"client_hash"`
	ServerHash        string                 `json:"server_hash"`
	Artifacts         []artifactCommitment   `json:"measurement_artifacts"`
	MutationArtifacts []artifactCommitment   `json:"mutation_artifacts"`
}

type finalCellObservation struct {
	ID                   string             `json:"id"`
	Seed                 string             `json:"seed"`
	Terminal             string             `json:"terminal"`
	StartedOffsetMillis  uint64             `json:"started_offset_millis"`
	TerminalOffsetMillis uint64             `json:"terminal_offset_millis"`
	CleanupOffsetMillis  uint64             `json:"cleanup_offset_millis"`
	ObserverEvidence     artifactCommitment `json:"observer_evidence"`
	TelemetryEvidence    artifactCommitment `json:"telemetry_evidence"`
}

type finalProfileResult struct {
	ID         string `json:"id"`
	Terminal   string `json:"terminal"`
	Attempts   uint16 `json:"attempts"`
	Successful uint16 `json:"successful"`
}

type finalCapacityBatch struct {
	Profile              string                   `json:"profile"`
	Terminal             string                   `json:"terminal"`
	Batch                uint16                   `json:"batch"`
	Offered              uint16                   `json:"offered"`
	Accepted             uint16                   `json:"accepted"`
	Refused              uint16                   `json:"refused"`
	MaximumRefusalMillis uint32                   `json:"maximum_refusal_millis"`
	EstablishedProgress  bool                     `json:"established_progress"`
	Cleanup              bool                     `json:"cleanup"`
	SecurityExact        bool                     `json:"security_exact"`
	ReservePercent       float64                  `json:"reserve_percent"`
	ResponseP95Millis    uint32                   `json:"response_p95_millis"`
	Resources            finalResourceObservation `json:"resources"`
}

type finalSustainedCell struct {
	Direction             string              `json:"direction"`
	DirectBeforeMbit      float64             `json:"direct_before_mbit"`
	DirectAfterMbit       float64             `json:"direct_after_mbit"`
	DirectBeforeValid     bool                `json:"direct_before_valid"`
	DirectAfterValid      bool                `json:"direct_after_valid"`
	Runs                  []finalSustainedRun `json:"runs"`
	EndpointCarrierRatio  float64             `json:"endpoint_carrier_ratio"`
	PublisherCarrierRatio float64             `json:"publisher_carrier_ratio"`
	DirectPairID          string              `json:"direct_pair_id"`
	DirectBefore          finalDirectRun      `json:"direct_before"`
	DirectAfter           finalDirectRun      `json:"direct_after"`
	EndpointCarrierBytes  uint64              `json:"endpoint_carrier_bytes"`
	PublisherCarrierBytes uint64              `json:"publisher_carrier_bytes"`
	DeliveredBytes        uint64              `json:"delivered_bytes"`
}

type finalDirectRun struct {
	StartedOffsetMillis  uint64 `json:"started_offset_millis"`
	FinishedOffsetMillis uint64 `json:"finished_offset_millis"`
	DurationMillis       uint32 `json:"duration_millis"`
	DeliveredBytes       uint64 `json:"delivered_bytes"`
	Digest               string `json:"digest"`
	PairID               string `json:"pair_id"`
	Complete             bool   `json:"complete"`
}

type finalSustainedRun struct {
	StartedOffsetMillis  uint64                   `json:"started_offset_millis"`
	FinishedOffsetMillis uint64                   `json:"finished_offset_millis"`
	WindowEndsMillis     []uint64                 `json:"window_ends_millis"`
	WindowsMbit          []float64                `json:"windows_mbit"`
	Resources            finalResourceObservation `json:"resources"`
	Complete             bool                     `json:"complete"`
	DeliveredBytes       uint64                   `json:"delivered_bytes"`
	Digest               string                   `json:"digest"`
}

type finalResourceObservation struct {
	EndpointCPUMean     float64  `json:"endpoint_cpu_mean_cores"`
	EndpointCPUP95      float64  `json:"endpoint_cpu_p95_cores"`
	EndpointRSSP95MiB   float64  `json:"endpoint_rss_p95_mib"`
	BridgeCPUMean       float64  `json:"bridge_cpu_mean_cores"`
	BridgeCPUP95        float64  `json:"bridge_cpu_p95_cores"`
	BridgeMemoryP95MiB  float64  `json:"bridge_memory_p95_mib"`
	HelperRSSP95MiB     float64  `json:"helper_rss_p95_mib"`
	HelperFDPeak        uint16   `json:"helper_fd_peak"`
	HelperSocketPeak    uint16   `json:"helper_socket_peak"`
	SwapEvents          uint16   `json:"swap_events"`
	OOMEvents           uint16   `json:"oom_events"`
	ReservePercent      float64  `json:"reserve_percent"`
	Samples             uint16   `json:"samples"`
	SamplesComplete     bool     `json:"samples_complete"`
	AdapterRSSP95MiB    float64  `json:"adapter_rss_p95_mib"`
	AdapterFDPeak       uint16   `json:"adapter_fd_peak"`
	AdapterSocketPeak   uint16   `json:"adapter_socket_peak"`
	AdapterStateBytes   uint32   `json:"adapter_state_bytes"`
	AdapterStateEntries uint16   `json:"adapter_state_entries"`
	ThreadsPeak         uint16   `json:"threads_peak"`
	GoroutinesPeak      uint16   `json:"goroutines_peak"`
	TimersPeak          uint16   `json:"timers_peak"`
	QueueItemsPeak      uint16   `json:"queue_items_peak"`
	QueueBytesPeak      uint32   `json:"queue_bytes_peak"`
	DurableMembers      uint16   `json:"durable_members"`
	DurableContacts     uint16   `json:"durable_contacts"`
	DurableAttempts     uint16   `json:"durable_attempts"`
	DurableRegimes      uint16   `json:"durable_regimes"`
	DurableStateBytes   uint32   `json:"durable_state_bytes"`
	EvidenceBytes       uint64   `json:"evidence_bytes"`
	EvidenceProjectedPC float64  `json:"evidence_projected_percent"`
	EvidenceDropped     uint16   `json:"evidence_dropped"`
	Descendants         uint16   `json:"descendants"`
	Capabilities        uint16   `json:"capabilities"`
	Collected           []string `json:"collected"`
}

type finalPressureCell struct {
	Schema               string                `json:"schema,omitempty"`
	ID                   string                `json:"id"`
	Terminal             string                `json:"terminal"`
	BaselineSockets      uint16                `json:"baseline_sockets"`
	Injected             uint16                `json:"injected"`
	PeakSockets          uint16                `json:"peak_sockets"`
	Offers               uint16                `json:"offers"`
	Refused              uint16                `json:"refused"`
	HighSamples          uint16                `json:"high_samples"`
	LowSamples           uint16                `json:"low_samples"`
	Batches              uint16                `json:"batches"`
	Units                uint16                `json:"units"`
	StreamMbit           uint16                `json:"stream_mbit"`
	DurationMillis       uint32                `json:"duration_millis"`
	CadenceMillis        uint32                `json:"cadence_millis"`
	PartialBytes         uint16                `json:"partial_bytes"`
	RatePerSecond        uint16                `json:"rate_per_second"`
	MaximumRefusalMillis uint32                `json:"maximum_refusal_millis"`
	ExitMillis           uint32                `json:"exit_millis"`
	Progress             bool                  `json:"progress"`
	Protect              bool                  `json:"protect"`
	Drain                bool                  `json:"drain"`
	Normal               bool                  `json:"normal"`
	Cleanup              bool                  `json:"cleanup"`
	OOMEvents            uint16                `json:"oom_events"`
	Residuals            uint16                `json:"residuals"`
	UpwardTrend          bool                  `json:"upward_trend"`
	Reconciliations      []finalReconciliation `json:"reconciliations,omitempty"`
}
type finalReconciliation struct {
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

type finalRecovery struct {
	Attempts              uint16 `json:"attempts"`
	ConnectionLoss        uint16 `json:"connection_loss"`
	LaterStarts           uint16 `json:"later_starts"`
	Residuals             uint16 `json:"residuals"`
	AttemptIdentityStable bool   `json:"attempt_identity_stable"`
	DeadlineStable        bool   `json:"deadline_stable"`
}
