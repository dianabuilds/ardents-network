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
// DescriptorScope separately retains its fixed original kernel scope and events
// through root post-start or independent actual-self observations. Only literal
// fixed-purpose openers reach the shared physical mechanism; ordinary files,
// pathname absence and a replacement empty scope cannot prove original join.
package cgroup

import "errors"

var errBinding = errors.New("installation cgroup observation differs")
