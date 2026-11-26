// Package worker observes fixed root-installed worker artifacts, systemd
// activations and credentialed attachments, and joins one pinned original
// cgroup. These mechanisms grant no local or Network authority. The Execution
// launch consumer owns serialization, readiness and exact Job handoffs.
//
// The launch orchestration is a portable skeleton (activate.go): verify parent
// service, pin artifact, verify activation socket, baseline listing, dial,
// prepare socket, recheck socket identity, await the new instance and bind its
// identity. Each platform step stays behind a capability symbol. The Linux
// adapter implements those symbols through the installed systemd/cgroup
// mechanism (service_linux.go, inventory_linux.go, instance_linux.go,
// properties_linux.go, cgroup_linux.go, cleanup_linux.go, artifact_linux.go,
// manager_linux.go, activate_linux.go). The macOS adapter (manager_darwin.go)
// is an explicit typed refusal that documents the missing G1–G6 confinement and
// lifetime guarantees; the non-Unix adapter (native_refusal_other.go) refuses
// likewise. Both refuse at the first parent-service verification, before any
// artifact, activation or cleanup pin. A missing required protection is never a
// weaker accepting path (ADR-0129). A macOS native confinement profile proving
// G1–G6 through macOS mechanisms is a separate future slice, not admitted here.
//
// The attachment's AF_UNIX/SCM_RIGHTS transport is Unix-family (Linux and macOS,
// attachment_unix.go). The per-frame peer-credential authentication that binds
// the worker identity is a capability port at that boundary: Linux authenticates
// every frame through SO_PEERCRED/SO_PASSCRED/SCM_CREDENTIALS (peercred_linux.go),
// while macOS exposes only connect-time LOCAL_PEERCRED and therefore refuses with
// an explicit typed unavailable outcome (peercred_darwin.go); non-Unix platforms
// refuse the attachment outright.
//
// Shape review (#866, design §2.2): Instance.Cgroup is the current Linux
// spelling of a cleanup-scope identity fact, not a platform-neutral guarantee.
// The capability facts should ultimately be guarantee-facts — identity bound
// (G1), confinement effective (G2), lifetime bound (G3), original-invocation
// identity (G4), cleanup joined with no residual execution (G5), frame
// authenticated (G6) — so a macOS adapter could prove the same guarantees through
// sandbox/rlimits/process-group/audit-token facts rather than systemd properties.
// The proposed factoring renames the cleanup-scope field to a neutral identity
// fact and keeps the cgroup path as the Linux adapter's private proof detail.
// That refactor is deliberately deferred: it must preserve every Linux property
// check and the cgroup ENODEV removal proof byte-identically, so this slice only
// records the direction and leaves the Linux facts unchanged.
//
// Responsibility clusters and their tests:
//   - portable activation orchestration and installed Stream qualification:
//     activate.go, activation.go, artifact_linux.go
//     (stream_activation_installed_linux_test.go; linux native profile
//     "linux && amd64 && document_worker_installed")
//   - Unix-family attachment transport and per-frame peer credentials:
//     attachment_unix.go, peercred_linux.go, attachment_refusal_other.go
//     (attachment_unix_test.go; unix, attachment_linux_test.go; linux)
//   - macOS connect-time-only credential boundary: peercred_darwin.go
//     (peercred_darwin_test.go; darwin)
//   - cgroup cleanup ownership and removal proof: cleanup_linux.go,
//     cgroup_linux.go (cleanup_linux_test.go, cleanup_query_linux_test.go,
//     cleanup_stop_linux_test.go; linux)
//   - instance and inventory observation: instance_linux.go,
//     inventory_linux.go (instance_linux_test.go; linux)
//   - systemd manager transport and executable observation: manager_linux.go,
//     original_command_linux.go (manager_linux_test.go; linux)
//   - unit binding and slice property facts: properties_linux.go
//     (properties_linux_test.go; linux)
//   - endpoint service facts and platform admission: service_linux.go
//     (service_linux_test.go; linux)
package worker
