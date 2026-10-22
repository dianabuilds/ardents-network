// Package cgroup owns bounded Installation kernel-scope inventory and original
// cgroup2 event-descriptor lifetimes. Kernel filesystem and descriptor checks
// prevent pathname reuse or ordinary files from supplying physical completion.
// Installation predecessor, candidate and recovery owners consume its retained
// observations. RetainStarted preserves partial original custody after a start
// effect; pending-start PID zero cannot match any worker. The caller joins every
// retained original descriptor before releasing that failed attempt's lease;
// it grants no Release, manager, process identity, stop or startup authority.
// SourceScope separately retains the fixed Source directory and original events
// object, observes one original PID with no child scopes, and reports actual
// original join independently of caller cancellation. Root's post-attempt
// partial observation and non-root actual-self observation are distinct openers;
// neither widens the Endpoint/worker scope inventory. Copies share one original
// Source observation and physical Close result. Installation owns attempt,
// process/manager matching, stopping and when to release these observations.
package cgroup

import "errors"

var errBinding = errors.New("installation cgroup observation differs")
