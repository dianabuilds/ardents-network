# H4-8 — Integration, qualification, and release closure

Status: **accepted H4-8 direction; no alpha result is Public Beta
qualification.**

## Decision

H4-8 is the final integration and closure epic for one explicitly selected H4
release profile. It does four connected jobs:

1. test the complete User-to-Service path, including real multi-host alpha
   operation and selected long-running/hostile simulations;
2. find and repair bugs, operational failures, missing limits, and misleading
   user states exposed by that evidence;
3. re-run the affected evidence until the exact release profile is coherent;
   and
4. leave a maintainable repository: current code and documents have one owner,
   while superseded experiments, stage material, compatibility paths, fixtures,
   and process documents have either a named remaining purpose or are retired.

It is not a final smoke-test bucket into which unfinished H4-1 through H4-7
work disappears. When integration exposes a missing product contract or an
unselected mechanism, that defect returns to its owning epic/research question;
H4-8 records the integration evidence and verifies the repair.

## Selected release profile

Qualification begins only after the Product Owner names one bounded release
profile: exact build and source revision; operating systems; H4-2 Carrier/Entry
profile; network topology; known operator/family and direct-source conditions;
H4-3 Service workload; optional H4-4A name path; resource ceilings; and the
precise claims and non-claims shown to alpha participants.

The first profile should be the smallest usable loop—install Endpoint, become
ready, publish a bounded HTTP Service, share a Target Link or selected Service
Link, open it from another Endpoint, recover or fail explicitly, and stop or
update safely. A more ambitious transport, browser, Contributor, Namespace, or
protected-application profile is another qualification target, not an implicit
extension of a passing result.

### Candidate H4-alpha-1 — readiness profile under qualification

The current endpoint artifact candidate is source revision
`70bf425ef427188694232a6ea873ac3c10f4b5fd`; its complete identity and public
custody companions are recorded in the
[H4-alpha-1 bounded release profile](08b-alpha-1-release-profile.md). It
narrows the existing selected directions into one reproducible qualification
target; this records a candidate and its evidence gates, **not** a release
acceptance or a claim that an immutable participant artifact has been published.

| Boundary | Candidate selection | Evidence gate |
|---|---|---|
| Endpoint | Ubuntu LTS `x86-64` unprivileged Portable with explicit Alpha Enrollment Pin, local replacement, and custody boundaries | H4-1A/B native user-session qualification plus a concrete immutable artifact and independent first-contact handoff |
| Carrier | State-selected TCP/TLS v1 is release-gating. QUIC v1 remains a maintained separately qualified compatible profile, never a fallback or an implicit part of this candidate's release claim. | H4-2 local and two-host TCP/TLS evidence; any QUIC participant claim requires its own selected matrix row |
| Service | One exact Target Link, loopback Browser Adapter, and the dynamic H4-3B HTTP/1.1 workload: POST body/header preservation, cookie/redirect follow-up, chunked response, explicit withdrawal, Publisher Application reset, and Publisher Endpoint loss | local process, constrained Docker, VPS Docker, actual two-host C-2, and selected browser/platform cells; every failure must retain the exact no-fallback oracle |
| Naming | Target Link is the release path. The retained `ardents-alpha://` corpus may support controlled fixture evidence but H4-4 Browser Entry is not release-gating. | no participant `.ard`/DNS/DoH/HTTPS claim |
| Control | enrollment-pinned H4-6A catalog and independently verified Release, Network, Compatibility, and corpus components | two fresh enrolled Endpoints accept or reject the same concrete component identities for the documented reason |

The remaining first-alpha selection is the concrete Ubuntu artifact/contact,
the exact two-host topology, and one browser/version on the Endpoint platform.
Until those inputs exist, their cells are active gates rather than passing
substitutes.

## Delivery slices

### H4-8A — release-readiness matrix

**Goal:** turn the selected profile into a finite, reproducible test plan before
calling it ready.

Register its required deterministic, process, affected-platform, live, soak,
and qualification scenarios under the engineering test policy. Each scenario
states its observable oracle, inputs, topology, platform/host prerequisites,
resource attribution, fault/adversary, duration where relevant, retained
artifacts, and failure/invalid-environment outcome. The R-023 normal,
startup, recovery, overload, and requalification cells apply where their
preconditions are met.

**Done when:** there is no uncategorized “manual final testing.” A scenario is
active with a checked entry point, or explicitly inactive with the decision and
missing prerequisite that prevents activation. A skipped required platform,
Docker capability, privilege, binary, or host is an invalid environment, not a
passing result.

### H4-8B — live alpha and long-running simulations

