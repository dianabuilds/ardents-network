# Communication Contracts

## 0. Role In Docs Set

This is a supporting contract-discipline document.

It refines communication shape across:

- the canonical local control surface;
- discovery wire contracts;
- data wire contracts.

It does not override the core architecture documents or domain ownership
defined in `docs/domains/`.

## 1. Purpose

This document fixes the required communication contract shape for Ardents `v1`
across:

- the canonical local control surface;
- signed discovery traffic on the canonical `Waku` foundation;
- signed data-transfer request/response traffic on the canonical `Waku`
  foundation.

It does not replace domain requirements. It defines the contract discipline
that those domains must follow so request, response, status, and error
semantics do not drift by surface.

## 2. Contract Layers

Ardents must treat communication as three distinct but aligned layers:

### 2.1 Local Control Surface Contract

This is the operator-facing contract exposed through the canonical local API.

It is responsible for:

- operator-visible status and runtime truth;
- canonical command/query result projection;
- canonical error projection;
- diagnostics-compatible explainability.

The v1 local-control security boundary is a single-admin, token-authenticated
loopback surface:

- plaintext binding is accepted only on IPv4/IPv6 loopback or `localhost`;
- remote plaintext and wildcard binds fail during configuration, before the
  node runtime starts;
- remote TLS ingress is not currently a supported mode and must be designed as
  an explicit secure profile rather than enabled with a bind-address override;
- the local administrator receives an explicit domain capability set; wildcard
  authority is not used by the product assembly;
- server-side tokens may come from one environment value or one regular secret
  file, never both; Unix token files must deny group/other access;
- request bodies, headers, header reads, unary execution, writes, and idle
  connections have finite limits; the node-event stream is explicitly exempt
  from unary execution and server write deadlines and remains bounded by its
  authenticated client context;
- authentication failures and mapped domain errors must not echo credentials,
  raw authorization headers, payloads, or internal error strings.

### 2.2 Discovery Wire Contract

This is the signed network contract for discovery publication and intake on top
of `Waku`.

It is responsible for:

- signed canonical node/service records;
- freshness and expiry fields that are part of the signed network truth;
- deterministic import, validation, and merge behavior.

It must not carry local-only provenance state as if it were canonical network
truth.

### 2.3 Data Wire Contract

This is the signed network contract for data announce/request/response flows on
top of `Waku`.

It is responsible for:

- versioned message kind;
- request/response correlation;
- signed sender identity;
- machine-usable status and error semantics;
- privacy-safe carrier usage.

## 3. Non-Negotiable Rules

### 3.1 Do Not Collapse Layers

The system must not use one generic envelope for every communication path.

Local operator API, discovery wire, and data wire have different ownership and
different constraints. They must stay separate contracts.

### 3.2 Keep Shared Semantics Aligned

Even though the contracts are separate, they must align on:

- explicit operation/result status;
- deterministic error codes/categories;
- versioning discipline;
- explainable degraded behavior.

### 3.3 No Local Provenance In Network Truth

Fields whose meaning is local-only, such as:

- import source classification;
- local seen-at timestamps;
- cache/bootstrap provenance labels;

must not be treated as canonical signed network payload fields.

They may exist locally after intake, but they must be derived or attached by
the receiving node, not accepted as authoritative network truth.

### 3.4 Signed Fields Must Cover Runtime-Critical Meaning

If a network field affects:

- freshness ordering;
- expiry;
- authorization;
- identity binding;
- payload meaning;
- request/response matching;

then that field must be inside the canonical signed payload for that wire
contract.

### 3.5 Errors Must Be Structured At Product Boundaries

At product-facing boundaries, errors must not degrade into ad-hoc freeform
strings as the primary contract.

String details may exist for explanation, but machine-usable fields must carry:

- stable code;
- stable category;
- retryability where relevant;
- operation/domain context where relevant.

## 4. Local Control Surface Contract

### 4.1 Result Shape

All canonical local control surface RPC methods must return one of two forms:

- `status + result payload`
- `status + collection payload`

Read/list methods must not arbitrarily omit `status` for some resources while
including it for others.

### 4.2 Status Semantics

`status` must always represent operator-visible outcome for that method call.

