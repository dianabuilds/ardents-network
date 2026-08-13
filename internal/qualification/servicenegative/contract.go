package servicenegative

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

// Receipt is the complete mandatory negative-case observation.
type Receipt struct {
	Schema     string                         `json:"schema"`
	Negatives  map[string]bool                `json:"negatives"`
	Mechanisms map[string]string              `json:"mechanisms"`
	Operations map[string]bool                `json:"operations"`
	Classes    map[string]string              `json:"classes"`
	Counts     map[string]uint32              `json:"counts"`
	Recovery   map[string]RecoveryObservation `json:"recovery"`
}

// RecoveryObservation is one externally consumable terminal-negative receipt.
type RecoveryObservation struct {
	TerminalCount         uint32 `json:"terminal_count"`
	EndpointTerminalCount uint32 `json:"endpoint_terminal_count"`
	Class                 string `json:"class"`
	WithinNanos           int64  `json:"within_nanos"`
	Passed                bool   `json:"passed"`
	InjectionKind         string `json:"injection_kind,omitempty"`
	InjectionDigest       string `json:"injection_digest,omitempty"`
	AttackAttempts        uint32 `json:"attack_attempts,omitempty"`
	RecoveryCount         uint32 `json:"recovery_count,omitempty"`
	RouteGeneration       uint64 `json:"route_generation,omitempty"`
}

// Run exercises every distinct negative case and fails if any forbidden action is accepted.
func Run(ctx context.Context, selected ...string) (Receipt, error) {
	fixture, err := newFixture()
	if err != nil {
		return Receipt{}, err
	}
	if len(selected) == 1 {
		return fixture.runRecoveryCase(ctx, selected[0])
	}
	if len(selected) > 1 {
		return Receipt{}, fmt.Errorf("only one isolated recovery case may be selected")
	}
	negatives, mechanisms := fixture.run(ctx)
	operations, classes, counts := fixture.streamObservations(ctx)
	result := Receipt{Schema: "ardents-h3-service-negative-v1", Negatives: negatives, Mechanisms: mechanisms,
		Operations: operations, Classes: classes, Counts: counts, Recovery: map[string]RecoveryObservation{}}
	for name, passed := range result.Negatives {
		if !passed {
			return result, fmt.Errorf("service negative case failed: %s", name)
		}
	}
	for name, passed := range result.Operations {
		if !passed {
			return result, fmt.Errorf("stage 3 stream observation failed: %s", name)
		}
	}
	return result, nil
}

func (value fixture) runRecoveryCase(ctx context.Context, name string) (Receipt, error) {
	mechanisms := map[string]string{
		"recovery-no-alternate": "eligible-candidate-set-empty", "recovery-cancellation": "active-recovery-context-cancel",
		"recovery-deadline": "attachment-opener-deadline", "recovery-forged-attachment": "replacement-tls-proof-rejected",
		"recovery-queue-full":          "finite-service-connection-queue-backpressure",
		"recovery-replayed-attachment": "replayed-continuity-proof",
		"recovery-stale-attachment":    "stale-generation-continuity-proof",
		"recovery-cross-binding":       "cross-binding-continuity-proof",
	}
	mechanism, ok := mechanisms[name]
	if !ok {
		return Receipt{}, fmt.Errorf("unknown isolated recovery case: %s", name)
	}
	var observation RecoveryObservation
	if name == "recovery-queue-full" {
		started := time.Now()
		passed, _ := value.observeRecoveryQueue(ctx)
		observation = logicalRecoveryObservation(passed, "local timeout or cancellation", time.Since(started).Nanoseconds())
	} else if name == "recovery-replayed-attachment" || name == "recovery-stale-attachment" ||
		name == "recovery-cross-binding" {
		observation = value.observeContinuityAttack(ctx, name)
	} else {
		openers := map[string]func(context.Context, serviceconn.Recovery) (net.Conn, error){
			"recovery-no-alternate": unavailableRecovery, "recovery-cancellation": blockedRecovery,
			"recovery-deadline": blockedRecovery, "recovery-forged-attachment": forgedRecovery}
		timeout := time.Second
		if name == "recovery-cancellation" {
			timeout = 0
		}
		if name == "recovery-deadline" {
			timeout = 100 * time.Millisecond
		}
		observation = value.observeRecoveryTerminal(ctx, openers[name], timeout)
	}
	receipt := Receipt{Schema: "ardents-h3-service-negative-v1", Negatives: map[string]bool{name: observation.Passed},
		Mechanisms: map[string]string{name: mechanism}, Operations: map[string]bool{}, Classes: map[string]string{},
		Counts: map[string]uint32{}, Recovery: map[string]RecoveryObservation{name: observation}}
	if !observation.Passed {
		return receipt, fmt.Errorf("service negative case failed: %s", name)
	}
	return receipt, nil
}

func logicalRecoveryObservation(passed bool, class string, elapsed int64) RecoveryObservation {
	value := RecoveryObservation{Class: class, WithinNanos: elapsed, Passed: passed}
	if passed {
		value.TerminalCount, value.EndpointTerminalCount = 1, 2
	}
	return value
}
