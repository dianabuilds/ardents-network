# Feature And Evidence Catalogue Research Packet

## Decision

- Decision owner: Engineering / Product Capability Governance / Release
- Date: 2026-07-25
- Baseline commit:
  `main@180decc1b03f94a6115b59a4046b4795308ec235`
- Research class: R1 bounded investigation
- Recommendation: implement a strict JSON canonical source with one generated
  Markdown projection

The canonical source should be:

```text
docs/engineering/capabilities.json
```

The existing:

```text
docs/engineering/capability-evidence-register.md
```

should become a generated, byte-for-byte checked projection. It must not remain
an independently editable catalogue.

JSON is selected because the repository already uses strict JSON manifests for
architecture and audit traceability, Go can decode it without a new direct
dependency, and a validator can reject unknown fields, duplicate keys, empty
collections and noncanonical paths deterministically. Markdown remains the
reader interface, not the source of truth.

Current maturity of the catalogue capability itself is:

| Dimension | Status | Basis |
|---|---|---|
| Implemented | no | only a manually edited Markdown register exists |
| Reachable | partial | humans can read the register; tooling cannot query a canonical capability model |
| Operable | no | no fail-closed owner/interface/evidence/status/link/domain validation |
| Qualified | no | no validator, negative matrix or CI gate exists |

## User Outcome

An engineer, Operator, product owner or release reviewer can resolve one stable
capability ID and answer:

- what user outcome is supported;
- which domain and implementation owner are responsible;
- which Operator/Application/SDK interface is supported;
- where the capability is observable and recoverable;
- which evidence gates are required and who owns them;
- the current implemented/reachable/operable/qualified state at one exact
  source commit;
- which ADR, research packet and backlog slice governs it;
- which behavior is intentionally unsupported.

CI fails closed when the catalogue is empty, incomplete, inconsistent with
active claims, or attempts to set `Q=yes` without a complete commit-bound gate
set.

## In Scope

- stable capability identifiers and domain coverage;
- user outcome, domain owner, supported interface, implementation owner and
  operability surface;
- required evidence gates and evidence owner;
- current I/R/O/Q state and exact reported source commit;
- R0/R1/R2/R3/Deferred classification;
- ADR, research packet and backlog links;
- constraints and unsupported behavior;
- strict schema, generator, validator and negative tests;
- active-document status projection and drift checks;
- integration with existing static CI and retained evidence metadata.

## Out Of Scope

- changing any product capability, trust model or wire protocol;
- qualifying a capability merely by migrating it into the catalogue;
- replacing testcatalog, audittrace, architecture acceptance or release gates;
- storing secrets, credentials, runtime telemetry or personal data;
- making historical audit findings the current product backlog;
- automatically deciding product scope from package names;
- parsing arbitrary natural-language claims with heuristics;
- creating a second manually edited Markdown register.

## Current Supported Interface

The current reader interface is
`docs/engineering/capability-evidence-register.md`. It has eight vertical
sections and a summary table with I/R/O/Q and research class. It also identifies
current interface, implementation, evidence, gaps and disposition.

The current evidence/tooling interfaces are separate:

- tagged scenarios are machine catalogued by `tests/tooling/testcatalog`;
- audit finding-to-gate traceability is owned by
  `tests/ci/audit-test-traceability.json` and
  `tests/tooling/audittrace`;
- architecture ownership and composition are owned by
  `docs/engineering/architecture-acceptance.json` and
  `tests/tooling/archaccept`;
- active documentation constraints are checked by
  `tests/tooling/doccontract`;
- CI jobs and retained artifacts are declared in `.github/workflows/ci.yml`;
- release material identity is governed by ADR 0009 and release tooling.

No current interface joins those sources at capability level. The Markdown
register can say that evidence exists, but tooling cannot establish that an
owner, supported interface or complete gate set is present.

## Current Reachable User Journey

Today a reviewer must:

1. find a prose vertical in the register;
2. infer a capability name that has no stable ID;
3. follow prose paths to production modules and tests;
4. inspect testcatalog/CI manually;
5. compare the remediation ledger and research plan;
6. decide whether `implemented`, `reachable`, `operable` and `qualified` were
   used consistently;
7. separately inspect README/Changelog for release overclaim.

