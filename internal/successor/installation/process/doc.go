// Package process retains original kernel process observations of the separately
// fixed installed Endpoint and Source. It owns proc-directory identity, start clock, executable,
// arguments, credentials, cgroup membership and InvocationID observations.
// Retain is root-only predecessor observation; RetainSelf is limited to the
// actual non-root calling process. Kernel supplementary groups may contain only
// its primary group. Expected facts grant no Release or startup authority.
// SourceExpected, RetainSource and RetainSourceSelf choose a separate closed
// node-executable/Source-argv/Source-cgroup policy; Endpoint's Expected/Retain
// contract is unchanged. No public selector generalizes either purpose.
// Installation checks its original lease, bytes and manager around observations;
// cgroup separately owns descendant join. No process stop is performed here.
package process

import "errors"

var errBinding = errors.New("installation process: observation mismatch")
