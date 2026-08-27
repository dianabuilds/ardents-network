---
id: R-101
title: Cross-platform Endpoint state and local attachment profile
status: decided
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-24
---

# R-101 — Which minimal cross-platform Endpoint composition can own protected state and local Application attachment on Windows 11 and Ubuntu LTS without treating temporary Unix sockets or a user profile directory as a security boundary?

## Decision this unlocks

Select the H4-1A Endpoint state-root and local attachment composition required
before its lifecycle experiment. The result may authorize a small platform
Adapter boundary and exact state-class ownership; it does not select H4-7
malicious-sibling isolation or a supported Installed profile.

## Current contract

- H4-1 separates replaceable program bytes from Vault, configuration/Grants,
  monotonic floors, cache, live state, and diagnostics.
- H4-3/H4-7 distinguish useful generic local attachment from a qualified
  Application Principal/isolation claim.
- Current `endpoint run` owns temporary Unix sockets only; its plan has no state
  root and it has no Windows local attachment profile.
- Portable runtime remains unprivileged and must not imply remote
  administration, global proxy/DNS, autostart, or Contributor duty.

## Hypotheses

- **H1:** a shared logical Endpoint-state layout plus thin platform path/IPC
  adapters can give Windows and Ubuntu the same generic H4-1A contract.
- **H2:** the two platforms need visibly separate H4-1A subprofiles, while
  preserving one state-class and failure contract.
- **H0:** no small profile can safely distinguish state classes and local
  attachment under normal user permissions; alpha must narrow platform support.

## Evaluation criteria

- named locations/ownership and permissions for every seven state class;
- exclusive state-root ownership, crash/interruption behavior, and no program
  directory authority;
- local IPC identity/access, cleanup, restart, path-length and collision rules;
- Windows and Ubuntu normal-user behavior, upgrade/replacement and removal;
- generic/unqualified claim boundary versus Application Principal isolation;
- standard-library/platform maintenance cost, dependency/license implications.

## Evidence plan

### Primary sources

- Official Microsoft documentation for known folders, named pipes, ACLs and
  user-scoped data locations.
- Official freedesktop/XDG, systemd and Linux Unix-domain socket/permissions
  documentation.
- Current Endpoint, Broker, release/custody, operating-model and threat
  contracts.

### Experiment

For each candidate platform Adapter, create a disposable ordinary-user harness
that creates all state classes and local attachment, attempts duplicate owner,
wrong permission/path, stale socket/pipe, crash/restart and artifact-directory
deletion, then captures remaining state/handles/listeners. It uses synthetic
keys and no Ardents Authority.

### Failure scenarios

- A program replacement deletes or imports protected state.
- Another process/user reaches a local attachment or blocks restart through a
  stale object.
- A path/ACL/default exposes a Vault/floor or treats loopback/pipe as a
  malicious-sibling boundary.
- Windows and Ubuntu silently diverge in durable-state or cleanup semantics.

## Findings

- **Sourced fact:** XDG distinguishes user configuration, persistent state,
  non-essential cache, and runtime files. It places runtime objects such as
  sockets under `$XDG_RUNTIME_DIR`; its default persistent state is
  `$HOME/.local/state`, cache is `$HOME/.cache`, and a newly created write
  directory should use mode `0700`. [XDG Base Directory
  Specification](https://specifications.freedesktop.org/basedir/) (accessed
  2026-08-24).
- **Sourced fact:** Windows named pipes have ACL-controlled client/server
  access. A default pipe descriptor grants read access to Everyone and
  Anonymous; Windows documentation also warns that a pipe can be remotely
  accessible unless the application denies `NT AUTHORITY\\NETWORK` or uses a
  local-only alternative. [Named pipe security](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights)
  and [named-pipe overview](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipes)
  (accessed 2026-08-24).
- **Inference:** a Windows named-pipe candidate needs an explicit DACL and
  locality test; a default pipe is unacceptable even for the generic alpha
  profile. This does not itself create a malicious-sibling isolation claim.
