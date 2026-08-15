package servicesmoke

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (observer dockerObserver) waitReady(ctx context.Context, service string) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		raw, _ := observer.compose(ctx, 10*time.Second, "logs", "--no-color", "--no-log-prefix", service)
		if bytes.Contains(raw, []byte(`"kind":"ready"`)) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New(service + " did not become ready within 20s")
}

func (observer dockerObserver) runDetached(ctx context.Context, service string) (string, error) {
	raw, err := observer.compose(ctx, time.Minute, "run", "-d", "--no-deps", service)
	identity := containerIDFromOutput(raw)
	if err != nil || !validContainerID(identity) {
		return "", errors.New(service + " container identity is invalid")
	}
	return identity, nil
}

func containerIDFromOutput(raw []byte) string {
	lines := strings.Fields(string(raw))
	for index := len(lines) - 1; index >= 0; index-- {
		if validContainerID(lines[index]) {
			return lines[index]
		}
	}
	return ""
}

func (observer dockerObserver) serviceID(ctx context.Context, service string) (string, error) {
	raw, err := observer.compose(ctx, time.Minute, "ps", "-a", "-q", service)
	identity := strings.TrimSpace(string(raw))
	if err != nil || !validContainerID(identity) {
		return "", errors.New(service + " container identity is invalid")
	}
	return identity, nil
}

func (observer dockerObserver) waitContainer(ctx context.Context, identity string, success bool) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := observer.docker(ctx, 10*time.Second, "inspect", "--format", "{{json .State}}", identity)
		if err == nil {
			var state struct {
				Running  bool `json:"Running"`
				ExitCode int  `json:"ExitCode"`
			}
			if json.Unmarshal(bytes.TrimSpace(raw), &state) == nil && !state.Running {
				if (state.ExitCode == 0) == success {
					return nil
				}
				logs, _ := observer.docker(ctx, 10*time.Second, "logs", "--tail", "20", identity)
				if len(logs) > 4<<10 {
					logs = logs[len(logs)-(4<<10):]
				}
				return fmt.Errorf("container %s returned exit code %d, expected success=%t: %s",
					identity, state.ExitCode, success, strings.TrimSpace(string(logs)))
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("container did not terminate within 30s")
}

func (observer dockerObserver) hostileObservation(ctx context.Context, identity string) (hostileObservation, error) {
	raw, err := observer.docker(ctx, 10*time.Second, "inspect", "--format", "{{json .State}}", identity)
	if err != nil {
		return hostileObservation{}, err
	}
	var state struct {
		Running  bool `json:"Running"`
		ExitCode int  `json:"ExitCode"`
	}
	if json.Unmarshal(bytes.TrimSpace(raw), &state) != nil {
		return hostileObservation{}, errors.New("hostile sibling state is malformed")
	}
	raw, err = observer.docker(ctx, 10*time.Second, "inspect", "--format", "{{json .Mounts}}", identity)
	if err != nil {
		return hostileObservation{}, err
	}
	var mounts []struct {
		Destination string `json:"Destination"`
	}
	if json.Unmarshal(bytes.TrimSpace(raw), &mounts) != nil {
		return hostileObservation{}, errors.New("hostile sibling mounts are malformed")
	}
	result := hostileObservation{RuntimeID: identity, Running: state.Running, ExitCode: state.ExitCode,
		MountDestinations: make([]string, 0, len(mounts))}
	for _, mount := range mounts {
		result.MountDestinations = append(result.MountDestinations, mount.Destination)
	}
	output, err := observer.docker(ctx, 10*time.Second, "logs", identity)
	if err != nil || len(output) > 4<<10 {
		return hostileObservation{}, errors.New("hostile sibling output is unavailable or oversized")
	}
	result.Output = string(output)
	return result, nil
}

func validContainerID(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}
