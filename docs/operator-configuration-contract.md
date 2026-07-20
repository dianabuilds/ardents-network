# Ardents Operator Configuration Contract

## 1. Role And Ownership

This document defines the versioned operator configuration contract for the
`v1` node. It does not create a Configuration product domain.

- runtime assembly owns loading, normalization, change classification, and
  atomic application of the document;
- each product domain owns validation and behavior of its fields;
- Policy owns behavior-changing admission and denial rules;
- the canonical local control surface exposes only the effective, redacted
  snapshot and reload outcome;
- Waku remains the canonical network foundation. A transport profile selects a
  Waku-backed participation variant and never selects a different substrate.

## 2. Source And Precedence

The canonical source is a strict UTF-8 JSON document selected by
`ARDENTS_CONFIG_FILE`. JSON is used in `v1` so strict decoding, duplicate
unknown-field rejection, and deterministic redaction need no new parsing
dependency.

Resolution order, from lowest to highest precedence:

1. versioned product defaults;
2. the canonical JSON document;
3. the process-only API credential override `ARDENTS_API_TOKEN` or
   `ARDENTS_API_TOKEN_FILE`.

The legacy environment-only startup path remains available when
`ARDENTS_CONFIG_FILE` is absent. Once that file is selected, non-secret legacy
environment variables do not silently override it. New fields are added to the
versioned document first. Ambiguous API secret sources are rejected.

## 3. Document Identity

Every file must contain:

```json
{
  "api_version": "ardents.config/v1",
  "node": { "name": "node-a", "profile": "service_node" }
}
```

Missing, empty, or unknown `api_version` values are rejected before any data
directory, key store, transport listener, workload, or API server is opened.
Unknown fields are rejected with a bounded JSON path. Deprecated fields are
rejected with the replacement path; they never silently change semantics.

## 4. Canonical Sections

The `v1` document contains these typed sections:

| Section | Owning behavior | Representative fields |
| --- | --- | --- |
| `node` | Node Runtime / assembly | `name`, `profile`, `data_dir` |
| `api` | local boundary | `listen_address`, `token_file` |
| `network` | Network Foundation | Waku `transport_profile`, bind/listen, bootstrap, trust, DNS, reachability, advertised endpoints, WSS material references, abuse limits |
| `privacy` | Identity + Network privacy assembly | protected capability-store/key references, trusted issuer public keys, and separate discovery/data replay ledgers; never raw selector/channel material |
| `workloads` | Workload Control + Policy | executor, registries, policy refs, runtime names, ingress allow-list and proxy image |
| `services` | Hosted Services | declared service/probe inputs that still require runtime backing before publication |
| `data` | Data Substrate + Policy | data/store paths, local/relay TTL, storage and replica quotas, replica target/minimum |
| `policy` | Policy | workload, route, capability, publication, retention, pinning and reservation rules |
| `logging` | process observability | `level`, `format` |
| `observability` | read-only operator monitoring boundary | loopback `listen_address`, optional protected `token_file` for defense in depth |
| `diagnostics` | Diagnostics | bounded event retention and operator-visible detail level |

Fields that the runtime cannot enforce are not accepted merely for future
compatibility. Adding a field requires real mapping, validation, effective
inspection, and tests in the same slice.

## 5. Change Classes

Every accepted field has one change class:

### 5.1 Immutable

Changes are rejected for the lifetime of the persisted node identity:

- `node.name` once identity exists;
- `node.data_dir` once state exists;
- private-key, capability-ledger, and replay-ledger locations after creation.

The outcome is `rejected_immutable`; the running configuration is unchanged.

### 5.2 Restart Required

The new document may be validated and retained as a candidate, but it is not
applied to the running process:

- node and Waku participation profiles;
- bind/listen/advertised endpoints and WSS material;
- bootstrap and DNS discovery sources;
- local API listen address or credential reference;
- observability listen address or scrape credential reference;
- workload executor/runtime and ingress proxy shape;
- storage paths and hard capacity limits;
- privacy channel identity/scope/material references.

The effective snapshot remains the running version and lists the changed paths
as `restart_required`. A restart re-validates the complete candidate before
partial startup.

### 5.3 Safely Reloadable

The first `v1` reloadable set is intentionally bounded:

- Policy deny/allow rules and TTL ceilings;
- logging level;
- diagnostics retention/detail bounds;
- discovery refresh interval;
- soft network abuse limits where the Waku adapter supports atomic replacement.

Unsupported live mutation is classified as restart-required, never silently
accepted. Reloadable values must be applied through the owning service, not by
mutating a shared config struct behind it.

## 6. Validation

Validation is deterministic and completes before application. It includes:

