package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
	"github.com/dianabuilds/ardents-network/internal/qualification/servicenegative"
)

func (observer dockerObserver) recoveryNegatives(ctx context.Context) (map[string]recovery.Negative, error) {
	_, _ = observer.compose(ctx, time.Minute, "down", "-v", "--remove-orphans")
	mapping := map[string]string{
		"no-alternate": "recovery-no-alternate", "cancellation": "recovery-cancellation",
		"deadline": "recovery-deadline", "forged-attachment": "recovery-forged-attachment",
		"replayed-attachment": "recovery-replayed-attachment", "stale-attachment": "recovery-stale-attachment",
		"cross-binding": "recovery-cross-binding",
		"queue-full":    "recovery-queue-full", "endpoint-restart": "recovery-endpoint-restart",
	}
	result := make(map[string]recovery.Negative, len(mapping))
	receipts := make(map[string]servicenegative.Receipt, len(mapping))
	for name, source := range mapping {
		if name == "endpoint-restart" {
			observed, err := observer.endpointRestartNegative(ctx)
			if err != nil {
				return nil, err
			}
			result[name] = observed
			continue
		}
		raw, containerID, elapsed, err := observer.runIsolatedNegative(ctx, name, source)
		if err != nil {
			return nil, err
		}
		var receipt servicenegative.Receipt
		for _, line := range splitLines(raw) {
			if json.Unmarshal(line, &receipt) == nil && receipt.Schema != "" {
				break
			}
		}
		if receipt.Schema == "" {
			return nil, errors.New("isolated recovery negative receipt is missing: " + source)
		}
		observed, ok := receipt.Recovery[source]
		if !ok {
			return nil, errors.New("structured recovery negative observation is missing: " + source)
		}
		result[name] = recovery.Negative{TerminalCount: observed.TerminalCount, Class: observed.Class,
			WithinNanos: elapsed, Passed: observed.Passed && elapsed <= int64(15*time.Second), ContainerID: containerID,
			InjectionKind: observed.InjectionKind, InjectionDigest: observed.InjectionDigest,
			AttackAttempts: observed.AttackAttempts, RecoveryCount: observed.RecoveryCount,
			RouteGeneration: observed.RouteGeneration}
		receipts[name] = receipt
	}
	if err := writeRecoveryReceipt(observer.input.EvidenceRoot, receipts); err != nil {
		return nil, err
	}
	return result, nil
}

func (observer dockerObserver) runIsolatedNegative(ctx context.Context, name, source string) ([]byte, string, int64, error) {
	raw, err := observer.docker(ctx, time.Minute, "create", "--network", "none", "--read-only", "--cap-drop", "ALL",
		"--security-opt", "no-new-privileges", "--user", "65532:65532", "--memory", "128m", "--pids-limit", "32",
		"--label", "com.docker.compose.project="+observer.project, observer.imageID,
		"/usr/local/bin/ardents-service-negative", source)
	containerID := containerIDFromOutput(raw)
	if err != nil || !validContainerID(containerID) {
		return nil, "", 0, errors.Join(err, errors.New("isolated negative container identity is invalid"))
	}
	defer func() { _, _ = observer.docker(context.Background(), time.Minute, "rm", "-f", containerID) }()
	started := time.Now()
	if _, err := observer.docker(ctx, time.Minute, "start", containerID); err != nil {
		return nil, containerID, 0, err
	}
	wait, err := observer.docker(ctx, 15*time.Second, "wait", containerID)
	elapsed := time.Since(started).Nanoseconds()
	logs, logErr := observer.docker(ctx, time.Minute, "logs", containerID)
	if err != nil || logErr != nil || strings.TrimSpace(string(wait)) != "0" {
		return logs, containerID, elapsed, errors.Join(err, logErr, errors.New("isolated negative container failed"))
	}
	return logs, containerID, elapsed, nil
}