**Goal:** observe the selected release under conditions that unit and local
process tests cannot reproduce.

Run the complete multi-host alpha journey, including installation, bootstrap,
readiness, publication, destination opening, transport loss/recovery or bounded
failure, Node drain/withdrawal where selected, release replacement, restart,
and removal/recovery boundaries. Run a declared-duration soak only after its
load, topology, observer, resource, and pass/fail contract are fixed; “it ran
for a long time” is not evidence by itself.

Inject the selected failures: source/distributor loss, stale/conflicting state,
Carrier/Entry failure, constrained CPU/memory/file descriptors/bandwidth,
overload, crash/reboot, clock uncertainty, and alpha control expiration. A
live project-operated topology can establish operating evidence; it cannot
prove independent capacity, censorship resistance, or Public Beta readiness.

### H4-8C — remediation and requalification

**Goal:** resolve what integration actually finds rather than hide it behind a
release label.

Each defect receives an owner, affected claim/profile, reproducer or retained
observation, and disposition: fix and re-run, reduce the profile/claim, move
back to its owning H4 epic, or reject the release. A correction updates the
current implementation, behavior tests, technical/operational reference, and
participant-facing limitations together. Rerunning never erases the initial
failure; the evidence records both failure and resolution.

A changed executable, profile, platform, topology constraint, or safety/control
input is a new candidate. Reuse only the evidence whose stated validity covers
the change; repeat the rest.

### H4-8D — repository and documentation closure

**Goal:** make the release repository explain the current product without
requiring a reader to reconstruct stage history.

Create a finite closure inventory of every candidate for retention, promotion,
or removal. For each item, record its current fact/behavior, canonical owner,
inbound references and tests, provenance/evidence value, and disposition.
Promote every surviving current fact to its product, security, ADR, technical,
operations, reference, or development owner before retiring transitional
material.

Remove or retire only targeted material whose unique current fact has been
promoted and whose inbound links, commands, packages, generated artifacts, and
test references are repaired. This includes obsolete experiments, superseded
tracers, unselected compatibility readers, stale fixtures, duplicated process
plans, and stage-only documentation. Git remains provenance; it need not remain
as runnable code or a second specification in the working tree.

Do not delete active evidence, an accepted ADR, or a research record still
needed for a pending decision merely to make the tree look smaller. Likewise,
do not retain a document or package only because it was once reviewed or is
lengthy. The current documentation-ownership and package-map policies are the
acceptance rules for this slice.

### H4-8E — promotion decision

**Goal:** state the strongest truthful release label, which may remain alpha.

Bundle the selected profile's immutable artifacts, complete test results and
denominators, raw observations where required, invalid environments, known
limitations, fixes/requalifications, and release decision. The Product Owner
may accept a bounded alpha result from this evidence. Public Beta additionally
requires all applicable earlier epic gates, effective independent
operator/source capacity, H4-6 public-control evidence, required external
review, and the named qualification evidence; absent people or evidence are a
block, not an invitation to lower the meaning of the label.

## Evidence and exit conditions

Every retained result declares the exact candidate, claim boundary, inputs,
platform, topology, state class, attempt denominator, resources, duration,
faults, failures, and artifacts needed to reproduce its verdict. A successful
end-to-end demo is necessary for the usable alpha but cannot substitute for
overload, recovery, hostile-failure, platform, or cleanup evidence.

H4-8 completes for an alpha only when the selected release profile has a
truthful published contract, all selected active checks are green, every known
failure has a documented disposition, and the closure inventory leaves no
unowned current code or documentation. It completes for Public Beta only when
the independently evidenced promotion gates above are also met.

## Non-goals

- Treating a Product Owner walkthrough, unit pass, project fleet, or partial
  benchmark as Public Beta qualification.
- Calling arbitrary elapsed time a soak test without a declared contract.
- Quietly weakening a claim, suppressing a failing cell, retrying away a
  failure, or retaining stale code/documentation to avoid a decision.
- Destructive bulk deletion without a reviewed closure inventory and repaired
  current owners/references.
- Fabricating independent operators, auditors, builders, custodians, or
  external security review when they do not exist.

## Open Product Owner selections

- Exact build/revision, live topology, resource ceilings, Reference workload,
  and user-visible claim text for the already selected Ubuntu Portable,
  TCP/TLS-only, Target-Link/loopback first-alpha direction once H4-1–H4-3 code
  exists.
- Which live and soak scenarios become active first, with their topology, load,
  duration, resource, and fault contracts.
- The closure-inventory scope and the release decision authority for the first
  alpha handoff.
