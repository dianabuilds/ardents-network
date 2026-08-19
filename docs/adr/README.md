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
- [0012 — Select standalone WebTunnel for the H3 Camouflage Adapter](0012-select-webtunnel-for-h3-camouflage.md)

Withdrawn decisions retained for history:

- [0013 — Withdraw the initial Stage 6 cryptographic suite](0013-stage-6-cryptographic-suite.md)

New ADRs use the next four-digit number and should remain short. When a decision
is superseded, retain the original record and link the replacement.
