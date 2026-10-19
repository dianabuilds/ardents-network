---
status: accepted
date: 2026-10-19
supersedes: ADR-0048 and ADR-0081 only for the Carrier set of the nearest closed release
---

# ADR-0122 — Select TCP/TLS alone for the nearest closed release

The Product Owner explicitly selects TCP/TLS as the sole Carrier of the nearest
closed release and defers QUIC. R-172 and the dependency register retain an
unresolved QUIC server finding; a supported correction has not been established.
Continuing a private QUIC implementation is not an assumed maintenance capacity
of the one Product Owner and Codex team. Keeping both Carriers would defer this
release; accepting the finding or reducing protection is rejected.

This decision supersedes the dual-Carrier selection and qualification clauses
of ADR-0048 and ADR-0081 for this release, including consumers of the retained
runtime. It preserves their other requirements and ADR-0077's common protection
baseline. The closed Route continues to use `ardents-carrier-tcp-tls-v2` and
`ardents-route-v3`; retained generation-2 TCP identities keep their exact meaning.
There is no new wire identity, security mode or transport negotiation.

## Required behavior and compatibility

- No maintained product command may start a QUIC listener or outgoing QUIC
  operation. Remove unused mechanisms or prove complete pre-effect exclusion at
  every real consumer, including retained Carrier factories. No hidden enabling
  option or production-reachable alternative factory belongs to this release.
- Historical signed QUIC profile identifiers retain their canonical bytes and
  meaning. Recognition or successful authentication is not permission to execute
  them. Selection, local duty activation and transport opening refuse deferred
  QUIC before socket/output, token presentation/spend or an accepting role effect.
  Never rewrite a signed record, treat QUIC as TCP, or fall back after refusal.
- Genuine TCP authority, admission, Hosting budgets, publication and Connection
  authentication, all Route roles, original deadlines, durable floors and joined
  termination remain required. Existing roots are not reset or adopted.
- Current positive release qualification covers TCP/TLS. QUIC receives explicit
  negative controls. Historical both-Carrier results remain evidence of their
  original artifacts, not qualification of the changed release. Mandatory gates
  remain enabled; affected checked native profiles must actually execute.

The dependency finding remains unresolved for deferred QUIC use. Verified absence
from the release's actual execution closure can remove its applicability to this
release; this is not a supported fix or general claim that quic-go is safe. Any
remaining runtime, test or build dependency needs its own normal acceptance.

## Delivery and return of QUIC

The product, technical, dependency, testing, qualification and domain/package
owners must agree with this selection. Source and canonical artifact closure,
causal no-effect refusal controls, actual TCP consumers and applicable native,
race, persistence and full repository gates prove adoption. Selection alone is
neither implementation nor Route acceptance. GitHub owns execution status.

R-172 and its experiments are deferred with their original limits and available
evidence preserved. No upstream submission, private fork, automatic experiment
or replacement dependency is selected. Returning QUIC needs a future explicit
Product Owner selection, a supported dependency disposition, exact compatibility
and accepted qualification of all affected uses. Installed Node, whole-host,
privacy, Source/provisioning and old-runtime cutover remain separate boundaries.
