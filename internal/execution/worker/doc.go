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
package worker
