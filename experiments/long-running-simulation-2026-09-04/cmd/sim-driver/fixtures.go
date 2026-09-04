//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// artifactsDir is the directory inside the compose containers that
// holds the pre-compiled ardents, ardents-node, and test-driver
// binaries. The bind-mount in docker-compose.yml maps it from the
// host evidence dir's artifacts/ subdirectory. Mirrors the
// artifactsDir constant in the slice 2 pilot's prebake.go; the
// duplicated constant is intentional because the slice 2
// test-driver is a build-ignored standalone Go binary and is not
// importable from a sibling build-ignored binary.
const artifactsDir = "/workspace/artifacts"

// EnsureSourceState is a defensive check that the slice 2 pilot's
// `test-driver prebake` has already populated the evidence dir's
// fixtures and source-a state root. S3.1's docker-compose runs the
// prebake as a separate one-shot service before the sim-driver
// service starts; this function is the sim-driver's last-line
// guardrail, NOT a substitute for the compose-level prebake
// service.
//
// If the prebake artefacts are missing, EnsureSourceState shells
// out to the slice 2 test-driver binary (which is in the same
// bind-mounted artifacts dir as ardents and ardents-node) and
// runs `prebake` against the supplied evidence dir. This is the
// "shell out" path the S3.1 contract calls out; the fallback path
// (a minimal Go re-implementation of the prebake) is documented
// in the code comment below but not implemented for S3.1.
//
// Returns nil if the prebake artefacts already exist; otherwise
// runs the prebake and returns the underlying exec / exit error.
func EnsureSourceState(evidenceDir string) error {
	if prebakePresent(evidenceDir) {
		return nil
	}
	testDriver := filepath.Join(artifactsDir, "test-driver-linux-amd64")
	if _, err := os.Stat(testDriver); err != nil {
		// Fallback path (NOT IMPLEMENTED for S3.1): the sim-driver
		// would build a minimal Go re-implementation of the slice
		// 2 prebake here, writing the source-a state root and
		// fixtures/current directly. The slice 2 fixtures.go +
		// record.go + epoch.go + encoding.go + sourceplan.go +
		// prebake.go together are ~700 lines of prebake; lifting
		// them into the sim-driver would violate the
		// "one new capability, one new file" per-slice rule and
		// is deferred to S3.4 if a Carrier Lab without the
		// test-driver binary becomes a real need.
		return fmt.Errorf("sim-driver: prebake artefacts missing and test-driver binary not found at %s; the docker-compose prebake service must run before the sim-driver service: %w", testDriver, err)
	}
	command := exec.Command(testDriver, "prebake", evidenceDir)
	output := &bytes.Buffer{}
	command.Stdout = output
	command.Stderr = output
	if err := command.Run(); err != nil {
		return fmt.Errorf("sim-driver: ensure-source-state exec %s prebake %s: %w (output: %s)",
			testDriver, evidenceDir, err, output.String())
	}
	if !prebakePresent(evidenceDir) {
		return errors.New("sim-driver: ensure-source-state ran prebake but fixtures/current is still missing")
	}
	return nil
}

// prebakePresent reports whether the slice 2 prebake has produced
// the artefacts the sim-driver depends on. The prebake writes
// fixtures/current (the generation string), fixtures/source-a.json
// (the source server plan), and source-a-state/ (the populated
// state root, marked with the production .ardents-network-state-v1
// marker file) into the evidence dir. All three must exist for the
// tick loop to start.
//
// The state root marker filename (.ardents-network-state-v1) is
// the production marker the maintained state store writes after a
// successful accept-offline; it is a dotfile whose contents are
// the schema id. The sim-driver does not parse the contents; it
// only checks for the file's existence, which is the same check
// the production source server performs on open.
func prebakePresent(evidenceDir string) bool {
	for _, rel := range []string{
		filepath.Join("fixtures", "current"),
		filepath.Join("fixtures", "source-a.json"),
		filepath.Join("source-a-state", ".ardents-network-state-v1"),
	} {
		if _, err := os.Stat(filepath.Join(evidenceDir, rel)); err != nil {
			return false
		}
	}
	return true
}

// WritePerTickPlan copies the shared fixtures/client.json source
// plan to dst, rewriting the local_role_state_root field to point
// at the per-tick local-roles directory. The production state
// store refuses to claim a non-empty unowned state root, so the
// per-tick state root MUST be empty when refresh-sources opens
// it; rewriting the plan's local_role_state_root is the only way
// to keep a single shared client.json while giving each tick its
// own local-roles lease.
//
// The production source client also requires the plan's sources
// list to contain EXACTLY 2 entries
// (cmd/ardents/source_plan.go rejects "len(plan.Sources) != 2"
// with "source plan is not canonical or complete"); the slice 2
// prebake always writes both source-a and source-b into the
// shared plan, and S3.1 starts both as honest containers, so
// the per-tick plan inherits the full sources list untouched.
//
// dst is overwritten if it exists. The shared plan is read
// fresh on every call so a hot-edit to the shared plan (e.g. for
// later slices that add a DriftInjector) is picked up without
// restarting the sim-driver.
func WritePerTickPlan(sharedPlan, dst, tickStateRoot string) error {
	if sharedPlan == "" || dst == "" || tickStateRoot == "" {
		return errors.New("sim-driver: write-per-tick-plan arguments are empty")
	}
	raw, err := os.ReadFile(sharedPlan)
	if err != nil {
		return fmt.Errorf("sim-driver: read shared plan %s: %w", sharedPlan, err)
	}
	var plan map[string]any
	if err := json.Unmarshal(raw, &plan); err != nil {
		return fmt.Errorf("sim-driver: parse shared plan: %w", err)
	}
	plan["local_role_state_root"] = filepath.Join(tickStateRoot, "local-roles")
	marshaled, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("sim-driver: marshal per-tick plan: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("sim-driver: mkdir plan dir: %w", err)
	}
	return os.WriteFile(dst, append(marshaled, '\n'), 0o600)
}

