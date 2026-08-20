# Stage 7 Application Isolation specification

Status: **accepted for Stage 7 development on 2026-08-20 under ADR-0016.**
R-052 development-host falsification and explicit native-host qualification
gates remain. The Windows profile uses only the bounded first-party `unsafe`
decision recorded in ADR-0016.

This document freezes the exact accepted R-052 profiles to implement and falsify. It does not claim
that naming one OS mechanism proves isolation. A profile qualifies only when
the complete R-051 principal tree passes every applicable F cell under
candidate-independent observers.

## 1. Profile classes and support

| Profile | Ubuntu 26.04 LTS `x86-64` | Windows 11 `x86-64` | Claim ceiling |
|---|---|---|---|
| `generic-v1` | owner-scoped named local endpoint or numeric-loopback browser Adapter | owner-scoped named local endpoint or numeric-loopback browser Adapter | `application-networking-unverified` |
| `ubuntu-bwrap-native-v1` | exact non-setuid bubblewrap profile below | unavailable | eligible only after R-051/R-052 evidence |
| `windows-appcontainer-native-v1` | unavailable | zero-network-capability AppContainer plus non-breakaway Job profile below | eligible only after R-051/R-052 evidence |
| `isolated-browser-v1` | unsupported in Stage 7 | unsupported in Stage 7 | none |

Installed and Portable use the same profile decision. Installation does not
create a stronger Application Principal or isolation result. A missing,
unpatched, misconfigured, or unobservable prerequisite returns
`isolation-unsupported`; it never launches generic and never converts an
unqualified result into success.

The Stage 7 claim-bearing development Applications are the controlled native
client and publisher. The R-056 direct binary remains first-class with claim
`none`. The default-browser Adapter remains first-class and generic, preserves
the browser's normal Internet/VPN policy, and displays
`application-networking-unverified`.

`isolated-browser-v1` is deliberately unsupported. On Windows, unpackaged
Firefox and an unpackaged reverse-HTTP Adapter require a host loopback
exemption rather than an application-private channel. On Ubuntu, a visible
unmodified browser needs display and possibly desktop-service IPC beyond the
current Ardents-only allow-list. Neither is silently admitted into a Stage 7
privacy claim. A future isolated-browser profile requires new research and an
accepted scope decision; it is not a hidden S7 implementation obligation.

## 2. Shared launch and result contract

The Application Isolation Module consumes one already validated R-051 launch
request: profile identity, executable and policy identities, private inherited
Broker channel, Local Grant, Isolation Context, resource parent, start identity,
Broker/Endpoint starts, and monotonic deadline. It returns only:

- exact platform/profile identity and observations needed by the Application
  Broker;
- `isolated` after every required pre-activation predicate is observed;
- `isolation-unsupported` when a declared prerequisite is unavailable; or
- `isolation-denied` when policy, identity, setup, or cleanup is invalid.

It cannot grant an operation, select a Route, retrieve an update, read Authority
Custody state, open public networking for the Application, or fall back to a
generic profile. The Broker remains the sole owner of the Local Grant and the
R-051 tree lifecycle.

The exact order is:

1. validate the named profile, committed surface/candidate manifest, executable,
   context, storage manifest, resource envelope, and observers;
2. create empty context storage and platform containment before untrusted code;
3. create the R-051 root stopped or atomically inside its complete tree owner;
4. apply and independently observe platform identity, network, filesystem,
   process, handle/descriptor, and resource facts;
5. close all launcher-only objects, activate the private channel challenge, and
   resume only after every predicate agrees;
6. continuously inventory tree membership, resources, network observations,
   context storage, and deadline; and
7. on revoke, EOF, crash, expiry, excess, or observation loss, deny new work,
   kill the complete tree, close every object, remove volatile context state,
   and prove zero residue within `5 s`.

No attach-after-launch, reconnect, tree transfer, child promotion, profile
downgrade, or cleanup success based solely on candidate self-report exists.

## 3. Ubuntu profile: `ubuntu-bwrap-native-v1`

### 3.1 Frozen dependency and preflight

The candidate is upstream bubblewrap `v0.11.2`, source tag `v0.11.2`, licensed
`LGPL-2.0-or-later`. Each campaign pins the actual Ubuntu package/source,
executable SHA-256, dynamic-library closure, build configuration, and advisory
state. The candidate MUST be built with setuid support disabled and MUST NOT
have a setuid/setgid bit or file capability. A distro backport or later version
is a different candidate until its exact binary and source identity are frozen.

The host must provide unprivileged user namespaces, cgroup v2, the R-051
`clone3(CLONE_INTO_CGROUP)`/pidfd/cgroup-kill profile, mount/PID/network/IPC/UTS
namespaces, and the required host observers. Missing support fails
`isolation-unsupported`; setuid bubblewrap, a privileged wrapper, permissive
`--*-try`, a permanent daemon, or post-start PID migration is forbidden.