That path is readable but not machine-verifiable. A missing vertical, owner,
gate or link is silent.

The proposed journey is:

```text
capability ID
  -> strict canonical JSON record
  -> existing production/interface/link existence checks
  -> required gate definitions and evidence snapshot
  -> computed/validated I/R/O/Q consistency
  -> generated Markdown reader projection
  -> active-document generated status blocks
  -> CI fail closed
```

## Current Implementation

There is no canonical capability data model or validator.

Reusable repository patterns do exist:

- `audittrace` uses a strict JSON manifest, rejects unknown fields, validates
  repository paths, joins evidence to CI gates and checks changed critical
  files (`tests/tooling/audittrace/main.go:20-240`);
- `archaccept` joins a JSON architecture policy to production packages,
  protobuf services and composition roots;
- `testcatalog` parses tagged scenario metadata and rejects malformed entries;
- static CI runs these tools and rejects an empty tagged catalogue
  (`.github/workflows/ci.yml:42-83`);
- `doccontract` owns active-document policy and link/claim checks.

The new catalogue validator should compose those sources by reference. It
should not reimplement their parsers or create a parallel evidence system.

## Existing Deterministic Evidence

Executed on
`180decc1b03f94a6115b59a4046b4795308ec235`:

```text
go run ./tests/tooling/testcatalog -tags "integration e2e" ./tests/...
```

Result: passed with 142 entries.

The existing local tooling/unit slices used by the first two packets also pass.
They prove that current implementation and scenario metadata can seed the
catalogue. They do not prove catalogue completeness because the catalogue and
validator do not exist.

## Historical Evidence

`docs/engineering/evidence/stabilization-baseline-75471a6.md` is retained
commit-bound predecessor evidence. It can be linked as a snapshot for
capabilities whose gates it actually covers, but it cannot set `Q=yes`: the
document explicitly records unavailable Docker/Linux/native/multi-host/release
gates.

`docs/audit/2026-07-23/03-audit-coverage.md` and
`04-findings-register.md` remain immutable historical sources. The catalogue
may link the current remediation ledger, not reinterpret historical findings
as active capability gaps.

## Proposed Canonical Data Model

Illustrative abbreviated JSON:

```json
{
  "schema_version": 1,
  "reported_source_commit": "180decc1b03f94a6115b59a4046b4795308ec235",
  "domains": [
    {
      "id": "application",
      "owner": "Application Interface",
      "required": true
    }
  ],
  "evidence_gates": [
    {
      "id": "application-linux-e2e",
      "owner": "Application Interface / QA",
      "kind": "tagged_scenario",
      "ci_job": "e2e",
      "required_environment": "linux-container"
    }
  ],
  "qualification_snapshots": [],
  "capabilities": [
    {
      "id": "application.installation-content",
      "user_outcome": "An Application enrolls as its own Principal and puts and gets immutable content.",
      "domain": "application",
      "domain_owner": "Application Interface",
      "supported_interfaces": [
        {
          "id": "go-sdk-v1",
          "kind": "sdk",
          "contracts": [
            "sdk/go/client/enrollment.go",
            "sdk/go/client/client.go"
          ]
        }
      ],
      "implementation_owners": [
        "internal/identity/access",
        "internal/applicationapi",
        "sdk/go"
      ],
      "operability_surfaces": [
        "ardentsctl identity application-ticket issue",
        "Application typed errors",
        "Operator grant and device revocation"
      ],
      "evidence_owner": "Application Interface / QA",
      "required_evidence_gates": [
        "application-unit",
        "application-linux-e2e",
        "release"
      ],
      "status": {
        "implemented": "yes",
        "reachable": "yes",
        "operable": "partial",
        "qualified": "no",
        "at_commit": "180decc1b03f94a6115b59a4046b4795308ec235",
        "qualification_snapshot": ""
      },
      "research_class": "R1",
      "adrs": [
        "docs/adr/0001-separate-application-interface.md",
        "docs/adr/0005-recoverable-one-time-ticket-handoff.md"
      ],
      "research_packets": [
        "docs/engineering/research/application-installation-journey.md"
      ],
      "backlog": [],
      "constraints": [
        "Application Interface is local Unix only.",
        "Unary Content is bounded."
      ],
      "unsupported_behavior": [
        "Remote Application transport",
        "Application Messaging",
        "Application Hosting"
      ],
      "active_documents": [
        "docs/product/application-api-and-sdk.md"
      ]
    }
  ]
}
```

