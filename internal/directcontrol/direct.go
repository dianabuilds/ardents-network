package directcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

const directRoleSchema = "carrier-lab-direct-role/v1"

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type directRoleConfig struct {
	SchemaVersion      string `json:"schema_version"`
	RunID              string `json:"run_id"`
	Case               string `json:"case"`
	Role               string `json:"role"`
	Address            string `json:"address"`
	CertificatePath    string `json:"certificate_path,omitempty"`
	PrivateKeyPath     string `json:"private_key_path,omitempty"`
	TargetRootPath     string `json:"target_root_path,omitempty"`
	ExpectedLeafSHA256 string `json:"expected_leaf_sha256,omitempty"`
	CanaryHex          string `json:"canary_hex,omitempty"`
	PayloadSeed        string `json:"payload_seed,omitempty"`
	PayloadSize        int    `json:"payload_size,omitempty"`
}

// RunDirectRole executes one fixed Direct TLS tracer role from a data-only
// configuration. The role never receives naming, discovery, or topology data.
func RunDirectRole(ctx context.Context, configPath, evidenceDir string) error {
	started := time.Now()
	config, err := readDirectRoleConfig(configPath)
	if err != nil {
		return err
	}
	if err := validateDirectRoleConfig(config); err != nil {
		return err
	}
	if err := requireDirectDirectory(evidenceDir); err != nil {
		return fmt.Errorf("direct TLS evidence directory: %w", err)
	}
	result := directRoleResult{
		SchemaVersion: directResultSchema, RunID: config.RunID, Case: config.Case, Role: config.Role,
		Status: "failed", TerminalResult: "explicit_failure", DirectRelationshipDisclosed: true, RouteFallback: false,
	}
	observation, runErr := runDirectTLSRole(ctx, config, evidenceDir)
	result.apply(observation)
	if runErr == nil {
		result.Status = "passed"
		result.TerminalResult = "completed"
	} else {
		result.Failure = runErr.Error()
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	result.ElapsedMilliseconds = time.Since(started).Milliseconds()
	result.HeapAllocBytes = memory.HeapAlloc
	result.Goroutines = runtime.NumGoroutine()
	evidenceErr := writeDirectJSON(filepath.Join(evidenceDir, "result.json"), result)
	return errors.Join(runErr, evidenceErr)
}

func readDirectRoleConfig(path string) (directRoleConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return directRoleConfig{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var config directRoleConfig
	if err := decoder.Decode(&config); err != nil {
		return directRoleConfig{}, err
	}
	if config.SchemaVersion != directRoleSchema || !runIDPattern.MatchString(config.RunID) {
		return directRoleConfig{}, errors.New("invalid Direct TLS role schema or run identity")
	}
	if config.Case != "positive" && config.Case != "wrong-instance" && config.Case != "modified-record" {
		return directRoleConfig{}, errors.New("invalid Direct TLS case")
	}
	if config.Role != "user" && config.Role != "service" {
		return directRoleConfig{}, errors.New("invalid Direct TLS role")
	}
	return config, nil
}

func validateDirectRoleConfig(config directRoleConfig) error {
	if config.Address == "" {
		return errors.New("direct TLS role address is required")
	}
	switch config.Role {
	case "service":
		if config.CertificatePath == "" || config.PrivateKeyPath == "" {
			return errors.New("direct TLS service fixture is incomplete")
		}
		if config.TargetRootPath != "" || config.ExpectedLeafSHA256 != "" || config.CanaryHex != "" || config.PayloadSeed != "" || config.PayloadSize != 0 {
			return errors.New("direct TLS service config contains user-only knowledge")
		}
	case "user":
		if config.TargetRootPath == "" || config.ExpectedLeafSHA256 == "" || config.CanaryHex == "" || config.PayloadSeed == "" || config.PayloadSize < 1 || config.PayloadSize > directRecordLimit {
			return errors.New("direct TLS user fixture is incomplete or out of bounds")
		}
		if config.CertificatePath != "" || config.PrivateKeyPath != "" {
			return errors.New("direct TLS user config contains service-only knowledge")
		}
	}
	return nil
}