It must include:

- `state`
- `reason`
- `accepted`

`accepted` is required even for reads so the result shape remains stable across
query and command flows.

Network status must expose participation and ingress truth independently. Its
reachability projection includes the configured mode, observed state, stable
reason, and observation time. It must not derive public reachability from peer
count, listener presence, or configured advertised addresses.

### 4.3 Error Semantics

The canonical local error model must include:

- `code`
- `category`
- `message`
- `domain`
- `operation`
- `retryable`
- `reason`
- optional `details`

Transport bindings such as Connect RPC may project these into transport-native
status codes, but they must preserve the canonical error detail.

## 5. Discovery Wire Contract

### 5.1 Canonical Payload

The discovery wire payload must be the canonical signed discovery record only.

The network payload must not wrap the record with local-only fields such as:

- `source`
- `seen_at`
- local cache/import annotations

### 5.2 Signed Discovery Fields

The canonical signed discovery record must include every field that affects:

- identity binding;
- record ownership;
- routing meaning;
- freshness;
- expiry.

At minimum this includes:

- `id`
- `kind`
- `subject`
- `node`
- `device`
- `owner`
- `service`
- `mode`
- `public_key`
- `endpoints`
- `issued_at`
- `expires_at`

The signature must be computed over those fields in canonical form.

### 5.3 Intake Provenance

After a record is received, the local node may attach intake metadata such as:

- local source classification;
- local seen-at timestamp;
- local quarantine/trust state;

but those values are local observations and must not participate in remote
signature verification.

## 6. Data Wire Contract

### 6.1 Versioned Envelope

Network request/response messages for data traffic must converge on a versioned
message envelope.

The envelope must contain:

- `version`
- `kind`
- request/response correlation fields
- sender identity fields required for verification
- payload or payload reference
- structured status/error fields
- signature

### 6.2 Structured Response Status

Response contracts must not rely on freeform text as the only failure signal.

They must expose:

- a stable status value;
- a stable error code when status is non-success;
- optional human-readable detail text.

### 6.3 Privacy Rule

If the traffic carries private product meaning, readable meaning must not
appear in content topics, filter selectors, or other network-visible routing
labels.

Readable identifiers in payloads are acceptable only where the current product
surface explicitly treats the flow as non-private and the owning documents
allow it. Otherwise the flow must move toward opaque selectors and encrypted
payload envelopes.

## 7. Acceptance Criteria

The communication-contract slice is not complete until:

- local control surface methods use one stable result envelope discipline;
- canonical local errors remain structured across the local API and Connect
  projection;
- discovery publish/fetch uses canonical signed records instead of local entry
  wrappers;
- discovery signatures cover freshness/expiry fields;
- data request/response flows use explicit, versioned, structured message
  semantics;
- diagnostics and local surfaces continue to explain degraded behavior without
  leaking private selector material.

### 7.1 Privacy Status Projection

`GetNetworkStatus` exposes the active `ardents-private/1` profile separately
from the transport profile. The stable privacy fields are `privacy_state`,
`privacy_switch_reason`, `privacy_recovery_state`, and
`privacy_error_categories`; unavailable operations are also appended to
`reduced_capabilities` as `private_publication`, `private_discovery`, or
`private_data_exchange`.

The projection may expose only stable `privacy.capability.*` categories. It
must never include capability references, subjects, selectors, keys, store
paths, resolver error text, or decryptable material. A configured capability
that currently fails resolution is `recovery_pending`; a missing channel
configuration is `blocked`; a fully resolvable profile is `steady`.

The same snapshot exposes `node_profile` separately from `active_profile`
(transport families) and `active_mode` (dynamic runtime state). The three
fields must not be substituted for one another; their valid combinations are
defined by `docs/network-participation-profiles.md`.

The network snapshot also exposes bounded-admission truth through
`abuse_state`, `abuse_reason`, `rate_limited_operations`,
`backpressured_operations`, `oversized_messages`, and `banned_providers`.
Counters are process-lifetime observations. Provider identifiers, addresses,
content topics, payloads, and ban-map keys are never projected. A non-zero ban
count makes the abuse state degraded until its temporary entries expire.
