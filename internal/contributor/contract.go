package contributor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

const profileName = "h4-5-rendezvous-alpha-v1"

// SupervisorAction is one fixed system-service operation required by the
// dedicated-host lifecycle.
type SupervisorAction byte

const (
	SupervisorReload SupervisorAction = iota + 1
	SupervisorEnable
	SupervisorStart
	SupervisorRestart
	SupervisorStop
	SupervisorDisable
	SupervisorStatus
)

// SupervisorState is the bounded service-manager result visible to the Module.
type SupervisorState struct {
	Active  bool
	Enabled bool
}

// Action selects one supported operator lifecycle transition.
type Action byte

const (
	Diagnose Action = iota + 1
	Restart
	Drain
	Withdraw
	Remove
)

// Supervisor is the sole external Adapter seam. Production supplies systemd;
// behavior tests supply an in-process Adapter.
type Supervisor interface {
	Do(context.Context, SupervisorAction) (SupervisorState, error)
}

// Config selects one host filesystem root and its supervisor Adapter.
type Config struct {
	Root       string
	Supervisor Supervisor
}

// Profile owns one exact dedicated-host installation.
type Profile struct {
	paths      hostPaths
	supervisor Supervisor
}

// Report contains only non-secret installation and readiness facts.
type Report struct {
	Profile        string `json:"profile"`
	DeploymentID   string `json:"deployment_id"`
	Generation     uint64 `json:"generation"`
	ManifestDigest string `json:"manifest_digest"`
	ProgramDigest  string `json:"program_digest"`
	LifecycleState string `json:"lifecycle_state"`
	Active         bool   `json:"active"`
	Enabled        bool   `json:"enabled"`
}

// Open claims no files and performs no service operation.
func Open(config Config) (*Profile, error) {
	if config.Supervisor == nil || !filepath.IsAbs(config.Root) {
		return nil, errors.New("contributor host configuration is incomplete")
	}
	info, err := os.Stat(config.Root)
	if err != nil || !info.IsDir() {
		return nil, errors.New("contributor host root is unavailable")
	}
	return &Profile{paths: newHostPaths(filepath.Clean(config.Root)), supervisor: config.Supervisor}, nil
}
