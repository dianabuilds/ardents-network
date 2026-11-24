// Package worker observes fixed root-installed worker artifacts, systemd
// activations and credentialed attachments, and joins one pinned original
// cgroup. These mechanisms grant no local or Network authority. The Execution
// launch consumer owns serialization, readiness and exact Job handoffs.
//
// The attachment's AF_UNIX/SCM_RIGHTS transport is Unix-family (Linux and macOS,
// attachment_unix.go). The per-frame peer-credential authentication that binds
// the worker identity is a capability port at that boundary: Linux authenticates
// every frame through SO_PEERCRED/SO_PASSCRED/SCM_CREDENTIALS (peercred_linux.go),
// while macOS exposes only connect-time LOCAL_PEERCRED and therefore refuses with
// an explicit typed unavailable outcome (peercred_darwin.go); non-Unix platforms
// refuse the attachment outright. A missing required protection is never a weaker
// accepting path (ADR-0129). The systemd/cgroup activation, artifact and original
// invocation mechanisms remain genuine Linux.
package worker
