---
status: accepted
date: 2026-09-26
---

# ADR-0092 — Retire the generic Publisher administration and Endpoint Transit acquisition chain

## Context

The cite-or-die sweep (ADR-0091, Product Owner direction of 2026-09-26)
requires every production file to cite a C0 behavior trace, an accepted ADR
retention obligation, or dev-support with a real caller. Findings F-37, F-30,
and F-35 established that the generic Publisher administration chain has no
production root: `OpenServiceAdministration`, `StartPublisher`,
`OpenPublisherIntroduction`, and `configurePublisher` have no non-test caller,
and the only maintained headless lane is the protected text runtime, whose
plan validation refuses `transit_acquisition_root` before any composition
effect. Earlier retirements explicitly deferred this card: #157 retained the
shared owners "until their own retirement cards", and the #205 Transit issuer
retirement preserved the Endpoint-side client and acquisition only within its
own change boundary. The Node-side issuer engine they served is already gone,
so the retained client has no live server.

## Decision

1. Retire the generic Endpoint Publisher/Connection chain:
   `service_administration.go`, `publisher_start.go`,
   `publisher_introduction.go`, `publisher_attachment_acquisition.go`,
   `service_connection.go`, `service_stream.go`, `service_publication.go`,
   `service_name_origin.go`, their exclusive symbols inside
   `service_runtime.go` and `service_admission.go`, and the generic-chain
   test harness with its recovery/stream-semantics/binding tests.
2. Retire `internal/endpoint/transit` (the durable Transit Grant acquisition
   journals), `transit_credential_acquisition.go`, and
   `transit_client_certificate.go` with their package-map row, deterministic
   package entry, and exact deadcode allowances.
3. Update the architecture guards: the Transit-issuer retirement oracle drops
   the acquisition retention entry and the User Route retirement oracle drops
   the transit stale-completion entry, both alongside their retired subjects.
4. Retain: the protected text participant and its
   `PublishSnapshot`/`textAdministration` path, Broker `Admit`, the
   publication-root ownership, the shared Service TLS handshake/exporter
   implementation in `service_tls.go` consumed by `protected_service_tls.go`,
   the native Service Connection owner, `maximumStreamBytes` and the
   capability constants, and `durableroot` for the token-attempt journal. The
   `routeRecovery`/`destinationBinding` aliases and the legacy
   `connectionContext` builder die with the chain. ADR-0062 remains accepted
   as the historical scoping decision, and the Transit Grant v1 wire bytes and
   signature domain are unchanged.
5. `transit-grant-acquisition.md` becomes a historical contract record; the
   `internal/endpoint` and `internal/endpoint/durableroot` package-map rows
   drop their transit claims.
6. Retire the exerciser-loss consequences in Route and credential:
   `route.DecodeTransitGrant`, `route.WriteCredentialRelaySetup`, and the
   OHTTP client closure `credential/client.go`, `credential/message.go`, and
   `credential/profile.go` with their now-unreferenced contract vocabulary
   (`Request`, `Result`, `Outcome`, `ClientConfig`, `Exchange`, `Profile`).
   The Transit-issuer retirement guard flips from retaining `OpenClient` and
   `DecodeProfile` to asserting their absence, and the User Route guard adds
   `WriteCredentialRelaySetup` to its forbidden declarations.
7. Narrow platform build ownership: the Endpoint service-runtime files
   (`service_runtime.go`, `service_admission.go`, `service_credential.go`,
   `service_resources.go`, `service_tls.go`, `application_half_close.go`,
   `target_link.go`) become Linux-only together with the protected text
   runtime they serve, matching the qualification lane that already refuses
   on other platforms. The remaining Route v2 wire leaves
   (`VerifyTransitGrant` with its body validation, the endpoint transit
   binding and sealed Introduction codecs, and the shared framing closure)
   lose their last Linux production consumer and move from the
   Windows-only deadcode allowance into the common allowance; the bounded
   Route v2 closure audit disposes of them.

## Consequences

The Endpoint package loses its second, generic Publisher/Connection
composition, leaving one accepting path per boundary (the F-36 target), and
compiles its service runtime only on Linux. The `credential` package keeps
only the closed-admission permission and blind issuer flows. No
persisted-data reader is removed: the text-path publication root, the token
journal, and the reachability store decoders are untouched. The Route-side v2
wire leaves are unreachable from any maintained composition and sit under the
common deadcode allowance until their bounded closure audit; historical grant
evidence remains readable under ADR-0062 wherever its decoders are retained
by that allowance.
