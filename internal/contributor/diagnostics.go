package contributor

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type lifecycleEvent struct {
	Schema           string          `json:"schema"`
	Kind             string          `json:"kind"`
	State            string          `json:"state"`
	At               time.Time       `json:"at"`
	Epoch            uint64          `json:"epoch,omitempty"`
	Generation       string          `json:"generation,omitempty"`
	Assignment       string          `json:"assignment,omitempty"`
	CarrierProfile   string          `json:"carrier_profile,omitempty"`
	AssignmentDigest [32]byte        `json:"assignment_digest,omitempty"`
	Reason           string          `json:"reason,omitempty"`
	Resource         json.RawMessage `json:"resource,omitempty"`
}

func (profile *Profile) report(ctx context.Context) (Report, error) {
	record, err := readInstallation(profile.paths.record)
	if err != nil {
		return Report{}, err
	}
	if err := verifyInstalled(profile.paths, record); err != nil {
		return Report{}, err
	}
	lifecycle, err := readLifecycle(profile.paths.lifecycle)
	if err != nil {
		return Report{}, err
	}
	state, err := profile.supervisor.Do(ctx, SupervisorStatus)
	if err != nil {
		return Report{}, err
	}
	return Report{Profile: record.Profile, DeploymentID: record.DeploymentID, Generation: record.Generation,
		ManifestDigest: record.ManifestDigest, ProgramDigest: record.InstalledFiles["ardents-node"],
		LifecycleState: lifecycle.State, Active: state.Active, Enabled: state.Enabled}, nil
}

func (profile *Profile) awaitLifecycle(ctx context.Context, path, want string, timeout time.Duration) (lifecycleEvent, error) {
	deadline := profile.now().Add(timeout)
	for {
		if event, err := readLifecycle(path); err == nil && event.State == want {
			return event, nil
		}
		if !profile.now().Before(deadline) {
			return lifecycleEvent{}, errors.New("contributor lifecycle did not reach " + want)
		}
		if err := profile.wait(ctx, 50*time.Millisecond); err != nil {
			return lifecycleEvent{}, err
		}
	}
}

func readLifecycle(path string) (lifecycleEvent, error) {
	raw, err := readRegular(path, 4096)
	if err != nil {
		return lifecycleEvent{}, err
	}
	var event lifecycleEvent
	if err := decodeStrict(raw, &event); err != nil || event.Schema != "ardents-node-event-v1" || event.Kind != "lifecycle" || event.State == "" {
		return lifecycleEvent{}, errors.New("contributor lifecycle diagnostic is invalid")
	}
	return event, nil
}
