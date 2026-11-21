//go:build linux

package main

// recordedProcessSample is the process observation carried in retained Node
// and Source journals. Keep every known JSON field type so a malformed field
// invalidates the same recorded sample, including fields unused by verdicts.
type recordedProcessSample struct {
	RSSBytes          uint64  `json:"rss_bytes"`
	CPUUsageUsec      uint64  `json:"cpu_usage_usec"`
	MemoryBytes       uint64  `json:"memory_bytes"`
	GoMemoryBytes     uint64  `json:"go_memory_bytes"`
	SocketMemoryBytes uint64  `json:"socket_memory_bytes"`
	Sockets           uint64  `json:"sockets"`
	FDs               uint64  `json:"fds"`
	Goroutines        uint64  `json:"goroutines"`
	Threads           uint64  `json:"threads"`
	Timers            uint64  `json:"timers"`
	QueueItems        uint64  `json:"queue_items"`
	QueueBytes        uint64  `json:"queue_bytes"`
	StorageBytes      uint64  `json:"storage_bytes"`
	StorageFiles      uint64  `json:"storage_files"`
	CPUPressure       float64 `json:"cpu_pressure"`
	MemoryPressure    float64 `json:"memory_pressure"`
	IOPressure        float64 `json:"io_pressure"`
	HighEvents        uint64  `json:"high_events"`
	EmergencyEvents   uint64  `json:"emergency_events"`
	AdmissionActive   uint64  `json:"admission_active"`
	AdmissionAccepted uint64  `json:"admission_accepted"`
	AdmissionRefused  uint64  `json:"admission_refused"`
}
