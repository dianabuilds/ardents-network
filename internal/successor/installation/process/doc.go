// Package process retains original kernel process observations of the separately
// fixed installed Endpoint, Source and Forwarding. It owns proc-directory identity, start clock, executable,
// arguments, credentials, cgroup membership and InvocationID observations.
// Retain is root-only predecessor observation; RetainSelf is limited to the
// actual non-root calling process. Kernel supplementary groups may contain only
// its primary group. Expected facts grant no Release or startup authority.
// Endpoint's Expected/Retain contract is unchanged; no public selector
// generalizes a purpose. The transitional Source invocation orchestration
// (SourceExpected/RetainSource/InterruptSource) is retired (ADR-0125, #604);
// the platform-led Source lifecycle owns Source delivery. Installation checks
// its original lease, bytes and manager around observations; cgroup separately
// owns descendant join.
// ForwardingExpected and its root/self openers choose only the Forwarding argv
// and unit. ForwardingInvocation hides the underlying Invocation type,
// retains its own root pidfd and shares permanent refusal/joined Close across
// copies. InterruptForwarding supplies only an original physical SIGTERM;
// Installation must separately establish stop admission and original domain join.
// RetainForwardingStop additionally compares the archived original proc inode
// after fresh fixed-purpose admission; copied archive facts grant no signal right.
// JoinExpected, RetainJoin and RetainJoinSelf independently fix only JOIN argv,
// unit and original native identity. They share physical source, not Source or
// Forwarding process custody, root signal rights or startup authority.
// DescriptorExpected and its root/self openers fix Descriptor argv, original
// process and unit independently. DescriptorInvocation exposes no signal right
// or underlying Invocation and cannot borrow a sibling process observation.
// CheckSourceRecoveryQuiescent independently excludes other processes of the
// actual Source principal, including commands outside the fixed unit. State
// separately retains semantic leases, manager and cgroup checks; procfs absence
// is not successful original domain completion and grants no signal right.
package process

import "errors"

var errBinding = errors.New("installation process: observation mismatch")
