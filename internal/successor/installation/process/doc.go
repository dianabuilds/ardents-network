// Package process retains the original kernel process observation of the fixed
// installed Endpoint. It owns proc-directory identity, start clock, executable,
// arguments, credentials, cgroup membership and InvocationID observations.
// Retain is root-only predecessor observation; RetainSelf is limited to the
// actual non-root calling process. Kernel supplementary groups may contain only
// its primary group. Expected facts grant no Release or startup authority.
// Installation checks its original lease, bytes and manager around observations;
// cgroup separately owns descendant join. No process stop is performed here.
package process

import "errors"

var errBinding = errors.New("installation process: observation mismatch")
