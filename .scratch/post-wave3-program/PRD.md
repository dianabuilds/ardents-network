# Post-Wave 3 Implementation and Release Program

Status: active

## Outcome

Turn the accepted Wave 3 research recommendations into an explicitly gated,
dependency-ordered implementation and qualification program without treating a
Proposed ADR as approved, mixing implementation with release evidence, or
promoting any capability before complete matching-commit DR-06 evidence exists.

The program has three independently managed streams:

1. stabilization qualification for the fifteen existing capability claims;
2. Authority, Multi-host Reachability, and Application Messaging;
3. Application Discovery, Application Hosting, and the discovery-only service
   handoff.

## Admission baseline

Admission was checked on 2026-07-25 at
`main@5c86a1e9fc9033fcb51b53b4b9861e938a3b48c6`.

- The worktree was clean and local `main` was six commits ahead of
  `origin/main@cbec069c37df9cf57756970a2c3a0eef8c232778`.
- All seven Wave 3 research issues were closed.
- ADR-0011 through ADR-0015 were `Proposed`.
- The canonical catalogue contained 24 capabilities in 8 domains with
  0 qualified.
- `go run ./tests/tooling/capabilitycatalog -check` passed.
- `go test ./tests/tooling/... -count=1` passed with an external Go build
  cache.

This admission result permits governance preparation, independent AD-01 work,
and DR-06 stabilization preparation. It does not approve an ADR, start a
dependent feature implementation, or qualify a capability.

## Canonical naming policy

The detailed research packets are canonical when their slice names differ from
the abbreviated Wave 3 synthesis:

- `application-discovery.md` defines AD-01 through AD-05;
- `channel-grant-authority.md` defines CGA-01 through CGA-07;
- `multi-host-reachability.md` defines MR-01 through MR-08;
- `application-messaging.md` defines AM-01 through AM-06;
- `application-hosting.md` defines AH-01 through AH-05;
- `direct-service-interaction.md` defines DSI-01 through DSI-04.

Shared plans and tracker text must be synchronized to those names before a
slice is assigned. A contract freeze belongs in the first executable vertical
slice rather than becoming a duplicate horizontal issue.

## Program dependency graph

```text
DR-06 stabilization preparation ------------------------------------> DR-06

ADR-0011 review
  -> maintainer acceptance
  -> CGA-01..06
       +-> ADR-0013 review -> MR-01..08
       +-> ADR-0015 review -> AM-01..05
  -> Authority + Multi-host + Messaging qualification

AD-01 -> AD-02 -> AD-03 -> AD-04 -> AD-05
  |
  +-- + ADR-0012 acceptance -> AH-01 -> AH-02 -> AH-03 -> AH-04

AD-04 + ADR-0012 acceptance
  -> ADR-0014 acceptance -> DSI-01 -> DSI-02 -> DSI-03

AD-05 + AH-05 + DSI-04 + real topology evidence
  -> Hosting and discovery-handoff qualification
```

ADR-0013 and ADR-0015 may be reviewed in parallel only after ADR-0011 is
accepted. MR-01/MR-02 and AM-01 may then overlap later CGA slices where their
explicit contracts permit it.

The apparent CGA-04/MR-04 cycle is split at a versioned
`DeploymentFenceEvidence` boundary. Authority owns membership acceptance and
terminal authority state. Deployment owns isolation evidence. Their production
claim is accepted only by a later joint E2E gate.

## First-wave tracker

The first wave intentionally contains only seven coarse, independently owned
issues:

1. DR-06 stabilization candidate and execution matrix.
2. ADR-0011 Authority review packet.
3. ADR-0012 Hosting review packet.
4. AD-01 protected Application admission seam.
5. CGA-01 Realm Authority genesis and redacted inspection.
6. AH-01 durable local Hosted Service tracer.
7. Dependent ADR-0013, ADR-0014, and ADR-0015 review gates.

Later CGA, MR, AM, AD, AH, and DSI files are published only when their direct
predecessor is accepted. Qualification slices remain separate from
implementation slices and commits.

## Progress checkpoint — 2026-07-26

- AD-01 was accepted after commit-bound APP-001 evidence and independent
  specification/standards reviews.
- AD-02 `Resolve a Trusted Service End to End` is closed after implementation,
  remediation, commit-bound APP-001 evidence, and clean repeat reviews.
