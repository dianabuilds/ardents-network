# ADR 0014: End Direct Service Interaction at Authenticated Discovery

- Status: Proposed
- Date: 2026-07-25
- Decision owners: Application Interface, Discovery, Security
- Research: `docs/engineering/research/direct-service-interaction.md`

## Context

Ardents can publish Node-signed service locator facts and the accepted
Application Discovery design exposes a bounded, privacy-filtered `Resolve`
operation. Accepted Application Hosting adds a leased owner-qualified service
lifecycle but deliberately hides workload and endpoint orchestration.

A direct client adapter would need to own target selection, dialing, TLS
identity, Principal authentication to the service, authorization translation,
credentials, limits, redirects, retry, partial results, protocol errors, and
connection recovery. Arbitrary HTTP/TCP services do not share those semantics.
The frozen product has no service credential issuer, no way for a workload to
verify an Ardents Access Grant, and no safe Application Hosting certificate or
private-key delivery lifecycle.

A Node Principal signature proves who asserted a discovery record. It does not
prove that the endpoint process possesses a TLS key, and a grant to resolve a
service type does not authorize an operation in that service.

## Decision

Ardents v1 direct-service responsibility ends after the authenticated and
authorized Application Discovery `Resolve` call returns a bounded eligible
target set. Ardents does not add a Direct Service wire service, SDK `Do` or
`Dial` adapter, gateway, tunnel, reverse proxy, sidecar, service credential, or
Access-Grant translation.

The Application owns target use through its standard HTTP/TCP stack. The
service's application protocol owns client authentication, service
authorization, methods, paths, schemas, payload and stream bounds,
acknowledgement, idempotency, redirects, retry, errors, and audit. An Ardents
Application Session, Credential, Access Grant, Delegation, root/device key, or
Channel Grant is never forwarded to or interpreted by the service.

The exact `application.discovery.resolve` authority permits disclosure of an
eligible locator only. It is not service-use authority. Revoking it blocks a
later Resolve but cannot terminate an external connection or revoke a
service-owned credential.

For an existing Operator-published HTTPS target, the URI host is the TLS
reference identity. Clients perform normal PKIX path and validity validation,
DNS-ID matching for DNS names, and exact IP-ID matching for literal addresses.
TLS failure never falls back to plaintext or disabled verification.
The accepted v1 Application locator admits only literal non-loopback hosts, so
its HTTPS case uses IP-ID; DNS endpoints remain Operator-only unless a later
decision explicitly widens the locator.

Discovery records carry no TLS trust root, certificate, SPKI hash, leaf
fingerprint, or alternate server name. Node Principals, discovery signing keys,
service IDs, and Waku Peer IDs are not TLS pins. Trust-on-first-use is rejected.
Certificate renewal under the same validated reference identity and trust path
does not require a discovery schema change. A hostname/IP change is published
as a new signed endpoint and the old endpoint is withdrawn or expires.

Application-owned Hosting remains limited to `http` and `tcp` in v1. Although
the existing ingress proxy can forward TLS bytes, Application Hosting does not
have an Operator-owned certificate reference, private-key delivery, renewal,
or same-identity readiness contract. A future HTTPS Hosting design requires a
separate R2 decision and must preserve ADR-0012 ownership, lease, and
withdrawal semantics.

Ardents does not define direct-service retry or error mapping. Resolve errors
remain Ardents errors. DNS, dial, TLS, and application errors remain native to
their owners. Applications must not automatically replay non-idempotent or
partially written operations; any retry requires application-protocol
idempotency and finite caller-owned deadlines/bounds.

## Consequences

- The Application-facing interface stays small and Discovery remains a deep
  module owning trust, eligibility, policy, privacy, ordering, and bounds.
- Ardents does not become a service mesh or a data-plane availability
  dependency.
- Applications must configure their own service credentials, trust roots,
  protocol limits, telemetry, and safe retry behavior.
- Operators diagnose locator and Hosting failures in Ardents, and DNS/TLS/
  protocol failures in the Application/service deployment.
- No new Ardents persistence, backup material, credential schema, certificate
  authority, token issuer, or mixed-generation direct-client state is added.
- Existing Operator-published HTTPS endpoints can be used only when the
  Application can validate their URI-host identity normally; Ardents makes no
  service-authentication claim.
- A future uniform authenticated service protocol is new R2 scope and cannot
  be introduced as an additive SDK convenience helper.
- Capability qualification remains a separate DR-06 decision.