### Required domains

The validator owns a closed first-version domain allowlist derived from the
current register and global plan:

```text
node
identity
application
network
discovery
content-transfer
workload-hosting
operations-release
```

Every required domain must appear exactly once and contain at least one
capability. Changing the allowlist is a reviewed schema/tooling change. This
independent check prevents an editor from silently removing a domain from both
the domain and capability arrays.

Deferred and explicitly unsupported directions remain capabilities in their
own owning domain with `supported_interfaces.kind = "none"` plus a mandatory
unsupported reason. Absence is not used to represent a product decision.

### Initial capability IDs

The first migration should refine the eight prose verticals into outcome-sized
stable records:

| Stable ID | Current I/R/O/Q at `180decc...` | Research class |
|---|---|---|
| `node.lifecycle` | yes / yes / yes / no | R3 |
| `operator.command-interface` | yes / partial / partial / no | R1/R3, canonical value `R1` until OCS slices complete |
| `identity.principal-access` | yes / yes / yes / no | R3 |
| `application.installation-content` | yes / yes / partial / no | R1 |
| `application.discovery` | no / no / no / no | R0 after completed R1 packet; links AD packet, does not duplicate AD issues |
| `application.messaging` | no / no / no / no | R2 |
| `network.waku-foundation` | yes / operator / yes / no, normalized as reachable `partial` | R2 |
| `discovery.operator-resolution` | yes / yes / yes / no | R3 |
| `content.operator-lifecycle` | yes / yes / partial / no | R1/R3, canonical `R1` while command smoke is open |
| `transfer.replication` | yes / partial / yes / no | R2 |
| `workload.lifecycle` | yes / yes / partial / no | R3 |
| `hosting.operator-publication` | yes / yes / partial / no | R3 |
| `application.hosting` | no / no / no / no | R2 |
| `service.direct-interaction` | partial / no / no / no | R2 |
| `operations.diagnostics` | yes / yes / partial / no | R1/R3, canonical `R1` while command smoke is open |
| `operations.configuration-reload` | yes / yes / yes / no | R3 |
| `operations.backup-upgrade-rollback` | yes / yes / partial / no | R3 |
| `operations.native-installation` | yes / yes / partial / no | R3 |
| `release.artifacts-provenance` | yes / yes / partial / no | R3 |
| `realm.channel-grant-authority` | deployment-owned partial / no / no / no, normalized as implemented `partial` | R2 |
| `deployment.multi-host` | partial / no / no / no | R2 |
| `deployment.kubernetes` | no / no / no / no | Deferred |
| `network.quic-webtransport-webrtc` | no / no / no / no | Deferred |
| `sdk.non-go-or-remote` | no / no / no / no | Deferred |

The canonical schema uses only one research class. Mixed prose values such as
`R1/R3` are resolved to the class of the next blocking work; later
qualification remains represented by required evidence gates and `Q=no`.

The normalized status vocabulary is:

```text
implemented, reachable, operable: yes | partial | no
qualified: yes | no
research_class: R0 | R1 | R2 | R3 | Deferred
```

Values such as `Operator`, `mixed`, `candidate`, `locally_verified` or an empty
string are rejected in I/R/O/Q. Those concepts belong in interface records,
evidence snapshots or constraints.

## Qualification Snapshot Model

A qualification snapshot is immutable metadata:

```json
{
  "id": "release-2026-08-01-<commit>",
  "source_commit": "<40 lowercase hex>",
  "environment": "canonical-linux-release",
  "results": [
    {
      "gate": "application-linux-e2e",
      "outcome": "passed",
      "artifact": "docs/engineering/evidence/<file>",
      "sha256": "<64 lowercase hex>"
    }
  ]
}
```

`Q=yes` is valid only when:

1. `status.at_commit` and snapshot `source_commit` are the same full commit;
2. every capability-required gate appears exactly once;
3. every result is `passed`;
4. every retained artifact path exists and its SHA-256 matches;
5. every gate owner and CI job/environment exists;
6. the snapshot contains the complete release gate required by the selected
   product scope;
