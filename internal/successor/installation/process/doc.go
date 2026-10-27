// Package process retains original kernel process observations of the separately
// fixed installed Endpoint, Source and Forwarding. It owns proc-directory identity, start clock, executable,
// arguments, credentials, cgroup membership and InvocationID observations.
// Retain is root-only predecessor observation; RetainSelf is limited to the
// actual non-root calling process. Kernel supplementary groups may contain only
// its primary group. Expected facts grant no Release or startup authority.
// SourceExpected, RetainSource and RetainSourceSelf choose a separate closed
// node-executable/Source-argv/Source-cgroup policy; Endpoint's Expected/Retain
// contract is unchanged. No public selector generalizes either purpose.
// Installation checks its original lease, bytes and manager around observations;
// cgroup separately owns descendant join. Root Source observation additionally
// retains its original pidfd; InterruptSource performs only one original
// SIGTERM, after Installation's independent explicit-stop admission. It grants
// no manager, startup, retirement or replacement permission.
// ForwardingExpected and its root/self openers choose only the Forwarding argv
// and unit. ForwardingInvocation hides the underlying Source-capable type,
// retains its own root pidfd and shares permanent refusal/joined Close across
// copies. InterruptForwarding supplies only an original physical SIGTERM;
// Installation must separately establish stop admission and original domain join.
// RetainForwardingStop additionally compares the archived original proc inode
// after fresh fixed-purpose admission; copied archive facts grant no signal right.
package process

import "errors"

var errBinding = errors.New("installation process: observation mismatch")
