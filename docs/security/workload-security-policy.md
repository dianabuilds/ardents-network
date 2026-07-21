# Workload Security And Resource Policy

## Scope And Ownership

This document defines the `v1` admission and enforcement contract shared by
`Policy` and `Workload Control`. Policy owns the allow/deny decision and its
stable reason. Workload Control owns the Docker Engine enforcement and observed
runtime outcome. Docker and the OCI runtime are adapters, not policy truth.

The sensitive assets are node credentials and files, Docker authority, workload
configuration values, image provenance, host network access, and host compute
capacity. A workload is hostile unless an operator-authorized policy reference
places it in the trusted tier.

The Docker endpoint is part of the control-plane trust boundary. Local
`unix`/`npipe` endpoints are accepted. TCP endpoints require TLS verification
and an explicit client certificate path. Plaintext TCP is available only to the
isolated Docker-in-Docker test fixture through an internal test-only constructor
option and is not exposed by `ardentsd` configuration.

## Closed Workload Configuration

The container configuration is a closed JSON object. Unknown and duplicate
fields are rejected. Accepted fields are:

- immutable `image` reference containing a `sha256` digest;
- numeric non-root `user`;
- bounded `command`, `entrypoint`, `working_dir`, and public `env`;
- optional resource requests within the node-enforced policy ceiling.

Mounts, devices, namespaces, privileged mode, capabilities, security options,
Docker sockets, network mode, port publication, and runtime selection are not
workload-controlled fields. Their presence is a policy denial, not an ignored
hint.

Environment values in this schema are public configuration. Secret values,
tokens, private keys, passwords, credentials, and secret-bearing URLs are not a
supported input channel and are rejected by key/value guards. `v1` does not
advertise workload secret injection. A future secret channel requires a
separate reference-only contract and a non-persistent delivery design before it
can enter this schema.

Read/status surfaces never return the raw container configuration. They return
workload identity, desired/observed state, policy reference, capabilities,
services, and safe execution outcome only.

## Trust And Runtime Selection

- Empty policy reference means `untrusted` and selects `runsc`.
- A non-empty policy reference is denied unless it appears in the node's
  explicit allowlist.
- The reserved allowed reference `trusted` selects hardened `runc`.
- The workload cannot select or override the runtime.
- Missing `runsc` for an untrusted workload fails closed; there is no fallback
  to `runc` or host process execution.

The isolated Docker integration fixture explicitly enables the trusted tier
because Docker-in-Docker does not provide gVisor. This is test environment
truth, not a product fallback.

`ardentsd` reads the operator boundary from:

- `ARDENTS_WORKLOAD_ALLOWED_REGISTRIES`;
- `ARDENTS_WORKLOAD_ALLOWED_POLICY_REFS`;
- `ARDENTS_WORKLOAD_TRUSTED_RUNTIME` (default `runc`);
- `ARDENTS_WORKLOAD_UNTRUSTED_RUNTIME` (default `runsc`).

The last two values name operator-installed Docker runtimes; they are never
copied from workload input. `ARDENTS_WORKLOAD_EXECUTOR=trusted-process` remains
restricted to the `local_development` node profile.

## Image Provenance

Digest pinning proves content identity but not operator authorization. Product
workload admission therefore requires a non-empty registry allowlist. A daemon
may start with an empty allowlist, but every container workload then fails
closed before creation. Image references
with an implicit registry normalize to `docker.io`; every other registry is
matched by normalized host name. A denied registry fails before container
creation. Signature/transparency verification is not claimed by `v1`; the
admission truth is an operator allowlisted registry plus immutable digest.

## Enforced Bounds

The node applies defaults and rejects values outside these `v1` ceilings:

| Resource | Default | Accepted range |
| --- | ---: | ---: |
| memory | 512 MiB | 32 MiB - 4 GiB |
| CPU | 1 CPU | 0.1 - 4 CPUs |
| processes | 128 | 16 - 512 |
| writable `/tmp` | 64 MiB | 1 - 512 MiB |

Memory swap equals the memory limit. Engine restart is disabled. The root
filesystem is read-only, `/tmp` is a bounded `noexec,nosuid,nodev` tmpfs, all
Linux capabilities are dropped, privilege escalation is disabled, and
host PID/IPC/UTS/user namespaces and devices are absent. Network mode remains
`none` when the workload has no admitted hosted-service ingress. Publication may
attach a service workload to an Ardents-owned per-generation internal bridge.
The workload container owns no host port. A separately labelled, resource-
bounded Ardents ingress proxy joins that bridge and a per-node ingress bridge;
only the proxy publishes the exact TCP ports derived from canonical service
declarations. The workload bridge supplies no arbitrary egress, while the proxy
contains no workload secrets and can dial only the fixed workload target/ports
provided by the executor. Container JSON cannot request a network, bind address,
proxy command, or published port. The operator must explicitly configure an
immutable Ardents ingress-proxy image, the ingress bind address, and every
advertised host. Missing or ambiguous ingress configuration fails admission
closed.

The Docker adapter accepts literal IPv4/IPv6 advertised hosts for direct
ingress. DNS advertisements require a separately verified operator ingress
mapping and are rejected until such a mapping is configured. Probe bindings
remain local to the daemon host; the remote bind is the configured host
interface (or an explicitly configured wildcard in controlled test/deployment
environments). The proxy image is a first-party, generation-labelled runtime
component with a fixed entrypoint and no Docker socket, mounts, capabilities, or
workload environment. Ports below 1024, duplicate host ports, protocol mismatches, and
endpoint sets beyond the hosted-service limit are rejected.

Container output uses Docker's bounded `local` log driver with 10 MiB files and
two-file rotation. This closes stdout/stderr disk growth without exposing logs
through the Ardents API. Image-layer storage remains node-operated and images
must already be present; workload start does not perform an implicit pull.

Public environment is limited to 64 entries, 4 KiB per value, and 64 KiB total.
Command and entrypoint argument counts and lengths are bounded. Working
directory must be an absolute clean container path.

## Explainability And Redaction

Admission errors use stable categories for invalid specification, denied image
provenance, denied trust policy, and exceeded resource/configuration limits.
They may identify the rejected field or registry host, but never echo an
environment value, credential, token, private path, or complete raw config.

Runtime inspection retains safe exit code/reason, OOM outcome, restart count,
and operator-action truth. OOM is reported as resource exhaustion. Docker errors
must be sanitized before reaching persisted status, diagnostics, API, or logs.

## Acceptance Boundary

The security contract is acceptable only when Linux container tests prove strict-schema
denial, registry and trust denial, no raw config exposure, host filesystem and
network denial, read-only filesystem behavior, capability/user bounds, process
and memory pressure outcomes, OOM truth, and error redaction. Absence of gVisor
on the development host remains explicit and prevents claiming untrusted
production eligibility.
