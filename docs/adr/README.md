# Architecture decision records

ADRs record accepted, consequential, hard-to-reverse decisions and their reason.
They do not record open questions, research notes, implementation progress, or
every selected library.

An accepted ADR may constrain a later Delivery Horizon without entering the
current backlog. [ADR-0008](0008-stage-research-before-public-network.md) and
[product scope](../product/scope.md) control promotion order.

Current decisions:

- [0001 — Public carrier with application-controlled services](0001-public-carrier-private-services.md)
- [0002 — Restart main as a greenfield research workspace](0002-greenfield-main.md)
- [0003 — Delegate bounded credentials to online Service Instances](0003-bounded-service-instance-credentials.md)
- [0004 — Authenticate shared epochs and separate Control Plane roots](0004-authenticated-epochs-and-separated-control-roots.md)
- [0005 — Separate hidden Route legs by domain and bound Entry exposure](0005-route-domains-and-bounded-entry-exposure.md)
- [0006 — Separate release safety from protocol transition](0006-separate-release-safety-from-protocol-transition.md)
- [0007 — Separate carrier privacy from Application networking](0007-separate-carrier-privacy-from-application-egress.md)
- [0008 — Stage route research before public-network implementation](0008-stage-research-before-public-network.md)
- [0009 — Adopt Go as the maintained project foundation](0009-go-project-foundation.md)
- [0010 — Keep a modular first-party monorepository](0010-modular-monorepository.md)
- [0011 — Separate unit, end-to-end, and live tests](0011-separate-unit-e2e-and-live-tests.md)
- [0014 — Authenticate and hide Stage 6 private naming exchanges](0014-private-naming-ohttp.md)
- [0015 — Separate release decision from versioned local activation](0015-separate-release-decision-from-local-activation.md)
- [0017 — Order root-name claims through authenticated epoch input](0017-authenticated-name-claim-ordering.md)
- [0018 — Authorize recovery with bounded individual signatures](0018-threshold-recovery-multisignatures.md)
- [0019 — Bound naming admission with scoped anonymous work](0019-bounded-anonymous-name-admission.md)
- [0020 — Authenticate the current Namespace in each Network Epoch](0020-authenticate-current-namespace-materialization.md)
- [0021 — Use password-derived Authority Custody](0021-use-password-derived-authority-custody.md)
- [0022 — Bind Target validity in the signed Name Record](0022-bind-name-record-validity.md)
- [0023 — Persist signed Namespace successors before materialization](0023-pending-signed-namespace-successors.md)
- [0024 — Select the native Interactive Route foundation](0024-native-interactive-route-foundation.md)
- [0025 — Use State-referenced Entry Invites](0025-state-referenced-entry-invites.md)
- [0026 — Use the closed Interactive Route v1 wire](0026-interactive-route-v1-wire.md)
- [0027 — Bind each Entry Invite to a fresh TLS attempt key](0027-entry-binding-v1.md)
- [0028 — Use the native Service Connection v1 grammar](0028-native-service-connection-v1.md)
- [0031 — Retire the generic live-test tree](0031-retire-generic-live-test-tree.md)
- [0032 — Use the canonical Target Link v1 grammar](0032-target-link-v1.md)
- [0033 — Use the closed Route RelaySetup v1 exchange](0033-route-relay-setup-v1.md)
Completed retirement decisions:

- [0029 — Retire Update V0 custody evidence by owned root migration](0029-retire-update-v0-custody-evidence.md)
- [0030 — Retire Update V0 as an unobserved test format](0030-retire-update-v0-as-test-format.md)

Superseded or withdrawn decisions retained for provenance:

- [0012 — Select standalone WebTunnel for the H3 Camouflage Adapter](0012-select-webtunnel-for-h3-camouflage.md)
- [0013 — Withdraw the initial Stage 6 cryptographic suite](0013-stage-6-cryptographic-suite.md)
- [0016 — Bind and isolate launcher-born Application Principals](0016-bind-and-isolate-launcher-born-application-principals.md)

ADR-0015 was accepted for the stopped Stage 7 work but remains the current
release/update ownership decision. New ADRs use the next unreserved four-digit
number and should remain short. When a decision is superseded, retain the
original record and link the replacement.