7. no result uses a retry to replace an earlier failure without a separately
   recorded clean rerun policy.

Local unit, Windows race, historical report or tagged-test existence may support
I/R/O, but cannot satisfy a required canonical environment gate unless that
gate explicitly names the environment.

## Active Documentation Contract

Natural-language truth cannot be safely inferred. Active documents that state
maturity must instead contain generated blocks:

```markdown
<!-- capability-status:begin application.installation-content -->
...generated status and constraints...
<!-- capability-status:end application.installation-content -->
```

The validator:

- regenerates each block from canonical JSON and compares bytes;
- rejects unknown/duplicate/misnested capability markers;
- rejects an `active_documents` reference without the exact block;
- scans the active-document allowlist maintained by `doccontract` for
  capability-status markers and rejects markers not declared in the catalogue;
- keeps general explanatory prose outside the block, but requires readiness
  claims to be inside or linked to the generated block.

README and Changelog retain their current stabilization/no-production-release
language. The catalogue must not generate `Q=yes` or production-ready wording
while required release gates are missing.

## Missing Or Unreachable Behavior

- no stable capability IDs;
- eight verticals mix multiple independently mature outcomes;
- free-form statuses such as `Operator`, `mixed` and `R1/R3` cannot be
  validated;
- no explicit evidence owner per capability;
- evidence prose does not identify a complete required gate set;
- no commit-bound qualification snapshot joins all required gates;
- ADR/research/backlog paths can silently become stale;
- an entire domain can be removed without a failure;
- active documents can drift from the register;
- Markdown tables cannot express nested interfaces, evidence snapshots and
  constraints without ambiguous delimiter/escaping rules;
- no CI entry point generates and checks the reader projection.

## Actors, Assets And Trust Assumptions

### Actors

- domain owner: owns user outcome and supported interface;
- implementation owner: owns production modules;
- evidence owner: owns gate definitions and retained results;
- research/decision owner: owns R1/R2 packet and ADR decisions;
- release reviewer: may approve `Q=yes` only from complete snapshots;
- CI: validates structure and joins but does not invent product truth.

### Assets

- canonical capability identity and status;
- supported-interface and unsupported-behavior claims;
- evidence ownership and commit-bound results;
- ADR, research and backlog traceability;
- generated reader projection and active-document status blocks.

### Trust assumptions

- Git commit identity and retained artifact hashes are authoritative;
- repository paths are canonical, relative and reviewable;
- CI job names and testcatalog metadata are machine-readable;
- domain/release owners review semantic changes; the validator checks
  consistency, not business correctness;
- historical evidence remains immutable and is not silently promoted.

## Proposed Module Boundary And External Interface

The external engineering interface consists of:

```text
docs/engineering/capabilities.json
docs/engineering/capability-evidence-register.md  # generated
go run ./tests/tooling/capabilitycatalog -check
go run ./tests/tooling/capabilitycatalog -generate
```

`-check` is read-only and is the CI entry point. `-generate` rewrites only the
known generated Markdown projection/blocks and is an explicit maintainer
action.

The JSON schema is versioned. A breaking representation change increments
`schema_version` and updates the validator/generator in the same change.
Capability IDs remain stable across schema versions; replacement requires an
explicit `supersedes`/`superseded_by` relation rather than silent rename.

## Proposed Internal Seam

Add `tests/tooling/capabilitycatalog` with narrow adapters:

- strict JSON loader with duplicate-key and unknown-field rejection;
- model validator independent of rendering;
- repository reference checker for production/interface/ADR/research/backlog
  paths;
- testcatalog adapter for tagged scenario IDs;
- CI workflow adapter for gate job existence;
- evidence snapshot/hash validator;
- deterministic Markdown renderer;
- active-document generated-block checker;
- optional `--base` diff check that requires affected capability/evidence
  ownership when declared implementation or interface paths change.

The tool consumes existing catalogues; it does not import production internal
packages or become runtime code.

## Dependencies

### In-process

- JSON model, validator and Markdown generator;
- architecture acceptance manifest;
- testcatalog output/metadata;
- CI workflow and audittrace gate vocabulary;
- doccontract active-document allowlist;
- Git diff/source commit inspection for optional changed-path coverage.

### Local-substitutable