// RunRefreshOnce invokes the maintained ardents refresh-sources
// binary against one per-tick state root and source plan. The
// function returns the combined stdout+stderr, the exit code, and
// any Go error from exec.Command.Run. The state root is created
// (mkdir -p) before the call so the production path's
// "refusing to claim a non-empty unowned state root" check sees
// a fresh, empty directory.
func RunRefreshOnce(stateRoot, planPath string) ([]byte, int, error) {
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		return nil, -1, fmt.Errorf("sim-driver: mkdir tick state root: %w", err)
	}
	ardents := filepath.Join(artifactsDir, "ardents-linux-amd64")
	command := exec.Command(ardents, "refresh-sources", "--once",
		"--state-root", stateRoot, "--source-plan", planPath)
	output := &bytes.Buffer{}
	command.Stdout = output
	command.Stderr = output
	err := command.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return output.Bytes(), -1, err
		}
	}
	return output.Bytes(), exitCode, nil
}

// copyPrebakedState seeds the consumer state root with the marker
// file and the generations/ directory from a prebaked source state
// root. This lets the FIRST tick of the run skip the production
// accept-offline step (which is the dominant per-tick cost). The
// lock file (.ardents-network-state-lock) is intentionally NOT
// copied: it is a runtime artefact written on open and would
// conflict between the source and the consumer if shared.
func copyPrebakedState(dst, src string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("sim-driver: read prebaked state root %s: %w", src, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		// Skip the runtime lock file; copy everything else
		// (marker + current + generations + local-roles).
		if name == ".ardents-network-state-lock" {
			continue
		}
		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)
		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0o700); err != nil {
				return fmt.Errorf("sim-driver: mkdir %s: %w", dstPath, err)
			}
			if err := copyDirRecursive(dstPath, srcPath); err != nil {
				return fmt.Errorf("sim-driver: recurse into %s: %w", srcPath, err)
			}
			continue
		}
		raw, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("sim-driver: read %s: %w", srcPath, err)
		}
		if err := os.WriteFile(dstPath, raw, 0o600); err != nil {
			return fmt.Errorf("sim-driver: write %s: %w", dstPath, err)
		}
	}
	return nil
}

// copyDirRecursive recursively copies a directory tree from src to
// dst. The dst directory must already exist. Symlinks are not
// followed (the production state root never uses symlinks, so the
// check is defensive). Used by copyPrebakedState to seed the
// generations/ subtree of the consumer state root from the
// prebaked source state root.
func copyDirRecursive(dst, src string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("sim-driver: read %s: %w", src, err)
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("sim-driver: refusing to follow symlink %s", filepath.Join(src, entry.Name()))
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0o700); err != nil {
				return fmt.Errorf("sim-driver: mkdir %s: %w", dstPath, err)
			}
			if err := copyDirRecursive(dstPath, srcPath); err != nil {
				return err
			}
			continue
		}
		raw, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("sim-driver: read %s: %w", srcPath, err)
		}
		if err := os.WriteFile(dstPath, raw, 0o600); err != nil {
			return fmt.Errorf("sim-driver: write %s: %w", dstPath, err)
		}
	}
	return nil
}

// SourceWaveEvent is the single JSON event the source-client CLI
// emits on every accepted wave. The shape is byte-compatible with
// the maintained type in the slice 2 pilot's convergence.go; the
// type is duplicated here because the slice 2 test-driver is
// build-ignored and not importable.
type SourceWaveEvent struct {
	Schema             string    `json:"schema"`
	Kind               string    `json:"kind"`
	Generation         string    `json:"generation"`
	Epoch              uint64    `json:"epoch"`
	SourceAttempts     uint16    `json:"source_attempts"`
	SourceOutcomes     [4]string `json:"source_outcomes"`
	LatestCompleteness string    `json:"latest_completeness"`
}

// ReadSourceWaveEventFromBytes extracts exactly one source-wave-
// accepted event from an in-memory capture. The reader is strict:
// a missing event, more than one event, or a malformed JSON line
// are all errors. The capture is expected to be a sequence of
// lines, some of which are JSON events; non-JSON log lines are
// skipped.
func ReadSourceWaveEventFromBytes(data []byte) (SourceWaveEvent, error) {
	reader := bufio.NewReader(bytes.NewReader(data))
	var found *SourceWaveEvent
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimRight(line, "\r\n\t ")
			if len(trimmed) > 0 {
				var event SourceWaveEvent
				if jsonErr := json.Unmarshal(trimmed, &event); jsonErr == nil {
					if event.Schema == "ardents-source-event-v1" && event.Kind == "source-wave-accepted" {
						if found != nil {
							return SourceWaveEvent{}, errors.New("sim-driver: more than one source-wave-accepted event in consumer output")
						}
						ev := event
						found = &ev
					}
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return SourceWaveEvent{}, fmt.Errorf("sim-driver: read consumer output: %w", err)
		}
	}
	if found == nil {
		return SourceWaveEvent{}, errors.New("sim-driver: no source-wave-accepted event in consumer output")
	}
	return *found, nil
}
