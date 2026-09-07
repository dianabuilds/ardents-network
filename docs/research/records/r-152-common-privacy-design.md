---
id: R-152
title: Selection of the common privacy architecture
status: completed
owner: Product Owner and Codex
started: 2026-09-07
reviewed: 2026-09-08
---

# R-152 — Which privacy architecture fits useful ordinary Service work?

## Decision this unlocks

Choose the common construction, then close the bounded product/protocol
contract before implementation tasks. [ADR-0078](../../adr/0078-select-common-split-circuit-privacy.md)
selects the architecture; [ADR-0081](../../adr/0081-select-closed-protected-service-contract.md)
and the [contract assessment](r-152-closed-scheme-contract.md) complete the
closed Link-first text-Service design. This record retains selection evidence,
not a second wire specification or current backlog.

## Current contract

The [workload](../../product/protected-service-workload.md),
[threat model](../../security/threat-model.md#closed-successor-claim-contract),
[common architecture](../../technical/common-privacy-architecture.md),
[dependency rule](../../development/dependencies.md) and
[qualification](../../development/privacy-qualification.md) own the current
requirements. NET-29–33 require one common baseline, maintained components,
useful work, bounded response/idle cost and honest protection limits.

The Product Owner accepts proceeding with residual both-end/active correlation,
without an autonomous useless-traffic generator; selects complete closed-network
testing before public autonomy; selects confined text publication/read on
Ubuntu at both endpoints; and selects Target Link before canonical Names.
These decisions do not qualify anonymity or supply independent operators.

## Hypotheses

- H1: a complete mixing composition fits the interactive workload and its cost.
- H2: split low-latency circuits with confidential control and scoped admission
  can meet the selected field-separation/authority contract at feasible cost.
- H3: an unchanged full-capacity filler schedule can also fit the idle ceiling.
- H0: none of the examined constructions fits the combined contract.

Small additional cost without material protection loss remains a hypothesis
for the actual implementation; no general overhead percentage is established.

## Evaluation criteria

Assess the whole operation, both endpoints, every control/data role, local
execution, resources, maintenance, failure and migration. Reject a hidden
direct contact, forged proof/currentness callback, operator independence
inferred from keys, unbounded work or first-party cryptographic primitive.
Existing NET-14 gates remain unless an explicit researched decision replaces
one. A component or arithmetic pass cannot qualify the complete system.

## Evidence plan

Primary sources were accessed on 2026-09-07; repository inspection used HEAD
0dd9fc09d9cf4938132d1ac43284c144cda10e1d plus the existing design working tree.
The disposable [TLS](../../../experiments/r-152-layered-tls/README.md),
[arithmetic](../../../experiments/r-152-privacy-envelope/README.md) and
[contract](../../../experiments/r-152-contract-probes/README.md) probes retain
their predeclared checks and reproduction inputs. Captures and generated
artifacts stay outside Git.

Failure coverage includes colluding role observations, active delay/drop,
sparse and repeated traffic, directory equivocation, Sybil/flood pressure,
Application escape, key capture, cancellation, journal ambiguity and migration.

## Findings

### Architecture selection and component evidence

**Source inspection:** generation-2 SealedIntroduction exposes Rendezvous,
join handle and handshake context in its authenticated visible prefix.
The successor moves this material into the Service-only capsule and removes
reusable data joins from the Descriptor. This is a concrete exposure change.

| Primary source | Decision-relevant sourced fact |
|---|---|
| [TLS 1.3, RFC 8446](https://www.rfc-editor.org/rfc/rfc8446.html) | Maintained authenticated ephemeral channels can protect contents; traffic analysis and endpoint logging remain separate. |
| [OHTTP, RFC 9458](https://www.rfc-editor.org/rfc/rfc9458.html) | Relay/Gateway separation depends on deployment. Replacing its transport does not replace proof/authority validation. |
| [Privacy Pass architecture](https://www.rfc-editor.org/rfc/rfc9576.html), [authentication](https://www.rfc-editor.org/rfc/rfc9577.html), [issuance](https://www.rfc-editor.org/rfc/rfc9578.html) | Public token verification avoids contacting the issuer at every spend; entitlement, consistency, replay and correlation require a deployment contract. |
| [Tor onion services](https://community.torproject.org/onion-services/overview/index.html), [vanguard paths](https://spec.torproject.org/vanguards-spec/path-construction.html) | Introduction/rendezvous are useful precedents; failure-based path exclusions can expose guard choices. Ardents does not inherit Tor's topology or guarantee. |
| [Introduction intersection preprint v2](https://arxiv.org/html/2602.23560v2) | The authors report repeated-probe attacks on their Tor setting. This is an attack-test lead, not a demonstrated Ardents exploit or evaluated defence. |

**Measurement:** Go 1.26.6 on Windows amd64 ran twenty fresh memory-pipe
stacks with 100 verified request/response exchanges each.

| Nested TLS channels | Fresh handshake bytes | Exchange bytes for 66,048 useful bytes | Maximum record-byte addition |
|---|---:|---:|---:|
| 1 | 3,171 | 66,158–66,312 | 0.40% |
| 2 | 6,452 | 66,356–66,576 | 0.80% |
| 3 | 9,843 | 66,576–66,840 | 1.20% |
| 4 | 13,344 | 66,840–67,104 | 1.60% |

The four-layer maximum adds 1,056 exchange bytes and separately 13,344 setup
bytes. The experiment omits IP/TCP/QUIC headers, acknowledgements, real Nodes,
admission, lookup, Introduction, framing, padding, retries and background.
It measures neither latency nor anonymity. An initial quick-check failed
because a disposable Go file lacked its build-ignore tag; the corrected run
passed. The failed attempt remains part of the evidence.

**Inference:** this supports choosing maintained TLS for further composition;
it does not establish total overhead. The full closed selection and remaining
implementation evidence are in the contract assessment.

### Low-overhead references and design direction

**Sourced comparisons, not selected dependencies:**

- [Tor connection padding](https://spec.torproject.org/padding-spec/connection-level-padding.html)
  estimates 103 aggregate bytes/s for its described mechanism, about 8.90 MB/day.
  That is neither all Tor traffic nor a transferred Ardents protection result.
  Tor retains [both-end correlation limits](https://support.torproject.org/about-tor/security/attacks-on-onion-routing/).
- [MUFFLER v2](https://arxiv.org/html/2504.07543v2) reports low added traffic in
  its passive-observer Tor/proxy evaluation. Its kernel/eBPF composition,
  traffic population and single-flow limitations do not supply Ardents's
  active/sparse/combined-observer claim.
- [Loopix](https://www.usenix.org/conference/usenixsecurity17/technical-sessions/presentation/piotrowska)
  and [Echomix v2](https://arxiv.org/html/2501.02933v2) study different messaging,
  cover and scheduling compositions. A packet encoder or shortened mix delay
  cannot transfer their whole-system result to an interactive Service stream.

**Inference:** information minimization and bounded actual-work multiplexing
fit the selected scope. A new padding/shuffling defence needs its own measured
benefit and cost before adoption. None of these references creates a delivery
task, parallel mode or requirement to manufacture a traffic crowd.

## Options

Choose split circuits with maintained TLS/HPKE, private role operations and
blind admission. Reject embedding a separate complete network/authority system
or selecting an experimental packet cipher merely to reduce assumed overhead.
Reject full-capacity filler for this product contract. Defer a general browser,
public entitlement and stronger correlation claims to their actual scopes.

## Ordinary operation and measurement contract

The [workload](../../product/protected-service-workload.md) defines one fixed
512-byte request and complete 64 KiB reference response, with a 4 MiB document
maximum. [Qualification](../../development/privacy-qualification.md) defines
cold/warm state, complete clock, real quotas, both Carriers, platforms and
3 s/1 s reference gates. These are selected acceptance criteria, not results.

### Cost model for the ordinary workload

Count delivered Application bytes separately from all endpoint and intermediate
Node ingress/egress. Attribute setup, control, padding, retransmission, failed
attempts and background independently and reconcile their sum. A forwarded
byte is received and sent. A quiet direction cannot offset a failed direction.

The causal [contract model](../../../experiments/r-152-contract-probes/cost_model.go)
retains every mandatory exchange in the selected sequence. It uses explicit
CPU/transfer/queue assumptions and is not a percentile measurement.
The complete cost and limits are reported in the contract assessment.

### Protection contract to resolve

The selected closed properties are now in the
[five-part claim table](../../security/threat-model.md#closed-successor-claim-contract).
Field confidentiality, exact Target/authority, context separation, confinement
and bounded fail-closed behavior are hard gates. Participation/activity and
both-end/active correlation remain visible limitations. No numeric anonymity
resistance has been qualified. A new stronger claim requires new predeclared
adversary/population/metric evidence; it is not part of this completed selection.

## Retained exploratory evidence

### Arithmetic findings

**Arithmetic measurement:** Python 3.12.12 ran the declared unchanged-rate
model. Its source and rejection rules are retained in the arithmetic experiment.

| Model input | Result | Scope |
|---|---:|---|
| 1,000,000,000 bytes / 24 h | 92.593 kbit/s aggregate | The idle ceiling includes both directions. |
| Equal unchanged rates under that cap | 46.296 kbit/s per direction | Optimistically excludes control/framing. |
| Constant 10 Mbit/s each way | 216 GB/day | Rejects that filler hypothesis, not useful 10 Mbit/s capacity. |
| 1,024-byte symmetric Poisson opportunities | 0.530 s p95 first-slot wait | Excludes network and later exchanges. |
| 2,048-byte opportunities | 1.060 s p95 wait | Rejects that unchanged schedule for a 1 s result. |
| Same with 20% reserved for other work | 1.325 s p95 wait | Still that particular model. |
| Nine exponential holds with 50 ms means | 0.722 s p95 holds-only delay | Cannot be added to other p95 values as an end-to-end p95. |

**Inference:** H3 fails for the full-capacity reference. Sending on useful
demand leaves the unchanged-rate hypothesis. This neither proves correlation
resistance nor rejects every low-overhead design. No adopted protocol parameter
or implementation prerequisite is derived from this rejected schedule.

Evidence root:
C:/Users/vitek/AppData/Local/Temp/ardents-privacy-design-f4ad4e86eb4e4323ae437b6bd8f20b59/
Receipt envelope/envelope.json SHA-256:
74ce2f38cafbee9c6ee306b034d0e776fa6295eefbba8aa723891762d80be139.
Source SHA-256:
dc48b2cafaa345863b8116e5565f32ffd35029e2e4c569d28a5528a8bbddeef1.
The checked-in model regenerates arithmetic if temporary evidence is lost.

## Recommendation

Choose H2 for the selected bounded closed scope under ADR-0078/0081.
The strongest limitation remains combined traffic correlation; the full
implemented cost, installed boundary and hostile behavior still require P1–P11.

## Disposition

Closed architecture/design selection is complete. Canonical Names, autonomous
public authority and stronger anonymity claims remain separate future decisions.
Current behavior belongs only in the promoted owners. Retain the three
disposable experiments and their honest evidence limits. No maintained subsystem
or public deployment is created by this documentation change.
