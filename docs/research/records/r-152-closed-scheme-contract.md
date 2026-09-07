---
id: R-152
title: Closed text-Service protocol, admission and confinement contract
status: completed
owner: Product Owner and Codex
started: 2026-09-07
reviewed: 2026-09-08
---

# R-152 — Is the selected closed scheme concrete enough for implementation?

## Decision this unlocks

Close the exact bounded contract left by ADR-0078: workload, receiving wire,
admission authority, supported components, local confinement, budgets, ownership
and migration. [ADR-0081](../../adr/0081-select-closed-protected-service-contract.md)
records the selection. The [implementation path](../../development/privacy-anonymity-map.md)
is the current reader route; this record supplies rationale and evidence.

## Current contract

The Product Owner selected complete closed-network testing first, confined
text-document publication/read on Ubuntu at both endpoints, and Target Link
before canonical Names. Residual correlation and no autonomous filler are
accepted initial limits. Public autonomy remains R-149. ADR-0054 prevents a
temporary alpha authority from masquerading as canonical naming.

The [protocol](../../technical/protected-route-protocol.md),
[admission](../../technical/private-admission.md),
[confinement](../../technical/application-confinement.md),
[workload](../../product/protected-service-workload.md),
[threat](../../security/threat-model.md#closed-successor-claim-contract) and
[qualification](../../development/privacy-qualification.md) owners are normative.
Current C0 remains generation 2 until explicit adoption/migration.

ADR-0048's v1 Carrier identifiers name its old ALPN and reciprocal LegBinding.
Reusing those identifiers for changed framing would silently redefine signed
State input. Select distinct v2 Carrier identifiers for the same TCP/TLS and
QUIC transport families. Authenticated generation-3 HELLO replaces that old
binding on new Node Carriers; bounded outer handshake allocation and inner
receiver admission are separate states. This is a compatibility design choice,
not a new transport dependency or additional claimed measurement.

## Hypotheses

- H1: standard TLS/HPKE plus reviewed blind RSA can implement the exact finite
  profile and receiver-local admission without a token bootstrap cycle.
- H2: the selected Ubuntu mechanisms can contain this fixed worker tree while
  retaining its one permitted local stream.
- H3: the complete causal schedule can fit NET-32 under explicit processing,
  transfer and queue assumptions without dropping required protection.
- H0: an incompatible format, escaping operation, circular authority or
  unavoidable model overrun invalidates this composition.

## Evaluation criteria

The experiment README fixed token/negative/positive-control and model rejection
rules before the relevant run. Imported cryptographic primitives, valid source
integrity, real closed authorities, per-context identities, finite durable
debits/spends, joined failure and exact owner boundaries are mandatory.
Design selection needs source and architecture-sensitive component evidence.
Qualification additionally needs the actual changed candidate and P1–P11;
a full implementation cannot be measured before it exists.

## Evidence plan

### Primary sources

Accessed/rechecked 2026-09-07:

| Source | Sourced fact relevant to the choice |
|---|---|
| [Go release history](https://go.dev/doc/devel/release), [security policy](https://go.dev/doc/security/policy) | Go supports the last two major lines. 1.26.8 is the current patch on the selected supported line; 1.26.6 component receipts do not test that patch. |
| [CIRCL v1.6.5 release](https://github.com/cloudflare/circl/releases/tag/v1.6.5), [tagged source](https://github.com/cloudflare/circl/tree/v1.6.5/blindsign/blindrsa), [license](https://github.com/cloudflare/circl/blob/v1.6.5/LICENSE) | Tagged BSD-3-Clause implementation of RFC 9474 variants with upstream vectors/tests. The module contains other experimental algorithms; this review selects only ordinary blindrsa and retained HPKE. |
| [CIRCL security reporting](https://github.com/cloudflare/circl/security/policy) | Cloudflare supplies a disclosure/reporting route. Absence of SECURITY.md in the tag is not absence of a reporting policy; no long-term branch-support guarantee is inferred. |
| [RFC 9474](https://www.rfc-editor.org/rfc/rfc9474.html), [RFC 9577](https://www.rfc-editor.org/rfc/rfc9577.html), [RFC 9578](https://www.rfc-editor.org/rfc/rfc9578.html) | Selected blind-RSA primitive and publicly verifiable token construction. Dedicated RSA-PSS SPKI bytes determine the token key ID; generic RSA serialization is not that encoding. |
| [quic-go v0.62.0](https://github.com/quic-go/quic-go/tree/v0.62.0) | Retained Go QUIC implementation; ADR-0048 already selects TCP/TLS and QUIC. Both must remain in the successor comparison. |
| [systemd 255 execution source](https://github.com/systemd/systemd/blob/v255/man/systemd.exec.xml), [socket source](https://github.com/systemd/systemd/blob/v255/man/systemd.socket.xml) | Installed service properties and socket activation can establish the selected process boundary. Socket-creation restrictions do not revoke inherited handles. |
| [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932) | Deprecated x/crypto/openpgp is unmaintained and unsafe. Its absence from a specified closure can support non-applicability; it does not exempt other versions, imports or tools. |

**Source inspection:** CIRCL's exact blindrsa import closure is its ordinary
blindrsa package, internal/common and internal/keys plus the Go standard library.
The local v1.6.5 module checksum verified. Its opaque blinding State is not a
durable interchange format; the selected protocol burns lost reservations
rather than serializing internals. Deterministic blind RSA is used only with
fresh high-entropy token nonces.

**Design selection:** Go 1.26.8, CIRCL v1.6.5 ordinary
SHA384PSSDeterministic, existing HPKE and quic-go v0.62.0, Ubuntu 24.04 LTS
systemd 255/cgroup v2. The existing library ownership contains the implementation;
there is no private crypto fork or new daemon dependency. A loss of upstream
support, applicable advisory, changed import/use/configuration or failed
candidate check invalidates the matching evidence. The exact updated Go/OS
candidate still needs current source/test/binary/package review before admission.

### Experiment and retained receipts

[Probe sources and run instructions](../../../experiments/r-152-contract-probes/README.md).
The original token/confinement evidence directory is:

C:/Users/vitek/AppData/Local/Temp/ardents-privacy-contract-e52a233bffad4cada157256707f4e4fa/

| File | SHA-256 |
|---|---|
| token_composition.go | 94bf9b2f336b5fc1ff706182f7583fa7a0db024e93fb46914471e50c7da4e8b6 |
| confinement_probe.go | c9cc855fc93ebc8e305e1a66c442235fb4da3e2514d48b635c14365d9d6f8dae |
| tokens.json | 0816e47691097fb310c5d2ec338e4e9d28aa66326acc71d529fa61a7a4ebee5d |
| baseline.json | 3cdcf82ff2d57418a4cdc744f707be54ff462b67b74295e759d488e6a10bb8c1 |
| sandbox.json | 5d44fb49bde5d2bfa1c20702538038429db85c040e1f2a02a82a0e184676a578 |
| summary.json | 949b36e1f8b2e67047c0581595f1571f9824d8c70c35bff62251009a6145fa5a |

Summary/configuration and raw vulnerability JSON are distinct evidence.
The retained db-live.json has zero bytes and is not a fresh-database receipt.
The scan identified govulncheck 1.1.4, Go 1.26.6 and database modification
2026-09-02T19:12:04Z. This is historical component screening, not a fresh
Go 1.26.8/artifact/Ubuntu admission. A future candidate must recheck current
advisory data, not use JSON exit zero or that empty file as approval.

### Failure scenarios

Tests must cover wrong profile/SPKI/receiver, altered token/capsule, duplicate
spend, reservation crash ambiguity, issuer compromise/flood, expired duties,
active lane/control interference, late asynchronous completion, worker/child
escape, inherited external descriptors, conflicting authority, and interrupted
adoption. The qualifier owns exact mandatory verdicts.

## Findings

**Measurement:** 64 composed blind-token round trips and 192 altered-message,
signature/receiver rejection checks passed with Go 1.26.6. RSA modulus was
2,048 bits with exponent 65,537; token/request/signature lengths were
354/259/256 bytes. Independent standard-library RSA-PSS verification agreed.
Local p95 was 3,118 microseconds and maximum 3,655 microseconds; neither is
network latency or a promised production CPU budget.

The selected RSA-PSS SPKI was 346 bytes versus 294 generic RSA bytes and had
a different key ID. **Sourced/encoding distinction:** the RFC's example uses
absent hash NULL parameters and a different length. Ardents deliberately pins
its exact encoding. Preserve original SPKI bytes when checking external RFC
vectors. This probe and the passing upstream RFC-9474 tests are component
evidence; they are not complete RFC-9578 deployment interoperability.
The actual token parser must pass the independent positive/negative vectors in P4.

**Measurement:** the Ubuntu 24.04.4 / systemd 255.4-1ubuntu8.14 probe on WSL
kernel 6.6.87.2 denied fourteen forbidden parent/child operations whose
unconfined controls worked. The local byte marker survived, with no effective
capabilities, NoNewPrivs=1 and seccomp enabled. The first launch failed before
Application execution because of its working directory; the corrected profile
sets WorkingDirectory=/. The positive-control correction is also retained.
These results do not exercise the complete installed socket/worker/Grant path.

The wrapper was subsequently made portable by taking explicit binary/evidence
paths and unique unit names. Its shell syntax passes Git Bash. An attempted
WSL syntax-only rerun in this workspace returned Wsl/Service/CreateInstance/
E_ACCESSDENIED; it is retained as an unavailable environment, not a repeated
confinement pass. The original component receipt remains separately identified.

**Measurement/source inspection:** Linux source-with-tests screening reported
GO-2026-5932 at module level only; the recorded import closure excludes openpgp.
The blindrsa Windows source/test scan reported no findings. A repeated
go mod verify and CGO-disabled upstream blindrsa test passed in this review.
Coverage is package/build specific. A changed compiler, import, platform,
tool or advisory needs reassessment; no global absence-of-vulnerabilities claim
follows, and no missing Ubuntu package inventory is declared checked.

### Causal cost model

**Assumptions before execution:** six physical links at 20 ms RTT each;
bounded optimistic handshake forwarding; parallel Rendezvous/Introduction
terminal preparation and JOIN/capsule delivery; current public State and ready
publication; same-context warm Node choice with Carriers retained from actual
work, always fresh terminal TLS/admission/join; minimum
planned token batches, with no private cold cache or token stock. The model
keeps Carrier/TLS setup, two issuance batches, prefix admission, Descriptor,
capsule delivery, Publisher join, Service TLS/authentication, application
completion, Terminal receipt and peer confirmation on their causal paths.
The initial InstanceChallenge/Proof and Continuity records are coalesced in
one generation-3 flight; every signature/MAC check still precedes Application
effects. Source inspection confirmed that C0 currently performs two sequential
exchanges. Reusing that old schedule would miss the modeled warm gate, so the
selected pipeline is explicit in the protocol and implementation brief.

Reserve 240 KiB cold transfer at 20 Mbit/s and 128 KiB warm at 100 Mbit/s,
plus 500/150 ms processing/launch and 250/50 ms queue allowances respectively.
Those byte/time reservations are falsifiable assumptions, not captures.
The 1.6% TLS-only result is not substituted for these complete categories.

**Arithmetic result**, Go 1.26.6, cost_model.go:

| Carrier | Cold at 20 ms/link | Warm at 20 ms/link |
|---|---:|---:|
| TCP/TLS | 2,248.30 ms | 930.49 ms |
| QUIC | 2,128.30 ms | 930.49 ms |

The warm model has only 69.51 ms headroom. At 40 ms/link both modeled
warm and cold gates fail; those are retained diagnostic envelope results.
No model value is a measured p95. Actual processor, loss, worker-launch and
queue behavior can invalidate H3. Run the retained NET-14AD normal cell and all
P8 measurements without hidden exchanges, changed thresholds or a weaker path.

Evidence:
C:/Users/vitek/AppData/Local/Temp/ardents-r152-final-design-model/cost-model-selected-handshake.json
SHA-256 579425ad82404363c33030ae40e4419ce7b4d1d8b3a016c80a0b652063b89de0.
Model source SHA-256:
5bcf9922749c04dbb489f9c6744988e1f7b918933a1cfb4f8a8ed9c0239896a9.

The intermediate cost-model-with-terminal-receipt.json is retained with SHA-256
c6e5dfcc75e3502eba756629702beb88e3b8b3924884fa1291258a0ec6db0c72.
It included Terminal control but had not made the initial Continuity scheduling
assumption explicit. The final source names that coalesced flight and the
protocol selects it; its numerical reservations and thresholds are unchanged.

The first cost-model.json, SHA-256
23f85efa324e5d30ec12dc38e586ef2d189ab9eb674367e59813bf4eeec43ddf,
omitted Terminal receipt/confirmation and is invalid as a complete schedule.
Its source is retained externally as cost-model-before-terminal-receipt.go,
SHA-256 d611b88d336fdb60dc83c7cd34a24a88b70836f5dc57f464df6cc129efd6f147.
Simply adding the omitted exchange exceeds the initial TCP warm budget.
The revised design permits bounded same-context Node choice/actual-work Carrier
reuse and concurrent JOIN/capsule delivery. It preserves the original time,
byte and processing assumptions and the mandatory authentication/receipt steps;
no failed first result is erased.
Keep source and parameters to reproduce arithmetic if temporary evidence is lost.

## Options

Select the finite closed authority/permission composition, scoped holder keys,
offline receiver spend, one-use joins, both accepted Carriers, one isolated
Publisher snapshot and fixed worker IPC. Retain public authority and canonical
naming as their real next-stage decisions.

Reject the draft TCP-only replacement without research, a shared cross-context
holder identity, eight-child/128-lane ceilings incompatible with retained
Publisher capacity, attachment-dependent logical contexts, a six-hundred-second
forwarding lease that truncates ten-minute trials after setup, and a temporary
canonical-name alias. These rejected instructions were removed from current
owners instead of being left as alternative implementation guidance.

## Recommendation

Choose the closed design under ADR-0081, with the current-owner contracts
complete enough for dependent implementation tasks. Confidence is in the
bounded construction and falsification plan, not demonstrated system behavior.
The strongest risks are traffic correlation, narrow warm-latency headroom,
upstream maintenance and correctness of the installed host boundary.

## Disposition

Closed design selection is complete. R-099's first job/platform choice is
resolved. Full public autonomy, canonical Names and stronger anonymity are not
declared solved. Preserve source/measurement evidence, remove duplicated
proposed specifications, and implement through the current owners and issue
ledger. Every slice has behavior checks; complete system qualification remains
required after implementation.
