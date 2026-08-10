package nativecircuit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"
)

const nativeEvidenceSchema = "carrier-lab-native-role-evidence/v1"

type roleResult struct {
	SchemaVersion             string   `json:"schema_version"`
	RunID                     string   `json:"run_id"`
	Role                      string   `json:"role"`
	Status                    string   `json:"status"`
	TerminalResult            string   `json:"terminal_result"`
	ConfigurationFields       []string `json:"configuration_fields"`
	ObservedFields            []string `json:"observed_fields"`
	TLSVersion                string   `json:"tls_version,omitempty"`
	Curve                     string   `json:"curve,omitempty"`
	CipherSuite               string   `json:"cipher_suite,omitempty"`
	SessionResumed            bool     `json:"session_resumed"`
	ApplicationBytesVerified  bool     `json:"application_bytes_verified"`
	ApplicationBytes          int      `json:"application_bytes"`
	QueueHighWaterBytes       int      `json:"queue_high_water_bytes"`
	StreamElapsedMilliseconds int64    `json:"stream_elapsed_milliseconds,omitempty"`
	ElapsedMilliseconds       int64    `json:"elapsed_milliseconds"`
	HeapAllocBytes            uint64   `json:"heap_alloc_bytes"`
	Goroutines                int      `json:"goroutines"`
	Failure                   string   `json:"failure,omitempty"`
}

func newRoleResult(config roleConfig) roleResult {
	return roleResult{SchemaVersion: nativeEvidenceSchema, RunID: config.RunID, Role: config.Role, Status: "failed", TerminalResult: "explicit_failure", ConfigurationFields: roleConfigurationFields(config)}
}

func (result *roleResult) applyEndpoint(observation endpointObservation) {
	result.TLSVersion = observation.TLSVersion
	result.Curve = observation.Curve
	result.CipherSuite = observation.CipherSuite
	result.SessionResumed = observation.SessionResumed
	result.ApplicationBytesVerified = observation.ApplicationBytesVerified
	result.ApplicationBytes = observation.ApplicationBytes
	result.QueueHighWaterBytes = observation.QueueHighWaterBytes
	result.StreamElapsedMilliseconds = observation.StreamElapsedMilliseconds
}

func (result *roleResult) finish(started time.Time, runErr error) {
	if runErr == nil {
		result.Status = "passed"
		result.TerminalResult = "completed"
	} else {
		result.Failure = runErr.Error()
	}
	slices.Sort(result.ObservedFields)
	result.ObservedFields = slices.Compact(result.ObservedFields)
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	result.ElapsedMilliseconds = time.Since(started).Milliseconds()
	result.HeapAllocBytes = memory.HeapAlloc
	result.Goroutines = runtime.NumGoroutine()
}

func writeRoleReady(directory string, config roleConfig) error {
	return writeRoleMarker(directory, config, "ready.json", "ready")
}

func writeRoleMarker(directory string, config roleConfig, name, status string) error {
	return writeRoleJSON(filepath.Join(directory, name), map[string]string{
		"schema_version": nativeEvidenceSchema, "run_id": config.RunID, "role": config.Role, "status": status,
	})
}

func writeRoleResult(path string, result roleResult) error { return writeRoleJSON(path, result) }

func writeRoleJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > 32*1024*1024 {
		return errors.New("native role evidence exceeds 32 MiB")
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func roleConfigurationFields(config roleConfig) []string {
	fields := []string{"run_id", "role"}
	if config.ListenAddress != "" {
		fields = append(fields, "listen_address")
	}
	if config.CertificatePath != "" {
		fields = append(fields, "node_certificate")
	}
	if config.PrivateKeyPath != "" {
		fields = append(fields, "node_private_key")
	}
	if len(config.AllowedNext) != 0 {
		fields = append(fields, "allowed_adjacent_roles")
	}
	if config.Profile != "" {
		fields = append(fields, "profile")
	}
	if config.Rendezvous != "" {
		fields = append(fields, "rendezvous")
	}
	if config.SlotHex != "" {
		fields = append(fields, "introduction_slot")
	}
	if len(config.IntroductionPath) != 0 {
		fields = append(fields, "introduction_path")
	}
	if len(config.DataPath) != 0 {
		fields = append(fields, "data_path")
	}
	if config.HPKEPublicPath != "" {
		fields = append(fields, "hpke_public_key")
	}
	if config.HPKEPrivatePath != "" {
		fields = append(fields, "hpke_private_key")
	}
	if config.TargetRootPath != "" {
		fields = append(fields, "target_root")
	}
	if config.ExpectedLeafSHA256 != "" {
		fields = append(fields, "expected_instance_leaf")
	}
	if config.EndpointCertificate != "" {
		fields = append(fields, "instance_certificate")
	}
	if config.EndpointPrivateKey != "" {
		fields = append(fields, "instance_private_key")
	}
	if config.PayloadSeed != "" {
		fields = append(fields, "payload_seed")
	}
	if config.StreamDirection != "" {
		fields = append(fields, "stream_direction", "stream_seed", "stream_duration")
	}
	if config.Fault != "" {
		fields = append(fields, "fault")
	}
	if config.DirectAddress != "" {
		fields = append(fields, "direct_address")
	}
	if config.AttachedSocket != "" {
		fields = append(fields, "attached_socket")
	}
	return fields
}

func netFileStat(path string) (bool, error) {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir(), err
}