The Installed package may declare the reviewed bubblewrap package as a runtime
prerequisite. Portable neither installs nor downloads it; Portable isolated
launch succeeds only when the same preflight passes. Direct and generic
Portable use remain available when it does not.

### 3.2 Namespace and privilege policy

The Broker creates the R-051 private socketpair, empty cgroup, pidfd-observable
root, read-only executable/runtime source handles, and empty per-context roots.
The trusted launcher is part of the same principal and tree inventory. It
constructs a bubblewrap invocation with these mandatory semantics:

```text
--unshare-user --unshare-pid --unshare-net --unshare-ipc --unshare-uts
--unshare-cgroup --disable-userns --assert-userns-disabled
--new-session --die-with-parent --cap-drop ALL --clearenv
--proc /proc --dev /dev --tmpfs /tmp
```

The actual manifest records one ordered argv vector; an omitted, duplicated,
unknown, `--share-net`, or `--*-try` option rejects before launch. The network
namespace contains only its private loopback device. No host network namespace,
DNS resolver, proxy, network manager, system or session D-Bus, SSH/GPG agent,
container socket, `/proc/sys` writer, cgroup control, Wayland/X11, audio, portal,
or host runtime socket is mounted or inherited.

Bubblewrap's internal setup/PID1 processes, the native Application, and every
helper are charged to and inventoried in the R-051 cgroup. The inherited Broker
channel is the only external communication handle. No network handle or
namespace FD is inherited or passed.

### 3.3 Filesystem and environment policy

The mount namespace starts empty. The campaign manifest names every read-only
runtime file/directory by host handle, normalized target path, type, mode,
owner, filesystem identity, and SHA-256 where finite. Bind-by-path after
admission is forbidden. The visible set is limited to:

- the exact verified Ardents/native Application executable and its measured
  loader/library/certificate/time-zone closure;
- synthetic read-only identity files required by that executable, containing
  no real host account, resolver, network, mount, or machine identity;
- private `/proc`, minimal `/dev`, and bounded tmpfs `/tmp`; and
- one writable context root plus declared bounded output files.

`HOME`, `XDG_*`, locale, time zone, and temporary paths are replaced with exact
context-local values. The environment allow-list is fixed in the candidate
manifest; argv/environment contain no grant, channel secret, Name, Target, or
Authority material. Host home, other contexts, package/update state, Vault,
evidence verdict root, and device/network configuration remain invisible.

The context root is mode `0700`, owned by the invoking user outside the
sandbox, mapped only at its fixed sandbox path, capped at the shared storage
bound, and removed after its declared retention policy. A symlink, mount,
hardlink, ownership, filesystem, or path mismatch denies launch.

### 3.4 Termination

Revocation closes the Broker channel, writes `1` to the authoritative cgroup
`cgroup.kill`, waits for recursive `populated=0`, reaps through the retained
process handle, closes namespace/source/storage handles, unmounts automatically
when the last process exits, and removes the empty cgroup and volatile context
root. `--die-with-parent` and the bubblewrap PID1 are defense in depth; they do
not replace the cgroup/observer proof.

## 4. Windows profile: `windows-appcontainer-native-v1`

### 4.1 Stable API candidate and preflight

The candidate uses documented desktop AppContainer APIs and the exact R-051
anonymous-pipe/suspended-root/Job profile. It does not use the experimental
`CreateProcessInSandbox` API, Windows Filtering Platform callouts, firewall rule
mutation, `CheckNetIsolation` loopback exemptions, a service, driver, or
packaged-app identity.

Before launch the Broker verifies the manifest-bound Windows 11 `x86-64` build and stable
availability of AppContainer profile/SID, extended process startup, token,
handle-list, Job, filesystem ACL, process, and host-observer APIs. The profile
requires a minimal Windows-only Win32 bridge for AppContainer profile calls,
`SECURITY_CAPABILITIES`, Job limits, and their fixed structures. ADR-0016 must
accept its exact `unsafe.Pointer` boundary, layouts, lifetimes, build tags, and
dedicated tests; otherwise this profile is `isolation-unsupported`.

### 4.2 Profile, token, channel, and Job

For each Isolation Context the Broker creates a random, non-user-meaningful,
at-most-64-character AppContainer profile name and obtains its AppContainer SID.
It grants **zero capability SIDs**, including zero Internet, private-network,
enterprise-authentication, device, and loopback capabilities.

