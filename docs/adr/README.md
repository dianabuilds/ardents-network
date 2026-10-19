# Current architecture decisions

These decisions retain a current contract or product constraint. Read them when
the current owner names the decision; this index is not an implementation plan.
Later explicit amendments take precedence over earlier byte/ownership editions.
ADR-0011 retains its testing separation while ADR-0031 removes the generic live
tree. ADR-0071 retains the separate challenge-bound time witness; it does not
restore the removed Entry/enrollment artifact. ADR-0054 retains independent
transition authorities, without restoring Namespace or concluded ceremonies.

ADR-0117 selects one closed Service v3 edition. It replaces earlier byte
identities and state handling while preserving security/lifecycle invariants,
including Terminal receipts. The [Service owner](../technical/endpoint-service-runtime.md)
and [protocol](../technical/protected-route-protocol.md) state the current rules.

## Surviving decisions

- [Public carrier with application-controlled services](0001-public-carrier-private-services.md)
- [Delegate bounded credentials to online Service Instances](0003-bounded-service-instance-credentials.md)
- [Authenticate shared epochs and separate Control Plane roots](0004-authenticated-epochs-and-separated-control-roots.md)
- [Separate hidden Route legs by domain and bound Entry exposure](0005-route-domains-and-bounded-entry-exposure.md)
- [Separate release safety from protocol transition](0006-separate-release-safety-from-protocol-transition.md)
- [Separate carrier privacy from Application networking](0007-separate-carrier-privacy-from-application-egress.md)
- [Stage route research before public-network implementation](0008-stage-research-before-public-network.md)
- [Adopt Go as the maintained project foundation](0009-go-project-foundation.md)
- [Keep a modular first-party monorepository](0010-modular-monorepository.md)
- [Separate unit, end-to-end, and live tests](0011-separate-unit-e2e-and-live-tests.md)
- [ADR-0015 — Separate release decision from versioned local activation](0015-separate-release-decision-from-local-activation.md)
- [ADR-0021 — Use password-derived Authority Custody](0021-use-password-derived-authority-custody.md)
- [ADR-0030 — Retire Update V0 as an unobserved test format](0030-retire-update-v0-as-test-format.md)
- [ADR-0031 — Retire the generic live-test tree](0031-retire-generic-live-test-tree.md)
- [ADR-0038 — Keep alpha disclosure separate from acceptance authority](0038-alpha-control-disclosure-reader-v1.md)
- [ADR-0043 — Derive Grace from signed Name deadlines](0043-derive-grace-from-signed-deadlines.md)
- [ADR-0048 — Maintain TCP/TLS and QUIC v1 behind one Carrier contract](0048-maintain-tcp-and-quic-carriers.md)
- [ADR-0054 — Separate Functional Alpha transition contracts](0054-separate-alpha-transition-contracts.md)
- [ADR-0064 — Separate Service Authority custody from host Instance enrollment](0064-separate-service-authority-custody-from-instance-enrollment.md)
- [ADR-0065 — Commit headless publication after live Introduction readiness](0065-commit-publication-after-live-introduction-readiness.md)
- [ADR-0067 — Retire completed local alpha ceremonies](0067-retire-completed-local-alpha-ceremonies.md)
- [ADR-0069 — Retire the active Browser implementation and qualification lanes](0069-retire-active-browser-implementation.md)
- [ADR-0070 — Route owns volatile User-route orchestration](0070-own-volatile-user-route-orchestration.md)
- [ADR-0071 — Recipient-bound offline Headless enrollment](0071-recipient-bound-offline-headless-enrollment.md)
- [Target public operation without appointed network administrators](0074-target-non-administrative-public-operation.md)
- [Evolve one common protection baseline](0077-evolve-one-common-protection-baseline.md)
- [Select the common split-circuit privacy architecture](0078-select-common-split-circuit-privacy.md)
- [Select the closed protected text-Service contract](0081-select-closed-protected-service-contract.md)
- [Bind issuer-bootstrap restriction to each authenticated Node child](0082-bind-bootstrap-restriction-to-node-child.md)
- [Activate one data lane after an authenticated Rendezvous pair](0083-activate-joined-rendezvous-data-lane.md)
- [Bound forwarding replenishment to the admitted parent channel](0085-bound-forwarding-replenishment.md)
- [ADR-0103 — Join the closed-profile Role Domain to the authenticated Epoch assignment](0103-join-closed-profile-role-domain-to-epoch-assignment.md)
- [ADR-0105 — Retire the Namespace subsystem; absence of any reader is the incompatibility](0105-retire-namespace-subsystem.md)
- [ADR-0106 — Retire the Entry Invite subsystem; the Invite-root writer stops at a before-effect refusal](0106-retire-entry-invite-subsystem.md)
- [ADR-0107 — The duty root schema becomes version 2; legacy generations convert in place and the Transit Grant spend ledger retires](0107-duty-root-version-2-spend-ledger-retirement.md)
- [ADR-0108 — The Portable Endpoint profile contracts to its consumed roots; the grants, Vault, diagnostics, and cache scaffold is no longer created](0108-contract-portable-profile-roots.md)
- [ADR-0110 — Keep ACA1 as the sole maintained control inspection format](0110-retire-aca2-corpus-inspection.md)
- [ADR-0111 — AREP v3 becomes the sole closed-Epoch intake schema; retired envelopes are refused with a typed error before any durable effect](0111-pin-arep-v3-sole-closed-epoch-intake.md)
- [ADR-0112 — Network enrollment v3 is the sole accepted descriptor grammar at every maintained command boundary; recognized v1/v2 descriptors are refused with a typed error](0112-pin-network-enrollment-v3-sole-descriptor.md)
- [ADR-0113 — The retained Alpha corpus/link compatibility surface and the local Name wire encoder are retired; the whole `name` command family refuses before effects](0113-retire-retained-alpha-compatibility-surface.md)
- [ADR-0114 — The dedicated-host Contributor retirement mechanism is removed entirely; the ADR-0089 owned-shutdown obligation closes for absence of object](0114-remove-dedicated-host-contributor-retirement.md)
- [ADR-0115 — Retire the uncomposed interactive State resolution projection](0115-retire-interactive-state-resolution-projection.md)
- [ADR-0116 — The dedicated-host resource profile and the Node clock-observation trigger are removed as production-unreachable](0116-remove-dedicated-host-resource-profile-clock-observation.md)
- [ADR-0117 — Reset closed Service formats to one v3 edition](0117-reset-closed-service-formats-to-v3.md)
- [ADR-0118 — Retain Direct Source guards through dependent work](0118-retain-direct-source-guards-through-dependent-work.md)
- [ADR-0119 — Authenticate the protected Endpoint generation in the same Release set](0119-bind-protected-endpoint-generation-to-release.md)
- [ADR-0120: Authorize an operator-prepared qualification release](0120-authorize-operator-prepared-qualification-release.md)

- [ADR-0121: Bound installed Connection setup to thirty seconds](0121-bound-installed-connection-setup.md)

- [ADR-0122: Select TCP/TLS alone for the nearest closed release](0122-select-tcp-only-closed-release.md)

## Former editions and rationale

[Archived decisions](../archive/README.md#architecture-decisions) retain
needed historical decision evidence unchanged, with its current owner. The
complete earlier archive and manifest remain available at the pinned Git
snapshot, including superseded, withdrawn, proposed and retired-subsystem
editions. Their recorded dispositions do not repeal surviving accepted
constraints.
Create a new ADR only for a consequential, hard-to-reverse accepted trade-off.
