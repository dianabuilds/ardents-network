// Package cgroup owns bounded Installation kernel-scope inventory and original
// cgroup2 event-descriptor lifetimes. Kernel filesystem and descriptor checks
// prevent pathname reuse or ordinary files from supplying physical completion.
// Installation predecessor, candidate and recovery owners consume its retained
// observations. RetainStarted preserves partial original custody after a start
// effect; pending-start PID zero cannot match any worker. The caller joins every
// retained original descriptor before releasing that failed attempt's lease;
// it grants no Release, manager, process identity, stop or startup authority.
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
// platform repair. The shared processScope/retainProcessScope start/stop kernel
// custody used by the retired Retain{Source,Forwarding,Join,Descriptor}
// invocations is removed; RetainStarted partial post-start custody and the
// fixed-scope quiescence mechanism remain for Endpoint and the retained repair.
package cgroup

import "errors"

var errBinding = errors.New("installation cgroup observation differs")
