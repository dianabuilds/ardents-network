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
- [0034 — Bind a separate Service Introduction HPKE key in Credential v2](0034-service-introduction-hpke-credential-v2.md)
- [0035 — Use live Introduction slots and EndpointTransitBinding v1](0035-live-introduction-slots-and-transit-binding-v1.md)
- [0036 — Resolve Target Links through private, current descriptors](0036-target-private-reachability-v1.md)
- [0037 — Carry private reachability through a closed Initiator operation](0037-private-reachability-entry-carrier.md)
- [0038 — Keep alpha disclosure separate from acceptance authority](0038-alpha-control-disclosure-reader-v1.md)
- [0039 — Use State-authorized opaque Transit Grants for C-2 admission](0039-state-authorized-transit-grants-v1.md)
- [0040 — Keep named alpha outside the canonical Namespace](0040-bounded-alpha-name-overlay.md)
- [0041 — Add a separate signed corpus component through alpha-control v2](0041-alpha-control-corpus-component-v2.md)
- [0042 — Bind the accepting alpha-control command to enrollment v3](0042-bind-alpha-control-command-to-v3-enrollment.md)
- [0043 — Derive Grace from signed Name deadlines](0043-derive-grace-from-signed-deadlines.md)
- [0044 — Revalidate Browser Entry proxy authentication](0044-revalidate-browser-entry-proxy-authentication.md)
- [0045 — Deliver the alpha Browser Entry as a signed unlisted Firefox add-on](0045-firefox-first-unlisted-browser-entry-delivery.md)
- [0046 — Bind the Destination Resolution Gateway profile in Network State](0046-state-selected-destination-resolution-gateway.md)
- [0047 — Issue dynamic membership-level Transit Grants through State and Entry](0047-dynamic-membership-transit-grants.md)
- [0048 — Maintain TCP/TLS and QUIC v1 behind one Carrier contract](0048-maintain-tcp-and-quic-carriers.md)
- [0049 — Do not select a blocked-entry profile for the functional alpha](0049-defer-blocked-entry-profile.md)
- [0050 — Keep closed-alpha release seeds in separate local custody](0050-separate-local-release-seed-custody.md)
- [0051 — Confirm the local release-seed public receipt without exporting secrets](0051-confirm-local-release-seed-public-receipt.md)
- [0052 — Build only fixed closed-alpha static inputs from local custody](0052-build-fixed-alpha-static-inputs.md)
- [0053 — Bootstrap functional-alpha Network State with a separate 1-of-1 authority](0053-bootstrap-functional-alpha-network-state.md)
- [0054 — Separate Functional Alpha transition contracts](0054-separate-alpha-transition-contracts.md)
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
