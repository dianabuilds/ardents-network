---
id: R-099
title: One confined text-Service job on Ubuntu
status: completed
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-09-08
---

# R-099 — Which bounded Application job can deny ordinary-network escape?

## Decision this unlocks

Select one concrete job, platform and enforcement boundary before implementing
an Application-level location protection condition. A generic Browser adapter
or local Grant alone does not deny ordinary networking.

## Current contract

The Product Owner selected publication/read of a UTF-8 text document with
confinement on Ubuntu at both endpoints, then Target Link first.
[ADR-0081](../../adr/0081-select-closed-protected-service-contract.md) and the
[workload owner](../../product/protected-service-workload.md) fix that scope.
The [confinement owner](../../technical/application-confinement.md) is the
implementation contract. Current C0 generic adapters remain unqualified;
their old browser path does not become the protected job.

## Hypotheses

- H1: distribution-maintained process isolation plus a finite inherited local
  stream can contain the complete reader/Publisher worker tree for this job.
- H0: a required denied path remains usable, or the permitted local stream
  cannot operate within the selected resource and lifecycle contract.

## Evaluation criteria

Deny IPv4/IPv6/UDP/DNS, host files/IPC, namespace/privilege escape and inherited
external descriptors before local authority or remote effects. Include child
processes, malicious sibling workers, restart, revoke and complete cleanup.
Positive controls must demonstrate the attempted forbidden operations.
The trusted owner/UI, Endpoint, installed launcher, root and kernel are explicit
surviving boundaries. Their complete compromise is not contained by a worker
sandbox.

## Evidence plan

Primary inputs, accessed 2026-09-07: distribution systemd 255 execution/socket
configuration and the actual Ubuntu 24.04.4 package/kernel runtime inspected
by the [contract probe](../../../experiments/r-152-contract-probes/README.md).
The [R-152 assessment](r-152-closed-scheme-contract.md) records exact sources,
commands, hashes, failures and limitations.

The falsification matrix was recorded before the original probe. Full installed
socket activation, worker IPC, snapshot import, local grants, revoke and the
actual text journey are P1/P6/P7/P10 implementation acceptance, not evidence
already supplied by a primitive-level probe.

## Findings

- **Measurement:** on WSL Ubuntu 24.04.4, systemd 255.4-1ubuntu8.14 and kernel
  6.6.87.2, fourteen parent/child forbidden operations were denied while their
  unconfined positive controls worked; the permitted local byte marker survived.
- **Measurement:** the first configuration failed before Application startup
  because WorkingDirectory was outside the confined root. The corrected profile
  sets WorkingDirectory=/; that correction does not erase the failed attempt.
- **Inference:** these results justify selecting the listed mechanisms for the
  fixed worker composition. They do not qualify bare Ubuntu, the complete
  installed Application or resistance to privileged host compromise.

## Options

Select installed systemd units, separate service identities and private
worker roots for the narrow text job. Reject proxy-only enforcement because
it cannot exclude alternate sockets/helpers. Defer a general browser,
arbitrary launcher framework, Windows sandbox and container-daemon dependency;
none is required by the selected first job.

## Recommendation

Choose H1's bounded engineering contract with implementation qualification
still required. Its strongest limitation is dependence on the surviving trusted
host and verified launch/descriptor boundary.

## Disposition

The job/platform/mechanism selection is complete; Application qualification is
not. Current requirements live in the workload, threat, confinement and
qualification owners. Retain the disposable probe as reproducible evidence.
The earlier instruction to wait for a job/platform is retired because that
choice has been made.
