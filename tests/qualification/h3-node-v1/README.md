# H3 Node qualification

This profile owns the complete local black-box Node qualification. It starts
two authenticated sources and two separately keyed Node processes, exercises
role probes, refresh withdrawal, restart without overlapping duty, bounded
resource pressure, hostile connections, and terminal cleanup.

`ardents-qualify prepare-node` creates keys, state, plans, clock files, and
evidence outside Git. Compose receives that absolute directory through
`ARDENTS_NODE_ROOT` and mounts only each process's owned state and credentials.
On a native Linux Docker Engine host, preparation must include
`--linux-uid-ownership` so the fixed Compose role UIDs own only their declared
paths. Ordinary unit-test preparation omits that flag and remains unprivileged.

Docker Desktop may run the development matrix locally without making a
qualification claim. An official Stage 1 campaign requires the preflighted
dedicated Ubuntu Docker Engine/cgroup-v2 environment defined by the Stage 1
brief; a local run is not a substitute.

The campaign binary accepts exactly three normal modes. `short` runs the
hostile fault/resource matrix. `churn-2h` runs for two hours with five-minute
source and Node restart cycles, periodic authenticated probes, one-second
external samples, and a final 120-second quiescence gate. `unattended-24h`
runs for twenty-four hours without deliberate churn, checking the process set
every 30 seconds and the authenticated probe path every 15 minutes. An
interrupted campaign, failed harness, missing observer, or changed manifest is
`invalid`; it is not a candidate failure.

On a preflighted dedicated Ubuntu host, run the three campaigns serially with
separate freshly generated fixture and evidence roots:

```bash
set -eu
test "$(id -u)" -eq 0
test "$(. /etc/os-release; printf %s "$ID")" = ubuntu
test "$(stat -fc %T /sys/fs/cgroup)" = cgroup2fs
test "$(docker info --format '{{.OSType}} {{.CgroupVersion}}')" = "linux 2"
test "$(go version | awk '{print $3}')" = go1.26.5

campaign_parent=$(mktemp -d /var/tmp/ardents-h3-stage1-XXXXXXXX)
go build -trimpath -o "$campaign_parent/ardents" ./cmd/ardents
go build -trimpath -o "$campaign_parent/ardents-qualify" ./cmd/ardents-qualify

run_campaign() {
  mode=$1
  run_root="$campaign_parent/$mode"
  mkdir -m 700 "$run_root"
  "$campaign_parent/ardents-qualify" prepare-node \
    --root "$run_root/fixture" \
    --at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --ardents "$campaign_parent/ardents" \
    --linux-uid-ownership
  "$campaign_parent/ardents-qualify" run-node \
    --fixture "$run_root/fixture" \
    --evidence "$run_root/evidence" \
    --compose "$PWD/tests/qualification/h3-node-v1/compose.yaml" \
    --mode "$mode"
}

run_campaign short
run_campaign churn-2h
run_campaign unattended-24h
printf 'evidence parent: %s\n' "$campaign_parent"
```

Each evidence root contains the sealed fixture manifest, production-source
digest, resolved Compose topology, Docker host/image/container identities,
binary toolchain and linked dependency receipts, one-second cgroup/process
samples, candidate events, fault outcomes, cleanup receipt, and terminal
`result.json`. Private keys and mutable candidate state remain only in that
campaign's fixture root outside Git.
