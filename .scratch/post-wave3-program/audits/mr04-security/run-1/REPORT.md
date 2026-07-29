# MR-04 Security Audit

## Executive summary

No exploitable vulnerability remains in the final MR-04 snapshot through
`fa942e9`. The audit initially confirmed a HIGH authorization and evidence-
provenance defect in the pre-remediation Authority path: an Operator holding
the broader membership action could submit self-asserted receipt digests and
could use fencing evidence where a survivor active receipt was required. The
implementation was corrected during this task and the original path now fails
closed.

## Baseline and scope

The closest operational baseline is a Pacemaker/STONITH-style fencing
coordinator with stricter Ardents-specific Authority checkpoint and survivor
receipt requirements. This run covers the MR-04 range
`5ba408f...fa942e9`, including the crash-resumable deployment coordinator,
strict journal, Authority integration boundary, and remediation. It is not a
repository-wide audit and it does not claim production host qualification.

## Findings

No confirmed finding survives in the final snapshot.

## Confirmed defect remediated during the audit

The existing `SubmitDeploymentFenceEvidence` RPC formerly reused
`realm.channel.membership.change` on a realm-channel resource, accepted
syntactically valid caller-supplied receipt digests without an authenticity
verifier, allowed evidence for survivor delivery recipients, and treated such
evidence as a substitute for survivor active receipts.

The final implementation:

- requires exact `topology.node.fence` authorization on
  `node:<target Principal>`;
- canonicalizes that Principal before authorization and repeats the exact
  action/resource check in the Authority domain;
- requires a configured `DeploymentFenceVerifier`; the current R1 production
  daemon has none and therefore rejects fresh evidence as unavailable;
- persists versioned verification provenance inside the signed
  ledger/checkpoint, while legacy unversioned evidence fails ledger
  validation;
- binds the exact canonical evidence digest into the hashed audit record and
  signed checkpoint head;
- requires a one-to-one reload-time cross-binding between each retained fence
  evidence item and its deterministic signed audit record, preventing store
  substitution, duplicate binding, or legacy provenance promotion without the
  Authority signer;
- permits fencing only for the removed membership target;
- requires every survivor delivery to retain its own active receipt.

Regression tests cover the old action, absent and denying verifiers, legacy
unverified evidence, survivor-target evidence, durable verification marking,
restart replay, and both checkpoint crash boundaries.

## Positive patterns

- Strict manifest, request, target, clock, receipt, checkpoint, repository and
  survivor bindings are revalidated across every durable phase.
- Ambiguous adapter calls have explicit immutable idempotency contracts.
- Journal state is bounded, private, atomically replaced, transition-checked,
  and redacted.
- The Authority evidence digest is canonical and excludes only the
  Authority-owned verification marker, preserving the deployment/Authority
  contract.
- Exact replay of already checkpointed verified evidence remains idempotent;
  conflicting replay fails closed.
- No production host adapter, remote shell, network mutation, deployment,
  qualification or capability promotion was introduced.

## Verification

- `go test ./... -count=1`
- `go test -race ./internal/deployment ./internal/deploymentjournal ./internal/authority -count=1`
- `go vet ./...`
- `govulncheck ./...` — no called vulnerabilities
- API generation check
- capability catalogue — 24 capabilities, 0 qualified
- audit traceability and import guard
- independent Spec, Standards and security regression re-reviews

## Coverage limits and deferred hardening

This R1 audit uses deterministic adapters and local protected files. It does
not validate Linux service-manager, firewall/router, DNS/static-peer, Waku
peer-deny, WORM administration, or three-host behavior; those remain R3/MR-08.
R3 must provide verifier and adapter conformance tests, including ambiguous
completion, and add OS-level single-coordinator exclusion around the file
journal. These are explicit unimplemented boundaries, not support claims.