- strict schema/version/unknown/deprecated-field checks;
- required secret references and regular-file checks without reading secrets
  into diagnostics;
- Waku node-profile, transport-profile, role, reachability, address, WSS, DNS,
  bootstrap, and abuse-limit constraints;
- privacy-required versus capability material and replay persistence;
- trusted-process executor only in `local_development`;
- workload registry, policy-ref, ingress, and runtime constraints;
- service endpoint/probe consistency without claiming publication readiness;
- retention TTL, storage quota, replica desired/minimum, and constrained-client
  storage compatibility;
- policy contradictions and TTL ceilings;
- logging and diagnostics bounds.
- observability binds only to a valid loopback TCP address; remote exposure is
  owned by the deployment boundary and must not be enabled by a bearer token alone.

Errors contain a bounded safe summary and the configuration field when that is
safe and actionable. Reload outcomes separately identify immutable and
restart-required paths. Errors never contain tokens, private keys, capability
material, raw selectors, protected filesystem paths, or secret file contents.

### 6.1 Private-channel provisioning shape

When `privacy.required=true`, the stopped node must already have its canonical
identity and an encrypted Identity-owned capability store provisioned through a
secure operator workflow. Runtime configuration references that store; it does
not import plaintext grants or create replacement secrets.

The privacy section contains one protected capability-store path, a separate
32-byte store-key file, the local identity subject, trusted issuer public keys,
and two channel bindings:

- `discovery` is fixed to `realm.discovery` and has an opaque local capability
  reference plus a durable replay-ledger path;
- `data` is fixed to `data.exchange` and has a distinct reference and replay
  ledger;
- `replay_key_file` is a separately protected 32-byte key used only for replay
  digests.

The daemon checks file type and private permissions, key length, issuer
principal/public-key binding, local identity/subject binding, grant validity,
scope, publish/subscribe/store permissions, and replay persistence before
constructing the node. `privacy.required=false` accepts no dormant privacy
material, so a typo cannot be silently ignored. Missing or wrong protected
material is a startup failure; there is no plaintext fallback.

## 7. Effective Configuration

The effective snapshot contains:

- schema version and monotonically increasing generation;
- normalized source fingerprint and load time;
- normalized non-secret values;
- secret fields represented only as `configured` or `missing`;
- active generation, validated candidate generation, and pending-restart paths;
- the last reload outcome and bounded safe reasons.

The snapshot never exposes API tokens, private keys, capability grants,
selectors, encryption material, environment secret values, or secret file
contents. Paths that reveal sensitive topology or identity storage are reduced
to a safe configured/not-configured state where appropriate.

## 8. Atomic Reload And Rollback

Reload follows one transaction:

1. read the selected source with a bounded size;
2. strictly decode, normalize, and validate the complete candidate;
3. compare it with the active effective generation;
4. reject immutable changes;
5. classify restart-required paths without mutating runtime state;
6. prepare every reloadable owning service;
7. commit all reloadable changes;
8. if any prepare or commit fails, restore every already-changed service and
   keep the previous active generation;
9. publish a redacted diagnostic outcome.

Outcomes are `unchanged`, `applied`, `restart_required`,
`rejected_invalid`, `rejected_immutable`, `rolled_back`, or
`rollback_failed`. `rolled_back` is valid only when every already-applied owner
confirmed restoration. `rollback_failed` means the runtime may contain a mixed
effective generation; the node becomes degraded, records an operator-action
reason, and must not claim the previous generation as operationally restored.
Reload is idempotent for the same normalized fingerprint.

## 9. Startup And Failure Truth

- The daemon validates the entire effective document before constructing or
  starting the node and before opening the local API listener.
- An invalid document is a process startup failure, not a degraded partially
  started node.
- A rejected or fully rolled-back live reload does not degrade already-correct
  running behavior; its diagnostic record explains the rejected candidate.
- A failed rollback degrades the node with
  `config.reload.rollback_failed`; operator recovery or restart is required
  because in-memory owners may no longer agree on one effective generation.
- A valid candidate with restart-required changes is visible but never claimed
  as active.

## 10. Acceptance

The contract is complete only when tests prove:

- defaults and a complete document map to real runtime behavior;
- unknown version/field, deprecated field, invalid combinations, missing secret
  reference, and oversized document fail before partial startup;
- effective inspection is deterministic and redacted;
- reload success changes behavior through owning services;
- invalid and mid-commit failures preserve the previous behavior/generation;
- rollback failure is distinct from successful rollback and degrades runtime
  truth instead of claiming restoration;
- immutable and restart-required changes produce distinct outcomes;
- restart activates a previously restart-required valid document;
- Docker integration covers the daemon and canonical local control surface.
