package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (observer dockerObserver) waitProgress(ctx context.Context, service string, minimum uint32) (uint32, error) {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		highest, _ := observer.currentProgress(ctx, service)
		if highest >= minimum && highest%16_384 != 0 {
			return highest, nil
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(2 * time.Millisecond):
		}
	}
	return 0, errors.New("receiver progress did not reach the seeded fault threshold")
}

func (observer dockerObserver) waitGate(ctx context.Context, root, role string, expected uint32,
	within time.Duration) (uint32, error) {
	return observer.waitGateFile(ctx, filepath.Join(root, role+".ready"), expected, within)
}

func (observer dockerObserver) waitSequentialGate(ctx context.Context, root, role string,
	expected uint32, within time.Duration) (uint32, error) {
	return observer.waitGateFile(ctx, filepath.Join(root, role+".ready."+strconv.FormatUint(uint64(expected), 10)),
		expected, within)
}

func resetSequentialGates(root string, offsets []uint32) error {
	var paths []string
	for _, role := range []string{"client", "publisher"} {
		for _, offset := range offsets {
			suffix := strconv.FormatUint(uint64(offset), 10)
			ready := filepath.Join(root, role+".ready."+suffix)
			paths = append(paths, ready, ready+".pending", filepath.Join(root, role+".release."+suffix))
		}
	}
	return removeHostGateState(paths)
}

func resetRecoveryGates(root string) error {
	var paths []string
	for _, role := range []string{"client", "publisher"} {
		ready := filepath.Join(root, role+".ready")
		paths = append(paths, ready, ready+".pending", filepath.Join(root, role+".release"))
	}
	return removeHostGateState(paths)
}

func removeHostGateState(paths []string) error {
	var result error
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, fmt.Errorf("remove prior host-controlled gate state %q: %w", path, err))
		}
	}
	return result
}

func (observer dockerObserver) waitGateFile(ctx context.Context, path string, expected uint32,
	within time.Duration) (uint32, error) {
	if within <= 0 || within > 4*time.Minute {
		return 0, errors.New("host-controlled stream gate wait is outside its bound")
	}
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			value, parseErr := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 32)
			if parseErr != nil {
				return 0, fmt.Errorf("decode host-controlled stream gate offset: %w", parseErr)
			}
			if uint32(value) != expected {
				return 0, fmt.Errorf("host-controlled stream gate reported offset %d; want %d", value, expected)
			}
			return uint32(value), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return 0, fmt.Errorf("read host-controlled stream gate %q: %w", path, err)
		}
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("wait for host-controlled stream gate %q offset %d: %w", path, expected, ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	return 0, fmt.Errorf("host-controlled stream gate %q offset %d was not reached within %s", path, expected, within)
}

func pacedGateWait(previous, next uint32, delayText string) (time.Duration, error) {
	const chunk = uint32(16_381)
	if next <= previous || next%chunk != 0 || previous%chunk != 0 {
		return 0, errors.New("stream gate offsets are not an increasing whole-chunk schedule")
	}
	delay, err := time.ParseDuration(delayText)
	if err != nil || delay <= 0 {
		return 0, errors.Join(err, errors.New("stream gate chunk delay is invalid"))
	}
	within := time.Duration((next-previous)/chunk)*delay + 30*time.Second
	if within > 4*time.Minute {
		return 0, errors.New("stream gate wait exceeds four minutes")
	}
	return within, nil
}

func (observer dockerObserver) currentProgress(ctx context.Context, service string) (uint32, error) {
	raw, err := observer.compose(ctx, 10*time.Second, "logs", "--no-color", "--no-log-prefix", service)
	if err != nil {
		return 0, err
	}
	var highest uint32
	for _, line := range splitLines(raw) {
		var value struct {
			Schema        string `json:"schema"`
			ReceivedBytes uint32 `json:"received_bytes"`
		}
		if json.Unmarshal(line, &value) == nil && value.Schema == "ardents-h3-stream-progress-v1" && value.ReceivedBytes > highest {
			highest = value.ReceivedBytes
		}
	}
	return highest, nil
}
