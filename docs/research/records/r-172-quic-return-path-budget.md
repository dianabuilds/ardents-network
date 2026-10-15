---
id: R-172
title: Current native QUIC return-path reply-budget admission
status: open
owner: Product Owner and Codex
started: 2026-10-15
reviewed: 2026-10-15
---

# R-172 — Does the current QUIC server enforce its return-path reply budget?

## Decision this unlocks

Resolve the applicability and supported disposition of
`ARDENTS-QUIC-2026-09-21-RETURN-PATH-REPLY-BUDGET` for the current receiving
Carrier. The selected [issue #519](https://github.com/dianabuilds/ardents-network/issues/519)
owns execution and complete acceptance. This record supplies hypotheses and
evidence, not a replacement implementation ledger or an admission waiver.

## Current contract

The [product scope](../../product/scope.md),
[threat model](../../security/threat-model.md),
[protected Route protocol](../../technical/protected-route-protocol.md),
[Node owner](../../technical/network-route-node.md),
[dependency register](../../development/dependencies.md#existing-quic-finding-return-path-reply-budget)
and [testing policy](../../development/testing.md#dependency-security-evidence)
remain authoritative. [ADR-0048](../../adr/0048-maintain-tcp-and-quic-carriers.md)
retains both Carriers; version admission belongs to the dependency register.
The [Route migration contract](../../development/route-migration-contract.md)
preserves R01–R08, including old/new isolation in tests and genuine consumers.

The candidate pins quic-go v0.63.0. Its current shared receiving listener
serves Forwarding, Issuer, Descriptor, Introduction and JOIN compositions.
Outgoing-only calls require a separate disposition. No private fork,
first-party QUIC or cryptographic primitive, generic socket firewall,
TCP-only fallback, changed admission policy or optional protection mode is
selected. Missing evidence keeps the finding unresolved.

## Hypotheses

- **H1:** an existing enforced boundary excludes the affected mechanism from
  every admitted server use. A permitted original authenticated connection
  reaching the supposedly excluded behavior falsifies this claim.
- **H2:** a maintained fixing version or supported configuration enforces the
  normative allowance throughout the admitted profile. Any cumulative
  per-address violation, uncovered admitted caller/backend, lost required
  both-Carrier behavior or unjoined physical work falsifies it.
- **H0:** no evaluated supported option meets the contract. Retain the blocker
  and a concrete proposal instead of reporting a partial pass as admission.

## Evaluation criteria

The independent normative oracle is RFC 9000 section 8: before address
validation, cumulative attributable UDP payload output to an address must
remain within three times its attributable input. Credit from a different
validated address cannot establish that bound. Sections 8.2.1 and 8.2.2
constrain probe padding by the same allowance. Keep UDP payload, IP headers,
segmented sends and original/candidate destinations distinct. Ambiguous byte
attribution or validation state is an indeterminate result, not a pass.

Protected right: finite return-path traffic allowance under the accepted
Carrier contract. Adversary: a malicious peer within the issue's declared
controlled test environment. Surviving conditions include original native
execution and trustworthy local observations. Measurements establish only
their exact artifact, configuration, platform and workload; no anonymity,
independent-security, whole-host or installed Node claim follows.

The candidate must preserve actual signed State/duty, original process and
producer, TLS custody, Receiving/Hosting, original resource profiles and joined
cleanup. Missing capacity or authority cannot select a weaker accepting path.
Maintenance, license, source/module integrity, current advisories and a
credible update/removal path are required alongside behavioral evidence.

## Evidence plan

### Primary sources

Read-only sources inspected on 2026-10-15:

- [RFC 9000, section 8](https://www.rfc-editor.org/rfc/rfc9000.html#section-8),
  [8.2.1](https://www.rfc-editor.org/rfc/rfc9000.html#section-8.2.1) and
  [8.2.2](https://www.rfc-editor.org/rfc/rfc9000.html#section-8.2.2).
- [quic-go v0.63.0 release](https://github.com/quic-go/quic-go/releases/tag/v0.63.0),
  [versioned public API](https://pkg.go.dev/github.com/quic-go/quic-go@v0.63.0)
  and [security policy](https://github.com/quic-go/quic-go/security/policy).
- [Official connection migration documentation](https://quic-go.net/docs/quic/connection-migration/).

### Accepted evidence method

Start with local source/configuration closure and existing ordinary tests.
Pin the original source, module sums, compiler and actual tested configuration;
keep all logs, generated files and binaries outside the repository. These
checks establish prerequisites only. Do not infer the finding's disposition or
a native/OOB measurement from them or from a scanner.

On 2026-10-15 the Product Owner accepted the Route owner's recommended composite
method, preserving both Carriers and the required fix/non-applicability. The
[testing owner](../../development/testing.md#quic-return-path-reply-budget-evidence)
defines the binding method: actual supported correction or enforcing boundary,
deterministic causal regression of the real dependency at send admission,
exact source/module/build/artifact correspondence, complete caller/config/TLS/
ALPN/backend and output-accounting closure, and ordinary native both-Carrier,
genuine Service/authority/owner-lifetime and cleanup checks. Exact equivalence
may justify one mechanism proof per caller group; distinct paths remain separate.

The original plan required active exact-native/OOB changed-address measurement,
including original/candidate address totals, validation and mapping controls.
That exclusivity is superseded by the accepted composite method; the original
measurement was never performed. Its absence and every earlier observation or
failure remain recorded. The composite method does not claim direct native
packet observations. Segmentation, retries and every other attributable send
path still need an explained accounting boundary; uncovered paths stay unresolved.
No active attack reproduction, additional privileges or host-setting changes
are selected. No changed-address reply-budget experiment has been performed here.

After evidence supports a remedy, complete normal dependency/architecture
acceptance before product edits. Require a causal regression, both Carriers,
genuine Service consumers and the complete source-matched gates, normal hook
and verified scoped dev delivery. Changed artifacts invalidate affected prior
receipts. No upstream report or message is authorized by this research.

### Failure scenarios

Include missing or diverging callers/backends; changed configuration or TLS;
ambiguous attribution; loss, delay, duplicates and incomplete validation;
original State, caller, process or Hosting loss; expiry, late writers and
incomplete join. Preserve refused startup and physical failures. No retry
erases a failed attempt or renews an original protocol deadline.

## Findings

- **Sourced fact:** the dependency register retains unresolved server admission
  on the present pin. The earlier v0.62.0 user-space observation is historical
  evidence with an explicitly narrower scope.
- **Source inspection:** the current shared listener opens a UDP socket and
  calls quic-go's Transport.Listen. This establishes source reachability only.
- **Source inspection:** the five new native role compositions call the same
  Receiver.Listen factory. Backend selection and actual execution still need
  their own evidence.
- **Source inspection:** the receiving composition in
  `cmd/ardents-next/route_linux.go` also consumes Receiver.Listen. The new outgoing
  paths in `prefix/prefix.go` and `receiver/receiver_linux.go` call the QUIC
  adapter's OpenEndpoint and OpenNode respectively; both use DialAddr rather
  than a server listener. The retained
  predecessor tree still has five shared receiving callers under `internal/node`
  and the direct credential-token listener under `internal/admission/issuer`.
  These are separate source obligations, not permission to connect the two
  runtime trees or resume predecessor implementation. A final disposition must
  identify every admitted server use or its actual retirement; covering only
  the five new compositions cannot silently disposition the retained callers.
- **Measurement:** no current native/OOB result has been obtained here.
- **Measurement:** existing portable transport tests, with race detection on
  Go 1.27.2 Windows amd64, pass for `route/transport`, `transport/tls` and
  `transport/quic`; the Route migration import/isolation policy tests pass.
  The initial restricted-environment attempt reported local socket access
  refusal and QUIC timeouts. The identical suite passed with authorized
  loopback access; the original failed attempt is retained. These are ordinary
  adapter controls, not measurements of the reply-budget finding or Linux OOB.
- **Measurement:** `make quick-check` completed successfully on the current
  documentation-only working tree with Go 1.27.2 Windows amd64. No gate or
  prerequisite was weakened.
- **Measurement:** the complete registered `make node-join-check` profile passed
  on Ubuntu 24.04.4 LTS under WSL2 kernel
  `6.6.87.2-microsoft-standard-WSL2`, systemd 255.4. The three native resource
  cases and all eight command workload groups executed; both TCP/TLS and QUIC
  passed their applicable cases. The runner retained its registered CPU,
  memory, task, file-descriptor and time bounds. A subsequent manager inventory
  found no remaining `ardents-join*` units, and all three artifact hashes still
  matched their independently supplied pins. This is original JOIN development
  evidence, not a reply-budget, actual OOB syscall, other-role or installed-host
  qualification result.
- **Sourced fact:** the official releases API still lists v0.63.0 as the latest
  published release on the access date. The compared upstream revision
  [54cf12e7](https://github.com/quic-go/quic-go/commit/54cf12e7c24dd65fae1559ddc8c87cab97fab0e2)
  is 36 commits ahead. Its complete `path_manager.go` is byte-identical to
  v0.63.0 and the inspected module source: 5,652 UTF-8 bytes, SHA-256
  `2b39a3304b7789c2c5e1d4fef11a3d002977326d4ea141c55cf09442c8712e8c`.
  The [bounded source comparison](https://github.com/quic-go/quic-go/compare/v0.63.0...54cf12e7c24dd65fae1559ddc8c87cab97fab0e2)
  also retains the path-probe packing/sending mechanism; inspected changes
  include MTU and packet-number handling, not an established per-address
  admission remedy. This source comparison does not reproduce a violation or
  establish that no unexamined remedy exists.

### Native baseline identity and reproduction

The reused canonical artifacts were built for the accepted runtime source
`55d0a2feb97aab86d1ac7ddce575fcf0d240370d`. The checked source at
`1ad8b47090f76bae4d94951bce601f0cb1667bee` changes only four Markdown owners;
this record and the research queue are the only additional working-tree changes.
No runtime, profile, runner, module or test source changed. Reuse establishes
that unchanged runtime baseline; it is not a new build of a repaired candidate.

| Runner input | Independently checked SHA-256 |
|---|---|
| `ARDENTS_NODE_NATIVE_COMMAND` | `f5004a6a4bbeabf7d891f86400e8a4bf501bbb55f3d48f93d29e6a27a6326749` |
| `ARDENTS_NODE_NATIVE_TEST` | `71698a17d5d5caca26ad61f9f4aae01017f765674cb4abfcb6864e0473c2c8d0` |
| `ARDENTS_NODE_RESOURCE_TEST` | `32231913828096e6225c0b560c455e2b1a46b21fa0150d7654da401276d3895f` |

Set those inputs to the original executable absolute paths and their matching
`_SHA256` variables, then invoke `make node-join-check` from the source-matched
Linux tree with its registered root-driver prerequisites. The maintained runner
performs exact test discovery and artifact verification. Raw local receipts,
including `native-join-baseline01.log` and its terminal status, are retained
outside the repository under the quality evidence directory `quic519`.

### Evidence still required for admission

1. Actual-component causal proof under the accepted composite method, tied to
   the canonical server's reviewed implementation and complete native backend
   output accounting. The present JOIN run supplies no proof of the reply-budget
   invariant. Direct native/OOB measurements remain absent; they are no longer
   the only admissible proof method. Ordinary client success, a behavioral model
   or unexplained backend assumptions cannot fill the gap. No tool was installed
   or host setting changed to make evidence appear available.
2. Complete current server-caller, TLS/ALPN, configuration and backend closure,
   with causal controls and the selected issue's bounded lifetime/cleanup matrix.
   A shared factory alone does not establish coverage of every caller. Outgoing
   client use needs its own narrowly stated applicability disposition.
3. A supported maintained remedy or a demonstrably enforced non-applicability
   boundary. The inspected release and bounded upstream comparison have not
   established one. No dependency/configuration change is justified yet.
4. After that decision, any accepted repair and causal regression, updated
   dependency disposition, source-matched both-Carrier/native/race and genuine
   Service-consumer checks, full gates, normal hook and verified scoped dev
   delivery. Local prerequisite passes cannot close #519, #496 or #497.

## Options

### Supported configuration review

**Source inspection, 2026-10-15:** the complete public `Config` and `Transport`
declarations in the pinned module were inspected, together with their relevant
call sites. The following controls do not establish the required boundary:

| Control | Actual responsibility | Why it is not a demonstrated remedy |
|---|---|---|
| `Transport.VerifySourceAddress` | Initial connection Retry/address-token validation; `server.go` invokes it in new-connection handling | It does not supply a decision for each later peer-address change |
| `Config.GetConfigForClient`, `Transport.ConnContext` | Incoming connection admission, configuration and original lifetime | No documented per-address byte allowance or per-send admission result |
| `Config.DisablePathMTUDiscovery` | MTU discovery, separately conditioned by the socket's DF capability | It does not disable the independent server path-validation mechanism |
| `InitialPacketSize` | Minimum/initial packet size; values below 1200 are invalid | A size setting is not cumulative per-address input/output accounting |
| Stream windows/limits, disabled datagrams/0-RTT, handshake capacity | Application stream, memory and initial-handshake bounds | They do not account for post-handshake path-control output |
| GSO/ECN backend controls | Socket optimization/capability selection | They provide no documented reply-budget boundary; changing the backend also invalidates affected evidence |
| Connection-ID length | QUIC demultiplexing; zero on the current shared Transport selects a default length | It is not a documented migration-disable or reply-budget control |

The [migration documentation](https://quic-go.net/docs/quic/connection-migration/)
describes automatic server handling of NAT rebinding. RFC 9000 section 9 also
distinguishes that event from voluntary active migration. Advertising
`disable_active_migration` alone therefore cannot be used as a finding waiver;
the public Config inspected here does not expose that parameter as a server
setting in any case. The [optimization documentation](https://quic-go.net/docs/quic/optimizations/)
does not turn GSO or MTU controls into security admission controls.

**Inference:** none of these inspected supported knobs supplies a justified
configuration-only fix. This rules out the named proposals, not every possible
future supported mechanism. Closing a connection after an application observes
an address change would not itself prove that earlier transport output was
bounded. No such callback, wrapper or host rule is introduced here.

### Nearby upstream changes

The official release and branch APIs were rechecked on 2026-10-15: the latest
published release remains v0.63.0 and the inspected master remains
`54cf12e7c24dd65fae1559ddc8c87cab97fab0e2`. A bounded public PR search returned
32 matches for amplification/path-validation/path-budget terms; both result
pages were read. Search completeness is not proof of the absence of a fix.

The actual changed-file patches, rather than their generated summaries, were
inspected for these two open, unmerged proposals:

| Proposal and pinned head | Inspected change | Disposition for this finding |
|---|---|---|
| [quic-go #5930](https://github.com/quic-go/quic-go/pull/5930), `768ef17e1b0c9338385476e389d34b77df6553cc` | Retry unanswered path challenges with backoff; retain multiple challenge responses; adjust path-manager and rebinding tests | No per-address received/sent byte ledger or corresponding send-budget gate is added by the inspected patch; not a demonstrated fix |
| [quic-go #5929](https://github.com/quic-go/quic-go/pull/5929), `af3a1cd0b6bbe88fd0ee2193ac90016eaf5f48ff` | Apply the candidate-path count limit only to genuinely new paths; add path-count regression cases | A path-count limit is not the normative cumulative byte allowance; not a demonstrated fix |

Neither PR was built, executed, adopted or copied. Their unit assertions cover
their stated neighboring defects, not the reply-budget invariant. The already
inspected [MTU-after-path-change change #5887](https://github.com/quic-go/quic-go/pull/5887)
also cannot substitute for that invariant.

### Alternative implementation screen

The already indirect module `golang.org/x/net v0.60.0` contains a separate QUIC
package. Its [versioned upstream package documentation](https://github.com/golang/net/blob/v0.60.0/quic/doc.go),
also read from the current module cache, explicitly excludes production
readiness and Go security-policy coverage. It also documents missing migration
and MTU features. Existing indirect module presence does not admit that unused
package or qualify it as a replacement Carrier. It is not selected as a remedy.
This is a screen of the named version, not a completed alternative-technology
evaluation or a new research selection.

### Concrete remediation proposal and ordinary regression

**Proposal, not an accepted dependency decision:** retain the current blocker
and prefer a maintained upstream release that implements the existing RFC 9000
per-unvalidated-address allowance inside QUIC's own path/send ownership. Require
the release's actual patch and tests to show that validation of one address
does not release a different address's limit, and that every attributable send
including path-control traffic respects the remaining allowance before physical
output. MTU padding, retransmission and backend segmentation must preserve that
same accounting. Route, Admission and Hosting keep their current policies.

The preferred ordinary first regression is deterministic and in memory at the
maintained dependency's actual send-admission boundary. Review or run the fixed
release's own unit cases for zero/partial/exact remaining allowance, cumulative
output, independent addresses, validation transitions, duplicate observations,
send failure and joined cancellation. Its oracle observes the real component's
accepted or refused output; it must fail for the original unbounded behavior.
No raw socket, spoofed source, reflection target or load workload is needed for
this unit layer. No such reproduction was run in this review.

The current dependency exposes no supported public test seam for Ardents to
replace that internal rule. A first-party model-counter test, copied path
manager or modified module cache would test a different implementation and is
not proposed. No executable regression for a repaired candidate can honestly be
supplied until a maintained candidate with that real seam or its own causal
regression exists. The inspected PRs do not provide one for this finding.

After a qualifying candidate exists, bind Ardents' existing ordinary adapter,
owner/lifetime and both-Carrier consumer regressions to that exact candidate;
repeat only the affected source-matched checks through normal integration.
The accepted composite native/backend profile proof remains outstanding. Unit
evidence alone cannot replace its other layers. Direct native/OOB measurement
remains absent and is not claimed by the method. Active attack reproduction, expanded
privileges and host-setting changes are outside this analysis step; no upstream
message or automatic experiment is authorized or scheduled by this proposal.

### Product Owner discussion: evidence method and Carrier scope

On 2026-10-15 the Product Owner requested discussion with the Route owner about
possible requirement changes. A read-only consultation checked the current
owners and returned these distinct alternatives. After explicit clarification
of methodology versus Carrier scope, the Product Owner answered
«я согласна с твоим выбором» and selected the recommended first alternative.
The alternatives below preserve that comparison; only composite methodology
is accepted. No Carrier or protection change follows.

| Alternative | What changes | What remains unresolved |
|---|---|---|
| Composite evidence, retaining both Carriers | Replace mandatory active native reproduction with actual maintained-fix inspection, causal in-memory regression of the dependency, exact source/artifact/config/backend correspondence and ordinary native consumer/cleanup evidence | No suitable supported fixing candidate is established yet. Unexplained backend output or accounting remains uncovered; successful baseline tests alone are insufficient |
| Defer affected-use admission | Keep the current contract and explicitly retain the dependency blocker while separately prepared components remain ready at their actual boundary | #519 and full Route acceptance remain incomplete; repeating baseline cannot remove that condition |
| Change the selected closed-release Carrier scope | A separately accepted product/architecture decision could select TCP/TLS alone for that release, with enforced exclusion of the unadmitted QUIC path | This removes QUIC capability from the selected release; it does not fix or qualify it. Actual execution, signed State/profile compatibility and the complete supported journey need corresponding checks |

The proposed composite method can share one mechanism proof across callers only
after an exact closure groups equal dependency versions, configuration, TLS/ALPN,
socket/backend and build settings. Each distinct path needs separate evidence.
Authority, spend, lifetime and cleanup role regressions remain role-specific.
The retained predecessor listeners do not inherit successor evidence and may
not be joined through a migration bridge.

The composite method must explain all attributable output through segmentation,
retry and alternative send paths. It establishes a component mechanism and its
presence in the product, not a direct measurement of every native UDP/OOB event.
An unexplained output path cannot be labelled covered. Native original-profile
observations absent from this record stay explicitly absent.

For this accepted methodology, the owning change revises this finding's
specific closure paragraph in `dependencies.md`, clarify evidence composition
and caller grouping in `testing.md`, preserve the original missing measurement
here, and update #519's experiment matrix and completion items 2 and 4. Keep
completion item 3 (supported fix or enforceable non-applicability), full gates,
scoped verified delivery, NET-30 and Route R01–R08. The direct Product Owner
decision replaces the explicitly requested proof method; it does not reduce
the required protection or treat preparation as completed admission.

Changing Carrier scope instead needs a superseding accepted ADR for the affected
ADR-0048/ADR-0081 selections, followed by consistent product scope, protected
Route, State/profile compatibility, qualification and #496/#519 criteria. A
State-selected QUIC attempt must not silently fall back to TCP, and leaving an
accepting QUIC listener reachable would not establish exclusion. This is a
proposal to change the product, not permission to weaken the RFC 9000 allowance,
accept an unresolved finding or create an optional protection mode.

The accepted choice is composite methodology with both Carriers. The changed
Carrier scope remains an unselected alternative. Checkout ownership, code,
dependency pin and protection limits remain unchanged. The dependency and
testing owners now contain the accepted method; issue execution and delivery
status remain in the selected tracker.

## Recommendation

Apply the accepted composite methodology and choose none of the inspected fixing
candidates yet. Retain the dependency pin and admission blocker while reviewing
a maintained supported correction or actual enforcing boundary. The inspected
configuration knobs, two nearby open PRs and screened alternative do not establish
a remedy. Confidence in final admission is insufficient: no qualifying supported
candidate, causal real-component proof or complete backend/caller evidence has
been established. Completed native baselines need no repeat merely for this
method change; required gates for delivery of the owning documents still apply.

## Disposition

Open question with an accepted evidence methodology. The selected issue owns
admission, current execution and delivery status. This record changes no runtime,
dependency pin, wire identity or safety bound. Full Route and project transition
acceptance remain separate; accepting the method does not close the finding.
