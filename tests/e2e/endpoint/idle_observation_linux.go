//go:build linux && endpoint_launch_native

package endpoint

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
)

// MeasureNET32Idle observes the installed Endpoint and evaluates one idle window.
func MeasureNET32Idle(ctx context.Context, hostingRoot, mainPID, stateRoot string, window time.Duration) (net32IdleVerdict, error) {
	series, report, err := observeNet32Idle(ctx, hostingRoot, mainPID, window)
	if err != nil {
		return net32IdleVerdict{
			Kind: "net32-short-projection", Status: "unavailable", UnavailableReason: err.Error(),
			Samples: report.Samples, FailedAttempts: report.FailedAttempts,
		}, err
	}
	stateBytes, stateErr := net32StateProfileBytes(stateRoot)
	return evaluateNet32Idle(report, series, stateBytes, stateErr), nil
}

// observeNet32Idle reserves the proportional NET-32 ceiling plus separate
// termination capacity in the live shared Hosting budget, then brackets fresh
// counter reads conservatively: the first completion is after every first
// interface read and the last start before every final read, so neither
// provisioning nor the final callback enlarges the monotonic denominator.
func observeNet32Idle(ctx context.Context, hostingRoot, mainPID string, window time.Duration) (series []net32IdleSample, report net32IdleReport, resultErr error) {
	budget, err := hosting.Open(hostingRoot)
	if err != nil {
		return nil, report, err
	}
	defer func() { resultErr = errors.Join(resultErr, budget.Close()) }()
	lifetime, cancel := context.WithTimeout(ctx, window+2*time.Minute)
	defer cancel()
	work, termination := net32IdleTraffic(window)
	reservation, err := budget.Reserve(lifetime, hosting.ReservationRequest{
		Work: work, Termination: termination,
		WorkUntil: time.Now().Add(window + 2*time.Minute), HoldUntil: time.Now().Add(window + 2*time.Minute),
	})
	if err != nil {
		return nil, report, fmt.Errorf("reserve NET-32 idle floor: %w", err)
	}
	defer func() {
		release, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		resultErr = errors.Join(resultErr, reservation.Release(release))
	}()
	var failedAttempts []net32FailedAttempt
	defer func() { report.FailedAttempts = failedAttempts }()
	// Let the Endpoint's initial refresh burst settle so the timed window opens
	// against a sampleable budget; startup lease contention is transient and is
	// excluded from the measured window.
	warmupDeadline := time.Now().Add(30 * time.Second)
	for attempt := 1; ; attempt++ {
		_, sampleErr := budget.Sample(lifetime, time.Second)
		if sampleErr == nil {
			break
		}
		failedAttempts = append(failedAttempts, net32FailedAttempt{Stage: "startup-hosting", Attempt: attempt, Error: sampleErr.Error()})
		if time.Now().After(warmupDeadline) || lifetime.Err() != nil {
			return nil, report, fmt.Errorf("NET-32 budget not sampleable after startup: %w", sampleErr)
		}
		select {
		case <-lifetime.Done():
			return nil, report, lifetime.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	resources, err := openNet32ClientResources(filepath.Join("/proc", mainPID), "/sys/fs/cgroup")
	if err != nil {
		return nil, report, fmt.Errorf("NET-32 client resource scope unavailable: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, resources.close()) }()
	sample := 0
	return collectNet32Idle(lifetime, window, time.Now, func(fresh bool) (net32IdleSample, error) {
		sample++
		s, err := readNet32Sample(lifetime, budget, hostingRoot, fresh, func(attempt int, err error) {
			failedAttempts = append(failedAttempts, net32FailedAttempt{Stage: "window-hosting", Sample: sample, Attempt: attempt, Error: err.Error()})
		})
		if err != nil {
			return net32IdleSample{}, err
		}
		s.resourceBegan = time.Now()
		s.memoryBytes, s.cpuUsec, err = resources.read()
		s.resourceFinished = time.Now()
		if err != nil {
			failedAttempts = append(failedAttempts, net32FailedAttempt{Stage: "client-resources", Sample: sample, Attempt: 1, Error: err.Error()})
			return net32IdleSample{}, fmt.Errorf("NET-32 client resources unavailable: %w", err)
		}
		return s, nil
	})
}

// readNet32Sample forces a fresh charged measurement at either boundary. It
// reads the confirmed persisted counters for the interface/ledger comparison.
func readNet32Sample(ctx context.Context, budget *hosting.Budget, hostingRoot string, fresh bool, failed func(int, error)) (net32IdleSample, error) {
	observation, err := retryNet32Hosting(ctx, func() (hosting.Observation, error) {
		if fresh {
			return budget.Observe(ctx)
		}
		snapshot, err := budget.Sample(ctx, time.Second)
		return snapshot.Observation(), err
	}, failed)
	if err != nil {
		return net32IdleSample{}, err
	}
	if observation.Drain {
		return net32IdleSample{}, errors.New("NET-32 hosting allowance requires drain")
	}
	state, err := readNet32HostingState(hostingRoot)
	if err != nil {
		return net32IdleSample{}, err
	}
	s := net32IdleSample{hostingAt: state.Observed, used: state.Used, boot: state.Reading.Boot}
	for _, iface := range state.Reading.Interfaces {
		s.ifaces = append(s.ifaces, net32IdleInterface{name: iface.Name, index: iface.Index, tx: iface.Tx, rx: iface.Rx})
	}
	return s, nil
}

// The runtime's periodic refresh briefly holds the same per-transact lease.
// A retry is part of the same sample, and each failed call remains in the report.
func retryNet32Hosting(ctx context.Context, observe func() (hosting.Observation, error), failed func(int, error)) (hosting.Observation, error) {
	for attempt := 1; ; attempt++ {
		observation, err := observe()
		if err == nil {
			return observation, nil
		}
		failed(attempt, err)
		if attempt == 5 || ctx.Err() != nil {
			return hosting.Observation{}, err
		}
		select {
		case <-ctx.Done():
			return hosting.Observation{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

type net32HostingState struct {
	Schema   string    `json:"schema"`
	Observed time.Time `json:"observed"`
	Used     uint64    `json:"used"`
	Reserved uint64    `json:"reserved"`
	Reading  struct {
		Boot       string `json:"boot"`
		Interfaces []struct {
			Name  string `json:"name"`
			Index uint64 `json:"index"`
			Tx    uint64 `json:"tx"`
			Rx    uint64 `json:"rx"`
		} `json:"interfaces"`
	} `json:"reading"`
}

func readNet32HostingState(hostingRoot string) (net32HostingState, error) {
	raw, err := os.ReadFile(filepath.Join(hostingRoot, "budget.json"))
	if err != nil {
		return net32HostingState{}, err
	}
	// budget.json is the Hosting owner's checksummed envelope {state, sha256}.
	var envelope struct {
		State  json.RawMessage `json:"state"`
		SHA256 string          `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return net32HostingState{}, err
	}
	digest := sha256.Sum256(envelope.State)
	if envelope.SHA256 != hex.EncodeToString(digest[:]) {
		return net32HostingState{}, errors.New("NET-32 hosting state checksum differs")
	}
	var state net32HostingState
	if err := json.Unmarshal(envelope.State, &state); err != nil {
		return net32HostingState{}, err
	}
	if state.Schema != "ardents-hosting-budget-v1" || state.Observed.IsZero() || state.Reading.Boot == "" || len(state.Reading.Interfaces) == 0 {
		return net32HostingState{}, errors.New("NET-32 hosting reading unavailable")
	}
	return state, nil
}

func net32StateProfileBytes(root string) (uint64, error) {
	var total uint64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("NET-32 State profile contains a non-regular entry")
		}
		if info.Mode().IsRegular() {
			if info.Size() < 0 || uint64(info.Size()) > (64<<10)-total {
				return errors.New("NET-32 State profile exceeds 64 KiB")
			}
			total += uint64(info.Size())
		}
		return nil
	})
	return total, err
}
