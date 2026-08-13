package recoverysmoke

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
				return errors.New("container returned an unexpected exit class")
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
