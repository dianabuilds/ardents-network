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

func (observer dockerObserver) waitGate(ctx context.Context, root, role string, expected uint32) (uint32, error) {
	path := filepath.Join(root, role+".ready")
	deadline := time.Now().Add(15 * time.Second)
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
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
	return 0, errors.New("host-controlled stream gate was not reached")
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
