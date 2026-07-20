---
name: ardents-dependency-safety
description: Dependency and security review workflow for Ardents. Use when selecting, adding, replacing, or upgrading dependencies that affect network, storage, crypto, observability, wire protocols, or other critical product foundations.
---

# Ardents Dependency Safety

Use this skill whenever a dependency can influence a critical product plane.

## Read First

- `docs/engineering-constraints.md`
- `docs/system-properties.md`
- `docs/canonical-network-foundation.md` if the dependency touches network/messaging
- the domain document affected by the dependency

## Decision Workflow

1. State which domain or substrate the dependency serves.
2. State whether the dependency is direct or only transitive.
3. Check whether the dependency solves a problem the documents actually require.
4. Check whether it replaces a forbidden self-built substrate.
5. Check whether it conflicts with canonical network foundation or other fixed decisions.
6. Review security posture and known vulnerabilities.
7. Accept only if the dependency is both product-fit and security-acceptable.

## Mandatory Checks

- active maintenance
- release process
- acceptable license
- no unresolved critical or high-risk vulnerabilities for the intended use
- realistic production fit for the target plane
- no major mismatch between dependency scope and product need

## Reject By Default If

- the dependency has unresolved critical vulnerabilities;
- the dependency is abandoned;
- the dependency forces a product direction that contradicts the system documents;
- the dependency solves a different problem than the one the domain actually has;
- a mature dependency is being bypassed in favor of a new self-built substrate.

## Output

When using this skill, produce:

- affected domain;
- dependency role;
- security posture;
- accept/reject recommendation;
- required mitigation or upgrade if risk exists.
