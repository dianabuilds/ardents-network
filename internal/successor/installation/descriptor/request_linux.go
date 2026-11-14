package descriptor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	ErrInput   = errors.New("descriptor installation: invalid input")
	ErrBinding = errors.New("descriptor installation: binding differs")
)

// Request declares original delivery inputs and five independent semantic roots.
// It supplies no account adoption, resource creation or startup permission. The
// provider's one Hosting root remains fixed across later admitted invocations.
type Request struct {
	Schema              string `json:"schema"`
	Invocation          string `json:"invocation"`
	Profile             string `json:"profile"`
	InstallationRoot    string `json:"installation_root"`
	StateRoot           string `json:"state_root"`
	ParticipationRoot   string `json:"participation_root"`
	SpendRoot           string `json:"spend_root"`
	HostingRoot         string `json:"hosting_root"`
	DescriptorRoot      string `json:"descriptor_root"`
	CredentialDirectory string `json:"credential_directory"`
	Plan                string `json:"plan"`
	Certificate         string `json:"certificate"`
	PrivateKey          string `json:"private_key"`
	Clock               string `json:"clock"`
}

// The finite read namespace is an Installation bound, not a network retry or
// another allowance. History paths are never recycled on exhaustion.
const attemptLimit = 64

func DecodeRequest(raw []byte) (Request, error) {
	var r Request
	if len(raw) == 0 || len(raw) > 16<<10 || json.Unmarshal(raw, &r) != nil {
		return Request{}, ErrInput
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || r.Schema != "ardents-descriptor-installation-v1" || r.Invocation != "descriptor" || r.Profile != "ardents-descriptor-process-v1" {
		return Request{}, ErrInput
	}
	paths := r.paths()
	for i, path := range paths {
		if !validPath(path) {
			return Request{}, ErrInput
		}
		for _, previous := range paths[:i] {
			if contains(path, previous) || contains(previous, path) {
				return Request{}, ErrBinding
			}
		}
	}
	for _, reserved := range r.reservedSiblingPaths() {
		if !validPath(reserved) {
			return Request{}, ErrBinding
		}
		for _, path := range paths {
			if contains(reserved, path) || contains(path, reserved) {
				return Request{}, ErrBinding
			}
		}
	}
	for _, path := range []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.DescriptorRoot, r.CredentialDirectory, r.Clock} {
		for _, hidden := range []string{"/tmp", "/var/tmp", "/home", "/root", "/run/user"} {
			if contains(hidden, path) {
				return Request{}, ErrBinding
			}
		}
	}
	// Linux pathname sockets need the final NUL in their 108-byte sun_path.
	// Check the entire retained namespace before any installation effect.
	for n := 0; n < attemptLimit; n++ {
		start := filepath.Join(r.InstallationRoot, fmt.Sprintf("descriptor-invocation-%08d", n), "start-completion.socket")
		if n == 0 {
			start = filepath.Join(r.InstallationRoot, "descriptor-startup", "start-completion.socket")
		}
		stop := filepath.Join(r.InstallationRoot, fmt.Sprintf("descriptor-retirement-%08d", n), "stop-completion.socket")
		if len(start) > 107 || len(stop) > 107 {
			return Request{}, ErrInput
		}
	}
	return r, nil
}

func (r Request) paths() []string {
	return []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.DescriptorRoot, r.CredentialDirectory, r.Plan, r.Certificate, r.PrivateKey, r.Clock}
}

func (r Request) reservedSiblingPaths() []string {
	paths := []string{r.InstallationRoot + ".descriptor-initial-journal", r.InstallationRoot + ".descriptor-initial-pending", r.InstallationRoot + ".descriptor-hosting-journal", r.InstallationRoot + ".descriptor-hosting-pending", r.InstallationRoot + ".descriptor-hosting", r.InstallationRoot + ".descriptor-first-start-journal", r.InstallationRoot + ".descriptor-replacement-pending"}
	for n := 0; n < attemptLimit; n++ {
		if n != 0 {
			paths = append(paths, r.InstallationRoot+fmt.Sprintf(".descriptor-invocation-%08d-journal", n))
		}
		paths = append(paths, r.InstallationRoot+fmt.Sprintf(".descriptor-retirement-%08d-journal", n))
	}
	return paths
}

// CheckPreparationOutput keeps an exclusive public output outside every original
// input, semantic root and retained journal namespace. It performs no native I/O.
func (r Request) CheckPreparationOutput(output string) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if _, err := DecodeRequest(append(raw, '\n')); err != nil {
		return err
	}
	if !validPath(output) {
		return ErrInput
	}
	for _, path := range append(r.paths(), r.reservedSiblingPaths()...) {
		if contains(output, path) || contains(path, output) {
			return ErrBinding
		}
	}
	return nil
}

func validPath(path string) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" || len(path) > 4096 || len(strings.Split(path, "/")) > 128 {
		return false
	}
	for _, c := range path {
		if c < 32 || c > 126 || strings.ContainsRune("%\\\"'*?[]{}$`", c) {
			return false
		}
	}
	return true
}

func contains(parent, child string) bool {
	return parent == child || strings.HasPrefix(child, parent+"/")
}
