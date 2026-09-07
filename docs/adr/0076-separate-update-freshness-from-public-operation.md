---
status: proposed
date: 2026-09-06
extends: ADR-0074
---

# Separate update freshness from public operation

## Context

[ADR-0074](0074-target-non-administrative-public-operation.md) excludes an
indispensable publisher heartbeat from autonomous public operation.
[R-149](../research/records/r-149-autonomy-transition.md#solution-proposals-by-outcome)
compares how to preserve software provenance without retaining that power.
The current [Release/Update owner](../technical/release-update-custody.md)
separates artifact verification from activation but still implements the
binding C0 safety policy. Expired metadata and an invalid operating context
must not be conflated when designing its public successor.

## Proposed decision

A selected public successor treats update authorization and installed-program
operation as separate decisions:

- Preserve authenticated initial provenance, exact artifact verification,
  bounded metadata processing, trust continuity and non-decreasing floors.
  An update needs current sufficient authorization and explicit owner-approved
  activation. Unavailable or expired update evidence cannot authorize new bytes.
- Record the exact authorized installed artifact and adopted network/rules.
  Publisher disappearance alone does not revoke this prior authorization.
  Network work still requires valid current State, compatible adopted rules,
  owner-scoped credentials, resources and finite Work Safety limits.
- Treat an authenticated vulnerability advisory as input to a previously
  selected local owner policy. That policy may warn or suspend the owner's
  operation; the advisory signer has no universal ban/install power.
  The proposed base policy reports missing update evidence without stopping
  otherwise admissible network work solely for that reason.
- Reject objectively invalid protocol behavior under the locally adopted
  profile. A publisher's build blacklist or a producer vote is not proof of
  protocol invalidity and cannot silently amend that profile.
- Keep a change of software trust channel or incompatible rules explicit.
  Exact trust-root/floor migration must be selected before implementation;
  neither missing metadata nor an operator convenience path clears old floors.

This proposal changes no current C0 behavior or wire identity. It selects no
new update format, channel operator, advisory protocol, automatic installer,
cryptographic implementation or supported host profile.

## Outcomes to verify

| Situation, with a correctly enforcing endpoint | Required public-successor result |
|---|---|
| Authorized installed artifact, valid operating context, update feed lost | Work remains governed by its other finite requirements and the owner's selected policy; update unavailability is visible. |
| Fresh update metadata, expired or invalid State/credential | No network-work permission is inferred from the update metadata. |
| Replay of expired metadata to install another artifact | Update refused; installed provenance and floors are not reset. |
| Signed advisory or recommendation to execute a build | Apply only the owner's chosen local policy; no remote forced activation. |
| Interrupted installation or change of trust channel | Use the separately accepted activation/migration contract; no mixed roots or implicit rollback. |

## Alternatives and consequences

A mandatory global software-safety renewal gives publishers a quick common stop
mechanism but also makes them indispensable. Ignoring update authenticity or
freshness would allow unsafe activation. The proposal preserves update checks
while accepting that installed software can become vulnerable without a
universal automatic stop or cure. An authenticated build is not proof of safe
code. A compromised local execution boundary may ignore this policy entirely.

The [TUF client workflow](https://theupdateframework.github.io/specification/latest/#detailed-client-workflow)
provides evidence for retained update verification and expiration checks; it
does not choose this runtime policy. Promotion requires an accepted detailed
successor, finite work/freshness composition, migration rules and actual outcome
evidence under R-149. Current Release Safety remains binding until then.
