// Package cgroup observes bounded actual kernel scope inventories. The
// Endpoint platform verifies an empty inventory before signed replacement.
// Fixed-role repairs separately check quiescence. These observations grant
// no process-management or software authorization.
// The transitional Forwarding scope orchestration (ForwardingScope,
// CheckForwardingQuiescent and the RetainForwarding* openers) is retired
// (ADR-0125, #604 FJD stage D); the platform-led Forwarding lifecycle owns
// attempt, process/manager matching and stopping. CheckForwardingRecoveryQuiescent
// remains the separate read-only per-unit kernel observation for explicit
// Forwarding participation-producer abandonment, consumed by the retained
// platform repair. The transitional JOIN scope orchestration (JoinScope,
// CheckJoinQuiescent and the RetainJoin* openers) is retired (ADR-0125, #604
// FJD stage D); the platform-led JOIN lifecycle owns attempt, process/manager
// matching and stopping. CheckJoinRecoveryQuiescent remains the separate
// read-only per-unit kernel observation for explicit JOIN participation-producer
// abandonment, consumed by the retained platform repair.
// CheckSourceRecoveryQuiescent is the separate read-only kernel observation for
// explicit Source producer abandonment; the transitional Source scope
// orchestration is retired (ADR-0125, #604).
// The transitional Descriptor scope orchestration (DescriptorScope,
// CheckDescriptorQuiescent and the RetainDescriptor* openers) is retired
// (ADR-0125, #604 FJD stage D); the platform-led Descriptor lifecycle owns
// attempt, process/manager matching and stopping. CheckDescriptorRecoveryQuiescent
// remains the separate read-only per-unit kernel observation for explicit
// Descriptor participation-producer abandonment, consumed by the retained
// platform repair.
package cgroup

import "errors"

var errBinding = errors.New("installation cgroup observation differs")
