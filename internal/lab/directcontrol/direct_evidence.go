package directcontrol

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	directResultSchema = "carrier-lab-direct-result/v1"
	directEvidenceCap  = 64 * 1024
)

type directRoleResult struct {
	SchemaVersion               string `json:"schema_version"`
	RunID                       string `json:"run_id"`
	Case                        string `json:"case"`
	Role                        string `json:"role"`
	Status                      string `json:"status"`
	TerminalResult              string `json:"terminal_result"`
	TLSVersion                  string `json:"tls_version,omitempty"`
	Curve                       string `json:"curve,omitempty"`
	CipherSuite                 string `json:"cipher_suite,omitempty"`
	SessionResumed              bool   `json:"session_resumed"`
	SNI                         string `json:"sni,omitempty"`
	ALPN                        string `json:"alpn,omitempty"`
	CanarySHA256                string `json:"canary_sha256,omitempty"`
	PayloadSHA256               string `json:"payload_sha256,omitempty"`
	PayloadBytes                int    `json:"payload_bytes,omitempty"`
	ApplicationBytesVerified    bool   `json:"application_bytes_verified"`
	ElapsedMilliseconds         int64  `json:"elapsed_milliseconds"`
	HeapAllocBytes              uint64 `json:"heap_alloc_bytes"`
	Goroutines                  int    `json:"goroutines"`
	DirectRelationshipDisclosed bool   `json:"direct_relationship_disclosed"`
	RouteFallback               bool   `json:"route_fallback"`
	Failure                     string `json:"failure,omitempty"`
}

func (result *directRoleResult) apply(observation directObservation) {
	result.TLSVersion = observation.TLSVersion
	result.Curve = observation.Curve
	result.CipherSuite = observation.CipherSuite
	result.SessionResumed = observation.SessionResumed
	result.SNI = observation.SNI
	result.ALPN = observation.ALPN
	result.CanarySHA256 = observation.CanarySHA256
	result.PayloadSHA256 = observation.PayloadSHA256
	result.PayloadBytes = observation.PayloadBytes
	result.ApplicationBytesVerified = observation.ApplicationBytesVerified
}

func requireDirectDirectory(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("path must be absolute and clean")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("path must name a real directory")
	}
	return nil
}

func writeDirectJSON(path string, value any) (runErr error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > directEvidenceCap {
		return errors.New("direct TLS evidence exceeds its fixed cap")
	}
	directory := filepath.Dir(path)
	if err := requireDirectDirectory(directory); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".direct-evidence-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if cleanupErr := os.Remove(temporaryPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			runErr = errors.Join(runErr, cleanupErr)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