- **Sourced fact:** Windows known folders include a per-user Local AppData
  location, but their actual path can differ due to installation/redirection.
  [Microsoft KNOWNFOLDERID documentation](https://learn.microsoft.com/en-us/windows/win32/shell/knownfolderid)
  (accessed 2026-08-24).
- **Inference:** R-101 must select a logical state layout and resolve its
  platform base path at runtime; hardcoding a user-profile path is not an H4-1
  profile.
- **Current-code fact:** current maintained Windows Entry and Node-duty roots
  use the already reviewed `golang.org/x/sys/windows` surface to set a
  protected DACL granting the current user full control, with inheritance on a
  root directory and explicit protection on children. Their Unix counterpart
  rejects a root with group/other permission bits. These are useful platform
  inputs, not an authorization to make the Entry/Node packages own Endpoint
  state. [Entry Windows permissions](../../internal/entry/root_permissions_windows.go)
  and [Unix permissions](../../internal/entry/root_permissions_unix.go)
  (inspected 2026-08-24).
- **Inference:** the leading H4-1A logical layout is:

  | Class | Ubuntu LTS base | Windows 11 base |
  |---|---|---|
  | configuration and Local Grants | `$XDG_CONFIG_HOME/ardents` | resolved `FOLDERID_LocalAppData\\Ardents\\Endpoint\\config` |
  | Vault, floors, diagnostics | `$XDG_STATE_HOME/ardents/{vault,floors,diagnostics}` | resolved `FOLDERID_LocalAppData\\Ardents\\Endpoint\\{vault,floors,diagnostics}` |
  | cache | `$XDG_CACHE_HOME/ardents` | resolved `FOLDERID_LocalAppData\\Ardents\\Endpoint\\cache` |
  | live state and local attachment | `$XDG_RUNTIME_DIR/ardents` | resolved `FOLDERID_LocalAppData\\Ardents\\Endpoint\\r` |

  The program artifact remains entirely outside these bases. The R-102 owner
  lock belongs under the persistent state/live area, while the attachment
  socket belongs only in the runtime area. Ubuntu must reject a missing or
  unsuitable `XDG_RUNTIME_DIR` rather than fall back to a global `/tmp`; this
  matches H4-1's no-linger session lifecycle. Windows resolves its known folder
  at runtime and uses the deliberately short `r` directory to preserve an
  explicit AF_UNIX path budget.
- **Inference:** on Ubuntu the selected roots/children require `0700` and
  regular protected files `0600`; on Windows the Endpoint-owned roots/children
  require a protected owner-only DACL. These normal-user access rules protect
  against other OS users under the stated conditions, not a process that shares
  the same user account or an administrator.
- **Measurement:** `go test ./internal/endpoint -count=1` passed on the current
  Windows host in 2.310 seconds on 2026-08-24. Therefore the current Unix-domain
  test path is technically executable on this host. The package still owns only
  temporary sockets and has no state root or declared platform profile, so this
  is not H4-1A evidence.
- **Sourced fact:** Windows supports the `AF_UNIX` address family for IPC between
  local Win32 processes. [Microsoft IPC documentation](https://learn.microsoft.com/en-us/windows/win32/ipc/interprocess-communications)
  (accessed 2026-08-24).
- **Implementation fact:** the current Go standard library has an explicit
  Windows `SupportUnixSocket` capability check and documents its minimum Windows
  requirement as build 17063. The current host is Windows build 26200, and the
  Endpoint package's Unix-socket test passes here. This supports a runtime
  capability check rather than assuming a socket family from the operating-system
  name alone.
- **Inference:** a filesystem Unix socket is now a viable shared *generic local
  attachment* candidate for the supported Windows 11 and Ubuntu LTS alpha. It
  remains unsuitable as evidence of Application Principal or malicious-sibling
  isolation; access, stale-object, and state-root behaviour must be measured.
- **Measurement:** on the current ordinary-user Windows host, the disposable
  R-101 harness completed with `state_classes=7`,
  `duplicate_owner_rejected=true`, `duplicate_listener_rejected=true`,
  `local_round_trip=ready`, `stale_socket_after_close=false`,
  `listener_recovery=true`, and `artifact_state_separation=true` on
  2026-08-24. The fixed experiment root was absent after the wrapper completed.
  This measures normal clean shutdown only: it does **not** simulate process
  termination, another user, or a hostile process under the same user account.
- **Measurement:** the same harness, cross-compiled with the current Go 1.26.6
  toolchain for Linux/amd64, completed in local Ubuntu 24.04 WSL (Linux
  `6.6.87.2-microsoft-standard-WSL2`) with the same seven successful result
  fields on 2026-08-24. The guest has no Go installation, so this is execution
  evidence for the produced Linux binary rather than an Ubuntu-native build.
  The direct binary invocation intentionally left its fixed experiment root;
  it was then removed only after its exact path was verified, and absence was
  confirmed. This is still a normal clean-shutdown result, not a crash, access
  control, or hostile-sibling result.
- **Measurement:** the Windows abrupt-termination harness created state and a
  Unix listener, wrote its readiness marker, then had only that child process
  forcibly terminated. Before wrapper cleanup, it observed
  `owner_file_after_kill=true`, `socket_after_kill=true`, and
  `vault_after_kill=true` on 2026-08-24. The experiment's short runtime root
  was necessary: two earlier long-root harness attempts exited before the
  marker, but their stderr was not captured, so their exact cause is not
  asserted. Future attempts now retain temporary stderr only long enough to
  report such a failure.
- **Inference:** a durable create-exclusive `owner.lock` records residue, not
  live ownership, after a forced process exit. It is unacceptable as the
  maintained Endpoint owner lock. A filesystem socket can also remain stale on
  this Windows profile. Restart therefore needs a separately selected
  process-lifetime locking primitive and a bounded, authenticated stale-socket
  recovery rule; it must not delete an attachment merely because it exists.
- **Inference:** the H4-1A runtime path must have a short, declared maximum and
  fail before attempting to bind if it exceeds it. General Windows long-path
  support does not establish an AF_UNIX socket path limit or make long dynamic
  profile paths supportable. [Microsoft path-length documentation](https://learn.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation)
  (accessed 2026-08-24).
- **Measurement:** the same abrupt-termination harness, cross-compiled for
  Linux/amd64 and run in local Ubuntu 24.04 WSL, observed
  `owner_file_after_kill=true`, `socket_after_kill=true`, and
  `vault_after_kill=true` after `SIGKILL` on 2026-08-24. Its wrapper then
  verified removal of only its fixed temporary root, marker, and log. This
  matches the Windows residue result; it is not evidence for a particular
  lock primitive or real multi-user access policy.
- **Inference:** stale durable owner residue and stale filesystem sockets are a
  two-platform H4-1A condition, not a Windows-specific cleanup anomaly. The
  liveness-lock and ordered recovery decision is separated into
  [R-102](r-102-endpoint-liveness-lock.md).
- **Measurement:** the first R-102 startup-substitution probe pre-created the
  exact `runtime` child as a Windows junction or Ubuntu symlink to a distinct
  sentinel. The unrefined harness followed both links and created its socket
  outside the intended state root. After replacing recursive creation with
  exact `Lstat`/single-directory checks, both platforms rejected the same case
  as `unexpected_runtime_root`, created no outside socket, and preserved the
  sentinel. This supports deterministic startup-path validation; it does not
  defeat a malicious same-user race after validation.
- **Measurement:** the refined harness enforced and read back the accepted
  access profile before readiness. Ubuntu 24.04 WSL verified effective-user
  ownership with `0700` directories and a `0600` socket. Windows 11 applied a
  protected DACL to every owned directory and the live `AF_UNIX` socket, then
  verified the current owner and that every returned DACL trustee was the
  current user. The first Windows assertion incorrectly required one ACE;
  Windows normalized an inheritable directory rule into effective and
  inherit-only entries for the same SID, so the assertion was narrowed to the
  actual sole-trustee invariant. Both full platform runs then passed. No second
  local account was available for an actual cross-account connect attempt, so
  this is permission-policy round-trip evidence rather than cross-user attack
  evidence.

## Options

1. Shared filesystem Unix socket plus XDG/LocalAppData base-path and permission
   adapters under one logical state contract.
2. Platform-specific generic IPC adapters under one logical state contract.
3. One loopback-only attachment on both platforms.
4. Retain Unix sockets and narrow early alpha to Ubuntu.

## Recommendation

**Product decision (2026-08-24): option 1 is selected for H4-1A.** It remains
subject to one purpose-owned Endpoint state-root implementation and behaviour
evidence. It
avoids a second IPC protocol for the two stated platforms while making paths,
permission rules, runtime lifetime, and AF_UNIX budget explicit. It must have
a runtime capability test and a defined failure path, rather than silently
falling back to loopback or a named pipe. R-102 supplies its liveness/recovery
order. No option earns H4-7 isolation by itself.

**Confidence:** medium. **Strongest argument against the candidate:** Windows
ACL inheritance and Linux/XDG runtime-directory lifetime must still be tested
by the purpose-owned Endpoint state-root module; a successful disposable
harness is not a full lifecycle/support result.

## Disposition

Selected for the generic H4-1A composition. The ordinary-user, abrupt-loss,
path-substitution, permission-policy, and guarded-recovery harnesses now have
matching Windows and Ubuntu LTS outcomes. R-102 owns the selected OS-lock
sequence. Purpose-owned Endpoint integration, an actual cross-account access
cell on each supported platform, and the complete H4-1 lifecycle run are
implementation-linked qualification tasks. None of those gates reopens the
rejected same-user isolation claim; H4-7 owns that distinct problem. The
decision is promoted to H4-1; retain this record only until its evidence enters
source history.