- filesystem repository fixture;
- synthetic JSON, Markdown, CI and testcatalog fixtures for negative tests;
- deterministic SHA-256 artifact fixtures.

### Remote But Owned

- hosted CI jobs that produce retained evidence artifacts;
- canonical release runner environment.

The validator checks declarations and retained results locally; it does not
start a remote job.

### True External

- Git hosting/CI artifact retention;
- container registries and external scanners required by release gates.

Their results are references in a snapshot, never silently treated as locally
available.

## Alternatives

### A. Canonical Markdown tables

Advantages:

- easiest for humans to edit;
- smallest change from the current register;
- renders without generation.

Disadvantages:

- nested interfaces, evidence sets and unsupported constraints require encoded
  delimiters or multiple loosely joined tables;
- duplicate IDs/fields and unknown columns are harder to reject;
- Markdown escaping and multiline values complicate deterministic parsing;
- empty or omitted domains remain easy to miss;
- comments/prose can contradict parsed cells.

Rejected as the source of truth. Markdown remains the generated projection.

### B. Canonical YAML with generated Markdown

Advantages:

- concise and comment-friendly;
- readable nested records.

Disadvantages:

- duplicate-key, anchors, implicit scalar typing and parser-mode decisions add
  avoidable validator surface;
- the repository's YAML dependency is currently indirect;
- comments can become a second unvalidated truth channel.

Viable, but not selected.

### C. Canonical strict JSON with generated Markdown

Advantages:

- matches current architecture/audit manifest practice;
- standard-library decoding and deterministic formatting;
- simple unknown-field, duplicate-key, enum and path validation;
- nested evidence snapshots remain explicit;
- no comments means all normative claims are schema fields.

Disadvantages:

- verbose and less pleasant for manual editing;
- requires an explicit generator for the reader view.

Selected.

### D. Store status only in test/CI metadata and derive capabilities

Rejected. Test names and packages cannot define user outcomes, supported
interfaces, product constraints or Deferred decisions.

## Failure, Retry, Restart And Recovery Behavior

| Condition | Validator/generator behavior |
|---|---|
| JSON missing, empty or malformed | fail before rendering; never preserve a stale projection as success |
| duplicate/unknown JSON key | fail closed with exact JSON path |
| unknown enum/status/research class | fail closed |
| capability/domain/owner/interface/evidence owner missing | fail closed |
| required domain absent or empty | fail closed |
| repository/ADR/research/backlog path absent | fail closed |
| unknown scenario/gate/CI job | fail closed |
| generated Markdown differs | `-check` fails and prints target; `-generate` deterministically replaces only generated targets |
| active block missing/stale/duplicated | fail closed |
| `Q=yes` with missing/mixed-commit/noncanonical evidence | fail closed |
| artifact hash mismatch | fail closed; never retry or update hash automatically |
| CI interruption | no status mutation; rerun produces a new explicit snapshot/result |
| product process restart | not applicable; catalogue is repository state |
| schema upgrade | old schema fails with explicit version error until migrated atomically with tool |

Generation writes temporary files in the target directory and atomically
replaces known outputs only after complete validation. A failed generation
leaves the prior files intact and returns nonzero.

## Security, Privacy And Abuse Analysis

- catalogue and evidence metadata must contain no secrets, Credentials, Session
  IDs, personal contact data or private artifact URLs;
- repository paths are relative, canonical and cannot escape the workspace;
- artifact hashes prevent a changed retained file from inheriting old status;
- active docs cannot independently promote readiness;
- `Q=yes` is impossible from test existence alone;
- external/manual gates require an owned retained artifact and cannot be
  represented as an unchecked boolean;
- stable IDs and supersession prevent silent capability deletion/rename;
- capability arrays and strings have bounded sizes to prevent pathological CI
  input;
- generator never executes commands found in JSON or Markdown;
- historical audit references remain labeled historical and cannot be evidence
  results for another commit.

## Observability And Operator Actions

The validator prints only bounded counts and exact record/path errors:

```text
capability catalogue valid: 24 capabilities, 8 domains, 0 qualified
```

CI retains the generated Markdown diff/check output and an optional normalized
JSON projection as static evidence. It must not emit secrets or entire external
artifact bodies.

Owner recovery procedure:

1. resolve the failing capability ID and JSON path;
2. correct the canonical JSON or the referenced owner/source;
3. regenerate Markdown explicitly;
4. run negative/unit checks and `-check`;
5. never set `Q=yes` merely to silence a missing gate;
6. if a gate is genuinely not required, change the capability scope/gate
   decision with its domain and release owners.

## Acceptance Matrix

| Level | Required evidence |
|---|---|
| Strict loader | rejects empty file/catalogue, duplicate keys, unknown fields, wrong schema, invalid UTF-8 and trailing JSON |
| Identity | IDs are canonical, unique and stable; supersession is explicit and acyclic |
| Ownership/interface | every capability has domain owner, implementation owner, supported interface, operability surface and evidence owner; `none` interface requires unsupported reason |
| Status | only known I/R/O/Q and research values; Q is boolean yes/no; status commit is full lowercase Git SHA |
| Domain coverage | exact required domain allowlist is present; every domain nonempty; unknown domain and silently omitted domain fail |
| References | every contract, implementation path, ADR, research packet and backlog file exists and is canonical; missing links fail |
| Evidence gates | gate IDs/owners/kinds/environments are unique and known; tagged scenarios and CI jobs exist |
| Qualification | Q=true requires every required gate passed once in one matching commit-bound snapshot with existing hash-matching artifacts and release gate |
| Projection | generated Markdown is deterministic and `-check` rejects any byte drift; JSON is the only editable truth |
| Active docs | generated status blocks match JSON; missing/unknown/duplicate blocks and readiness overclaim fail |
| Negative matrix | one fixture for every required failure in the user request, including empty catalogue and omitted domain |
| CI | static job runs unit tests, `-check`, nonempty guard and negative matrix; outputs are retained |
| Current migration | all initial IDs above are represented; AD research is linked but AD-01 through AD-04 are not duplicated in the new backlog |

## Open Questions

No open question changes the selected canonical format, file boundary,
validator entry point or first-version status vocabulary.

The exact capability granularity may add records during migration, but every
record must remain an independently understandable user outcome and every
required domain must stay nonempty. Splitting an ID after publication requires
explicit supersession; it is not an untracked rename.

## Recommendation

Implement strict JSON plus a generated Markdown projection. Seed it from the
current register using the initial IDs above, normalize mixed statuses, and
record all Q values as `no`. Add the qualification/active-claim checks before
any capability may be promoted.

Do not hand-edit both files, infer qualification from test existence, or use
the catalogue migration to claim production readiness.

## Vertically Sliced Issues And Dependency Order

### FEC-01 - Establish one canonical capability truth

- Parent: R1 Existing Product Truth / Feature and evidence catalogue
- User story: As an engineer or product owner, I can resolve every current
  Ardents capability by stable ID in one machine-readable and human-readable
  source.
- What to build: strict `capabilities.json` schema/model, required domain
  coverage, initial capability migration, deterministic Markdown generator,
  path/owner/interface/status/link validation, generated register conversion
  and static CI check.
- Acceptance criteria: loader, identity, ownership/interface, status, domain,
  references, projection and current-migration rows above pass; JSON is the
  sole editable source; catalogue reports Q=no for all current capabilities.
- Blocked by: none.
- Research class: R1 resolved to implementation.
- Proposed status: `ready-for-agent`.

### FEC-02 - Make evidence promotion and active claims fail closed

- Parent: R1 Existing Product Truth / Feature and evidence catalogue
- User story: As a release reviewer, I cannot promote or document a capability
  as qualified without one complete commit-bound evidence set.
- What to build: evidence gate/snapshot model, testcatalog/CI/artifact/hash
  joins, Q promotion rules, active-document generated blocks, negative matrix,
  README/Changelog readiness guard and retained static evidence.
- Acceptance criteria: evidence, qualification, active-doc, negative-matrix and
  CI rows above pass; mixed commits, absent artifacts, unknown jobs, omitted
  domains and stale docs all fail; no current Q status changes to yes.
- Blocked by: FEC-01.
- Research class: R1 resolved to implementation; actual environment execution
  remains R3.
- Proposed status: `ready-for-agent`.

Dependency order:

```text
FEC-01 canonical catalogue and projection
  -> FEC-02 evidence promotion and active claims
  -> existing R3 qualification snapshots
```

