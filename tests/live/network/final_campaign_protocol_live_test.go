//go:build live

package network_test

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
	CellOrder []string `json:"cell_order"`
	Seeds     []string `json:"seeds"`
}

type finalWorkerResult struct {
	Schema               string                `json:"schema"`
	CellID               string                `json:"cell_id"`
	Terminal             string                `json:"terminal"`
	EvidenceComplete     bool                  `json:"evidence_complete"`
	StartedOffsetMillis  uint64                `json:"started_offset_millis"`
	TerminalOffsetMillis uint64                `json:"terminal_offset_millis"`
	CleanupOffsetMillis  uint64                `json:"cleanup_offset_millis"`
	Observers            []finalRunnerObserver `json:"observers"`
	Residuals            []finalRunnerResidual `json:"residuals"`
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
