# Rendezvous Contributor functional-alpha candidate

Status: **candidate only; fresh-host qualification has not accepted this
profile.** These commands are the complete proposed H4-5A operator surface,
not a public Contributor offer or a capacity/availability claim.

## Exact supported shape

The candidate runs one `rendezvous` duty and nothing else on a freshly
provisioned Ubuntu LTS `x86-64` host with exactly 2 vCPU, a provider-declared
2 GiB allocation, cgroup v2, systemd as PID 1, and a provider-declared
symmetric 100 Mbit/s link. The host has no Client, Publisher, Application,
other Node duty, Docker workload, or project service. One unprivileged public
TCP port must reach the exact address in authenticated Network State. Outbound
TCP must reach the two literal-IP authenticated Source addresses in the plan;
there is no discovery, DNS fallback, alternate Source, or public admission.

The service runs through systemd `DynamicUser` with these enforced process
limits: `CPUQuota=100%`, `MemoryHigh=192M`, `MemoryMax=256M`, `TasksMax=64`,
`LimitNOFILE=256`, `GOMAXPROCS=1`, and `GOMEMLIMIT=134217728`. Rendezvous admits
at most four concurrent handshakes, two waiting legs, one paired route, and
16 MiB for that pair. The 100 Mbit/s host link is the candidate network
ceiling; it is not an Ardents throughput promise. The authenticated bundle is
bounded to one executable of at most 128 MiB and nine configuration inputs of
at most 64 KiB each. Update temporarily retains one previous generation.
Fresh-host evidence must still measure actual disk, network, CPU, memory,
socket, FD, task, and Go-runtime observations before these values can be
accepted.

## Authenticated input

The configuration authority supplies exactly two things by separate channels:

1. one directory containing `manifest.json` plus the closed inventory named by
   `ardents-contributor-bundle-v1`; and
2. the lowercase SHA-256 digest of the exact `manifest.json` bytes.

The operator compares the second value with the independently received pin;
the command verifies that pin before parsing the manifest, verifies every
listed file digest, rejects extra/missing inventory, and accepts only the fixed
Rendezvous plan and resource reservations. The bundle contains private keys.
Place its transfer copy in an owner-only temporary directory outside the
repository and remove that copy after a successful `apply`; the managed
installation never treats the caller-owned transfer directory as its own and
therefore never deletes it.

Run every lifecycle command as root from the exact candidate executable. There
are no environment-variable, interactive-shell, or arbitrary-systemd escape
hatches.

```sh
./ardents-node contributor apply --bundle /absolute/owner-only/bundle --manifest-pin MANIFEST_SHA256
```

Generation 1 requires an absent installation. A later `apply` must be the
exact same 32-byte deployment ID and exactly the next generation. Success is
reported only after the installed files and systemd unit match their recorded
digests, the unit is active, and the product Node has written `READY`. A normal
failure restores the prior authenticated generation; the next lifecycle
command also detects and recovers an update interrupted between filesystem
switches.

## Diagnose, restart, drain, and withdrawal

Each successful command emits one `ardents-contributor-report-v1` JSON object
containing only profile, deployment/generation and digest facts, lifecycle
state, and active/enabled state.

```sh
/usr/lib/ardents-contributor/current/ardents-node contributor diagnose
/usr/lib/ardents-contributor/current/ardents-node contributor restart
/usr/lib/ardents-contributor/current/ardents-node contributor drain
/usr/lib/ardents-contributor/current/ardents-node contributor withdraw
```

`diagnose` re-authenticates every managed file and the fixed unit, reads the
bounded last lifecycle diagnostic, and asks systemd for current state.
`restart` requires a verified installed generation and returns only after a
new `READY`. `drain` asks the Node to stop accepting handshakes, finish its
finite drain, and reach `WITHDRAWN`; the service remains enabled for a later
explicit restart. `withdraw` performs the same finite stop and then disables
the unit. A command fails instead of reporting a partial transition as
success.

The two bounded local diagnostics are:

- `/var/lib/private/ardents-contributor/diagnostics/lifecycle.json` — the last
  Node lifecycle event; and
- `/var/lib/private/ardents-contributor/diagnostics/resource.json` — the last
  resource observation or pressure transition.

They contain no private key or Application payload. Journal output remains a
host diagnostic and must not be published as network-capacity evidence.

## Complete managed removal

Removal is deliberately a two-step transition. First run `withdraw` and retain
its exact 32-byte deployment ID. Then run:

```sh
/usr/lib/ardents-contributor/current/ardents-node contributor remove --confirm DEPLOYMENT_ID
```

Removal is refused unless the service is inactive, disabled, and
`WITHDRAWN`, and unless the confirmation names the installed deployment. It
removes the managed executable generations, configuration and private keys,
Network State, local role state, diagnostics, installation record, and the
exact systemd unit, then reloads systemd and reports `REMOVED`. It does not
delete the caller-owned input bundle, unrelated host files, journal retention,
or provider snapshots. The operator must remove the owner-only transfer copy
after each apply and apply the provider's documented snapshot-retention policy;
those are declared external residues rather than hidden product repair steps.

## Honest failure boundary

If `diagnose` cannot authenticate the current installation, if READY or
WITHDRAWN is not reached within 15 seconds, or if the exact cgroup placement is
unavailable, do not edit managed files or invoke systemctl manually. Retain the
command error, bounded diagnostics, journal slice, and host observations as a
failed qualification attempt. A candidate profile is accepted only by the
frozen fresh-host matrix; ordinary lifecycle success alone does not qualify it.
