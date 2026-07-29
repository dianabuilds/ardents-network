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

## ADR-0015 disposition — 2026-07-29

- ADR-0015 now defines an independent monotonic Messaging checkpoint that
  detects rollback even when the Authority head is unchanged.
- Delivery-Node rebind maps to the existing same-membership Authority
  generation rotation; close maps to a checkpointed Messaging tombstone,
  stopped renewal and bounded grant/drain expiry without inventing an
  Authority close action.
- Content References use their canonical globally content-addressed identity;
  owner qualification remains on Content Objects, Content Manifests and
  authorization bindings.
- The maintainer accepted the corrected ADR-0015 and W3-D002. This accepts the
  design only; `application.messaging` remains `Q=no` and AM implementation
  still requires separate admission.

## MR-01 implementation checkpoint — 2026-07-29

- PW3-18 implements the pure bounded `ardents.topology/v1` compiler through
  exact tip `c981cc5a6409f9827d470fa95fb16be01107dd80`.
- Full local, race, tooling, catalogue and diff checks pass; repeat independent
  Standards and Spec reviews report no actionable findings.
- PW3-18 is `ready-for-human`. No host, network, signer, Authority, repository,
  runtime or production state was touched; nothing was pushed or deployed.
  `deployment.multi-host` remains `Q=no`.
- The maintainer accepted exact MR-01 implementation tip
  `c981cc5a6409f9827d470fa95fb16be01107dd80` and closed PW3-18. MR-02 is
  admitted as PW3-19 for the bounded protected three-Node status slice.
  Admission starts no host mutation or qualification; `Q=no`.

## MR-02 implementation checkpoint — 2026-07-29

- PW3-19 implements the bounded protected three-Node status slice through
  exact implementation tip `97fed1b68d8b2a21cbf1ba44aae0b027d48ef4e3`.
- Full local, race, tooling, tagged-compilation and capability checks pass.
  Independent Spec, Standards and Security re-reviews report PASS; retained
  security findings are empty.
- The security review's intermediate session-capacity finding was corrected
  with one bounded `EndSession` lifecycle cleanup per Node, outside the three
  protected product-observation calls and inside the same deadlines.
- The verified tree includes Windows listener hardening through `9572d40`.
  Local test listeners bind loopback; Linux-only private-interface acceptance
  scenarios skip on Windows, avoiding recurring Windows Firewall prompts
  without weakening their Linux evidence.
- PW3-19 is `ready-for-human`. No real host, network, signer, Authority,
  production state, qualification, capability promotion, push or deployment
  occurred. `deployment.multi-host` remains `Q=no`.
- The maintainer accepted exact MR-02 implementation tip
  `97fed1b68d8b2a21cbf1ba44aae0b027d48ef4e3` and closed PW3-19. This
  acceptance does not admit MR-03 by itself and does not change `Q=no`.

## MR-03 admission checkpoint — 2026-07-29

- The maintainer accepted MR-02 and closed PW3-19 in governance commit
  `5979e9b`.
- PW3-20 admits the local-substitutable protected Authority recovery
  coordination slice. Accepted DR-03/CGA-06 remains the only owner of the
  Authority ledger, signed checkpoint and immutable repository truth.
- MR-03 may bind the reviewed topology to protected Authority context, enforce
  authenticated clock preflight, acknowledge an exact recovery-only
  sequence/digest and project Authority-last versus Authority-first order. It
  may not administer repository history or perform host lifecycle mutation.
- Real independent backup/WORM and three-host recovery evidence remains R3.
  Admission changes no capability qualification; `deployment.multi-host`
  remains `Q=no`.

## MR-03 implementation checkpoint — 2026-07-29

- PW3-20 is `ready-for-human` at exact implementation tip
  `e4b8fff08abb3e5ffe4a17a41a1fbc0499849017`; its logical implementation range
  is `1dd3c89..e4b8fff`.
- Strict manifest-owned Realm/context/reference binding, three authenticated
  clock observations, exact same-Realm Authority verification and pure
  Authority-last/Authority-first order projection are implemented.
- Final independent Spec and Standards reviews pass. Focused security audit
  run 2 retained no confirmed vulnerability.
- Full, tooling, architecture, capability, race, tagged compile, API generation,
  vet and vulnerability checks pass.
