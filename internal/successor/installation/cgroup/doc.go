// Package cgroup owns bounded Installation kernel-scope inventory and original
// cgroup2 event-descriptor lifetimes. Kernel filesystem and descriptor checks
// prevent pathname reuse or ordinary files from supplying physical completion.
// Installation predecessor, candidate and recovery owners consume its retained
// observations. RetainStarted preserves partial original custody after a start
// effect; pending-start PID zero cannot match any worker. The caller joins every
// retained original descriptor before releasing that failed attempt's lease;
// it grants no Release, manager, process identity, stop or startup authority.
package cgroup

import "errors"

var errBinding = errors.New("installation cgroup observation differs")