- AD-03 `Close Projection, Privacy, and Abuse Boundaries` is published as
  `issues/09-application-discovery-ad03.md` and becomes the next serial
  Application implementation slice;
- DR-06 remains independently `ready-for-agent`;
- ADR-0011 is accepted and CGA-01 is ready for implementation triage; Hosting
  implementation remains gated on explicit maintainer disposition of ADR-0012.

## Governance checkpoint — 2026-07-28

- ADR-0013 was explicitly accepted after a compatibility review against the
  accepted Authority design and CGA-01 through CGA-06 implementation found no
  blockers.
- W3-D004 is accepted as a design decision only. MR-01 is not published or
  admitted, CGA-07 remains `needs-info`, and `deployment.multi-host` and
  `realm.channel-grant-authority` remain `Q=no`.
- ADR-0015 remains a separate ready review. ADR-0014 remains gated by
  ADR-0012; neither was bundled into the ADR-0013 disposition.

## Combined review and admission checkpoint — 2026-07-28

- ADR-0015 was returned with explicit restore-freshness, rebind/close cutover
  and Content Reference terminology blockers. It remains Proposed and AM-01 is
  not admitted.
- MR-01 passed its post-ADR-0013 admission audit and is published as PW3-18.
  This admits only bounded manifest compilation/validation, not host access,
  topology mutation, implementation assignment or qualification.
- CGA-07 was re-audited and remains `needs-info` because its dedicated DR-06
  scope, release candidate, real-host/WORM/backup/release environments and
  evidence destination are still undeclared.

## Stream ownership

- The integrator alone edits this PRD, the Wave 3 decision register, the
  canonical capability catalogue, the evidence register, and aggregate
  qualification snapshots.
- A slice agent owns only its assigned tracker and implementation files.
- Shared Application protocol composition and `sdk/go/client` integration are
  serialized through one integration owner.
- Qualification agents write to distinct evidence paths and never reuse an
  implementation commit as an unreviewed qualification snapshot.

## Stabilization boundary

Independent DR-06 stabilization covers exactly:

1. `node.lifecycle`
2. `operator.command-interface`
3. `identity.principal-access`
4. `application.installation-content`
5. `network.waku-foundation`
6. `discovery.operator-resolution`
7. `content.operator-lifecycle`
8. `transfer.replication`
9. `workload.lifecycle`
10. `hosting.operator-publication`
11. `operations.diagnostics`
12. `operations.configuration-reload`
13. `operations.backup-upgrade-rollback`
14. `operations.native-installation`
15. `release.artifacts-provenance`

Wave 3 capabilities are excluded from that candidate. Their later
qualification requires a separately declared DR-06 scope.

## Service handoff decision

ADR-0014 selects a discovery-only boundary. Ardents may authenticate and
authorize Resolve, project a bounded privacy-safe `Discovery.Target`, and then
return control to the Application. It does not dial, proxy, forward
credentials, translate Access Grants into service authorization, or promise
service authentication.

Until ADR-0014 is explicitly accepted, `service.direct-interaction` remains
unchanged and `Q=no`. After acceptance, governance must replace its misleading
interaction claim with a discovery-handoff claim, or keep it unqualified and
qualify only `application.discovery`. No implementation issue may silently
claim that an Ardents Direct Service adapter exists.

## Qualification policy

- Every evidence program is bound to one exact clean commit and release
  version.
- Commands, start/end times, environment, toolchain and base identities,
  results, artifact hashes, and run attempts are retained.
- A retry never replaces the first result. The flake is classified and the
  complete affected gate is rerun on the same clean commit.
- Capability promotion is computed from that capability's complete required
  gate intersection; there is no blanket `Q=yes`.
- ADR acceptance, implementation, and qualification use separate logical
  commits.
- Missing real-host, Docker, native-install, WAN, PKI, security, or release
  environments block the corresponding claim rather than being replaced by a
  weaker local test.

## Program completion

The program is complete only when:

- every selected ADR has explicit maintainer disposition;
- accepted vertical slices preserve exact authority, state ownership, finite
  bounds, restart, restore, migration, and downgrade behavior;
- the independent fifteen-capability stabilization candidate has a truthful
  matching-commit result;
- feature and hosting/handoff qualifications use their declared real topology
  and retained evidence;
- the capability catalogue, evidence register, README, Changelog, and
  remediation ledger agree with the accepted evidence;
- unsupported behavior remains explicit;
- the final worktree is clean and no push occurs without a separate user
  request.