- No real host, WORM administration, deployment, production state,
  qualification, capability promotion or push occurred.
  `deployment.multi-host` remains `Q=no`.
- The maintainer accepted exact MR-03 implementation tip
  `e4b8fff08abb3e5ffe4a17a41a1fbc0499849017` and closed PW3-20. This
  acceptance admits MR-04 dependency review but does not itself admit host
  fencing, rejoin, qualification or any capability change.

## MR-04 admission checkpoint — 2026-07-29

- PW3-21 admits the R1 local-substitutable crash-resumable fencing coordinator
  after exact MR-03 acceptance and accepted CGA-04 dependency review.
- Deployment owns only the strict durable Fence Transaction, attributable
  isolation evidence, phase coordination, replay binding, and redacted
  outcome. Realm Authority remains the sole owner of membership removal,
  generation, signed checkpoint, immutable repository, and survivor active
  receipt truth.
- The current tree has protected Node stop and CGA-04 Authority seams, but no
  complete accepted production seam for deployment ingress, DNS/static usable
  sets, and Waku peer deny. MR-04 therefore uses explicit consumer-owned
  adapters and failure injection; it may not use a remote shell or claim real
  host isolation.
- `recover --rejoin`, production adapters, and matching-commit three-host
  qualification remain later slices. Admission changes no capability
  qualification; `deployment.multi-host` remains `Q=no`.

## MR-04 implementation checkpoint — 2026-07-29

- PW3-21 is `ready-for-human` at exact implementation tip
  `fa942e9f52ea7ae2fc4ddf9db81322fd72732c09`; its logical implementation
  range is `5ba408f..fa942e9`.
- The R1 coordinator and strict journal implement crash-resumable fencing
  through durable isolation evidence, exact Authority evidence acceptance,
  checkpoint persistence, and both survivor acknowledgements.
- The security audit confirmed and corrected a HIGH pre-remediation Authority
  authorization/evidence-provenance defect. Final exact Node authorization,
  fail-closed receipt verification, signed verification provenance,
  duplicate-safe one-to-one checkpoint/audit/evidence cross-binding,
  removed-target-only fencing, and independent survivor receipts passed
  regression review; no confirmed finding remains.
- Full, race, tooling, architecture, capability, API-generation, vet and
  vulnerability checks pass. No production host adapter or receipt verifier
  is composed in R1, so fresh production fence evidence remains unavailable
  until R3 rather than being accepted without proof.
- No real host, network, Authority, repository, production state, deployment,
  qualification, capability promotion, or push occurred.
  `deployment.multi-host` remains `Q=no`.
- The maintainer accepted exact MR-04 fencing implementation tip
  `fa942e9f52ea7ae2fc4ddf9db81322fd72732c09` and closed PW3-21. This accepts
  the bounded R1 fencing transaction only. The canonical MR-04 `Rejoin`
  behavior remains unimplemented and must be admitted separately before MR-05;
  `deployment.multi-host` remains `Q=no`.

## MR-04b admission review checkpoint — 2026-07-29

- PW3-22 reviews the remaining R1 Rejoin portion of canonical MR-04 after exact
  MR-04a acceptance.
- The accepted terminal Fence Transaction and removal checkpoint remain
  immutable.
- Independent Spec review found the accepted ADR-0013 Rejoin ordering
  incompatible with accepted CGA-04: the target's fresh pending delivery must
  be installed before activation commit, while that commit already makes the
  fresh membership and generation current before active receipts.
- A dated compatibility amendment proposes a separate linked Rejoin
  Transaction with phase-truthful recovery. The review is returned with
  blockers and PW3-22 remains `needs-info` until the maintainer explicitly
  accepts that amendment.
- Accepted CGA-04 remains the only owner of membership add, generation,
  checkpoint repository, delivery and active-receipt truth. Rejoin introduces
  no second membership source and cannot reuse an old grant, receipt, or
  removal checkpoint.
- Production restoration/start/SSH/Authority adapters and real three-host
  evidence remain R3. Proposed PW3-22 is R1 consumer-owned coordination and
  compensation only; MR-05 remains blocked until the amendment and slice are
  accepted.
- This review changes no capability qualification; `deployment.multi-host`
  remains `Q=no`.

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