The Broker creates the R-051 anonymous-pipe pair and a fresh unnamed Job. It
starts the exact verified native image with `STARTUPINFOEX`,
`PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES`, an exact
`PROC_THREAD_ATTRIBUTE_HANDLE_LIST`, and `CREATE_SUSPENDED`. Only the child's
private read/write pipe handles and declared standard handles are inherited.
Before resume it assigns the stable process handle to the non-breakaway Job,
applies the R-051 process/memory/handle/time limits, and independently verifies:

- AppContainer token state and exact AppContainer SID;
- expected owner/logon/session/integrity facts;
- zero capability SIDs and zero loopback exemption;
- exact Job membership and no breakaway permission; and
- no undeclared inheritable handle, network handle, listener, or child process.

Any failed query denies before resume. PID, path, profile name, SID, loopback
port, or token alone is never an Application Principal.

### 4.3 Filesystem and network policy

The native image and measured loader/library closure are copied or materialized
by exact verified digest into a context-local executable root when the original
Portable path is not AppContainer-readable. No permanent ACL is added to an
Owner-chosen Portable path. Only the AppContainer SID, Broker/Owner recovery
identity, and required system identities receive the exact read/execute or
read/write ACLs for:

- one read/execute image/runtime root;
- one bounded read/write Isolation Context root; and
- no other program, update, Vault, evidence, home, registry, package, or
  Application-context state.

AppContainer network access is deny-by-default because the token receives no
network capability. The profile opens no loopback listener and installs no
loopback exemption. The sole external communication path is the inherited
anonymous-pipe channel; every child remains in the same Job and AppContainer.
The campaign still runs the complete socket/DNS/listener/handle-duplication
corpus because token configuration is a candidate mechanism, not the verdict.

### 4.4 Termination and profile cleanup

Revocation closes the Broker pipe ends, terminates the Job, waits for an empty
Job and root exit, closes all token/process/thread/Job/profile handles, removes
the volatile context/image roots, calls `DeleteAppContainerProfile`, and proves
the profile, SID-owned filesystem/registry state, rules, handles, listeners, and
processes absent within `5 s`.

Profile deletion is attempted only after all profile handles close. A failed
deletion has an undetermined cleanup state, is retried within the deadline, and
is `fail` or `invalid` according to observer evidence; it is never reported as
successful cleanup.

## 5. Exact browser disposition

The R-056 reverse-HTTP Adapter remains useful in `generic-v1`. It binds a random
numeric loopback port, enforces the exact origin/request contract, and launches
or hands off to the ordinary browser without changing system DNS, route,
default proxy/browser, or VPN policy. This is browser integration, not network
isolation.

Stage 7 does not grant `network-isolated-application-boundary` to Firefox on
either platform:

- Windows loopback is blocked for AppContainer by default. The available
  unpackaged-process loopback exemption is administrative/debug-oriented and
  broad enough to make host-local forwarders part of the egress surface.
- Ubuntu bubblewrap can create a loopback-only network namespace, but a visible
  unmodified Firefox session also needs desktop/display IPC not present in the
  accepted Ardents-only allow-list. Mounting D-Bus, portals, X11, or ambient
  runtime sockets would add escape authority; selecting a filtered compositor/
  portal profile is separate research.

Firefox launch flags, a fresh profile, preferences, loopback randomness, and an
R-051 tree owner do not override this result. The optional isolated-browser UI
reports `isolation-unsupported`; direct binary, native claim-bearing controlled
Applications, and generic browser remain available.

## 6. Evidence identity and falsifiers

Each candidate manifest freezes OS image/build, architecture, executable and
dependency identities, source/license/advisory review, exact argv/environment,
namespace/token/capability/Job/cgroup/storage policy, privilege, observers,
positive controls, attempt count, resource envelope, and cleanup inventory.

Every F0-F10 cell runs for controlled client and publisher trees through direct
code, child, grandchild, delayed helper, shell/interpreter, and inherited/
duplicated-handle variants where the host supports the action. The exact
envelope is inherited from R-051: at most `32` processes, `256 MiB` tree memory,
`1,024` handles/FDs, `256` queued frames, one `64 KiB` channel, and `5 s`
termination/cleanup; Ubuntu additionally enforces `pids.max=512` tasks.

One successful undeclared packet, DNS query, external or host-loopback
connection, reachable listener, callback, WebSocket/WebRTC/QUIC path, child or
Job/cgroup escape, cross-context read, undeclared IPC/handle, surviving process,
global policy change, profile/rule/storage residue, or generic fallback is
`fail`. A missing positive observer control or required observation is
`invalid`. Candidate self-report can corroborate but never replace the host
observer.

R-052 profiles are selected by ADR-0016 for implementation. Their scheduled
Ubuntu-Docker/current-Windows subset must pass before the development handoff;
unobservable native containment facts remain visibly `environment-deferred`.
No undeclared dependency, privilege, command, or isolation Adapter is
authorized beyond the accepted profile and the repository package rules.
