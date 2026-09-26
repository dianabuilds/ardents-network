# Naming and the retired Namespace

Status: **current maintained technical contract.** This document owns the
canonical Naming input gate (`internal/naming`), the `ardents name encode`
command, and the retirement boundaries of the former Name network surface.
The whole `internal/naming/namespace` control subsystem was removed by
ADR-0105; the former `internal/naming/resolution` transport package was
removed earlier by ADR-0100. Neither is a supported public Namespace, a
public resolver, or a claim of independent Network Epoch operation.

The [closed successor workload](../product/protected-service-workload.md)
uses Target Link first. Canonical Names remain a possible future stage that
would require an entirely new scoped design; ADR-0081 does not authorize an
alpha alias or simulated close to satisfy this boundary.

## Ownership and trust boundary

`internal/naming` owns canonical Service Name parsing, normalization, and
deterministic encoding checks as production-owned input gates. It has no
Namespace state, no transport, and no dependency beyond the standard
library. `ardents name encode` is its only operator command.

The former Namespace subsystem owned Authority/lifecycle transitions,
Recovery verification, claim evidence verification, admission-owned naming
facts, pending successor persistence, current Namespace materialization, and
local current-proof verification across seven packages
(`internal/naming/namespace` and its `admission`, `authority`, `claim`,
`epoch`, `record`, and `recovery` submodules). No maintained production
runtime ever composed that sequence into an operator path: ADR-0090 retired
the `name resolve`/`name control` adapters, ADR-0100 removed the private
resolution transport, and ADR-0105 deleted the subsystem itself after the
product owner confirmed that no deployed Namespace root deserves data
support or backward compatibility.

## Namespace subsystem retirement (ADR-0105)

The disposition is **typed incompatibility through absence**: no working-tree
code can open, read, convert, materialize, or delete an old Namespace root.

- An existing Namespace root stays on disk byte-for-byte. Nothing reads it as
  the new format, nothing converts it, and nothing deletes its data; the
  absence of any read path is itself the incompatibility error.
- The only operator commands that formerly touched such a root, `name
  resolve` and `name control`, keep their exact pre-effect refusal (below).
- Custody lost its three never-command-exposed Namespace operations
  (`sign-namespace-transition`, `prepare-namespace-submission`,
  `activate-recovered-authority`) together with their operation kinds,
  receipt proof/submission fields, and preparation/reconciliation files.
  The `AuthorityName` record kind remains recognizable to the generic
  custody envelope grammar so existing Name Authority vault records stay
  inspectable, verifiable, exportable, and purgeable through the ordinary
  record commands; that recognition needs no Namespace import.
- The uncalled generation-2 reachability writers (`Issue`, `Store.Publish`,
  `Store.Lookup` and their private helpers) retired under the same decision,
  per the deadcode registry rule that removes that tracer group with its
  superseding service decision. The then-retained v1/v2 decode grammar and
  floor comparison were subsequently deleted by ADR-0109, which closed card
  F-32 with a typed refusal of stored legacy records.
- `ardents name encode`, canonical Naming bytes, and the retirement refusals
  are unchanged.

A future confidential Name exchange or protected Service Name access
requires a separately selected authority/wire design and a fresh dependency
review, not revival of any removed adapter or module.

## Retained technical limits

| Boundary | Enforced current limit | Status / owner |
|---|---:|---|
| Canonical Name V1 | lowercase ASCII labels 1-63 bytes; total <=253 bytes; depth <=127 | retained R-041 profile, `internal/naming` |

The former Namespace corpus, journal, proof, and admission-profile limits
died with the subsystem; their historical values are evidence in the ADR
chain and the retired research dossiers, not current bounds.

## Operator Name network command retirement

[ADR-0090](../adr/0090-retire-name-operator-network-adapters.md) selects
retirement of the old `ardents name resolve` and `ardents name control`
HTTP/OHTTP adapters. A recognized command must refuse at command dispatch,
before validating its remaining arguments, decoding context, reading an input
or operation file, opening Network State, constructing or using transport,
writing output, or changing any durable state. The operator result explains
that the old Name network command is retired and protected Service Name
access is not yet selected; it supplies no Target-Link, AAI3, DNS, or other
fallback. The command enforces this boundary with the exact refusal `name
network command is retired; protected Service Name access is not selected`.
Its former plan/context, State-view, receipt, and operation adapters are
absent.

This command decision does not retire the Service Name grammar itself.
`ardents name encode` and canonical Naming bytes remain unchanged. ADR-0100
removed the private-resolution transport package, its tests, and its OHTTP
dependency closure; ADR-0105 then removed the Namespace subsystem that the
transport once served. Protected Service Name access remains not selected.

No existing State root, Namespace root, Record, journal, floor, proof, or
authority material is converted, reset, deleted, or promoted into successor
authority. No successor wire, Resolver/Gateway topology, authority,
governance, migration, or AAI3 Name path is selected. Until one is
separately selected and delivered, there is no maintained operator command
for network Name resolution or control.

The command regression executes the recognized resolve and control verbs
against a bounded fixture that materializes an authenticated State root, a
synthetic root shaped like the retired Namespace store, and plan/operation
files shaped like the former adapter inputs. Since ADR-0105 the fixture
composes no Namespace machinery: no working-tree code could read a real
committed root anyway. The regression proves zero transport attempts, no
output, and byte-for-byte unchanged contents of both durable roots after all
retired command invocations. Canonical encoding retains its exact behavior
vector.

## Alpha corpus compatibility

The Alpha Name Corpus is a separately authenticated historical local
overlay; it was never canonical Namespace state. Fresh corpus intake is
retired before any floor effect; the
[command reference](../reference/commands.md#ardents-control) owns that
observable refusal and the retained read-only inspection route. Existing
accepted floor bytes remain unchanged evidence, including their serial,
digest, withdrawal, rollback, and conflict facts; intake retirement does not
make them authority for new acceptance or fallback. No maintained Endpoint
destination consumes those floors: the historical Alpha prefix is recognized
only for its final refusal before any floor read or resolver work. The
parser and persistent reader remain solely as the ADR-0088 compatibility
obligation pending a separate data-retention decision.

[ADR-0088](../adr/0088-retire-alpha-service-links-and-corpus-intake.md)
retires fresh intake and live Alpha Service Links without grace or
conversion. The
[Endpoint contract](endpoint-service-runtime.md#alpha-destination-retirement)
owns the refusal order and retained-floor boundary.

There is no current C0 participant intake procedure or promoted corpus
download source. Fresh C0 Endpoint plans use Target Links; the complete
historical alpha plan is retained only under the
[Endpoint compatibility contract](endpoint-service-runtime.md#endpoint-process-contract).
The deferred [intake template](https://github.com/dianabuilds/ardents-network/blob/f82a52dde912e975df0b23bdcf459f1e5b71def3/docs/product/closed-alpha-name-corpus.md)
and [cohort notice](https://github.com/dianabuilds/ardents-network/blob/f82a52dde912e975df0b23bdcf459f1e5b71def3/docs/product/closed-alpha-name-corpus-notice-template.md)
are archived at that revision. They must not be followed as current
commands. Reintroduction requires an explicitly selected operator contract
compatible with current enrollment, not reuse of an old enrollment JSON
instruction.

## Verification

The maintained local gate is `make quick-check`; `make check` is required
before integration. The retirement boundaries are covered by the zero-effect
refusal oracle (`go test ./cmd/ardents/ -run
TestNameNetworkCommandsRetireBeforeEffects -count=1`), the architecture
guards in `internal/architecture/naming_resolution_retirement_test.go`
(subsystem and writer absence, exact refusal, bounded synthetic fixture),
and `go test ./internal/naming/... -count=1` for the canonical gate.

## Governing decisions

The maintained contract above is authoritative. The removed Namespace
subsystem historical choices are recorded by
[ADR-0017](../adr/0017-authenticated-name-claim-ordering.md),
[ADR-0018](../adr/0018-threshold-recovery-multisignatures.md),
[ADR-0019](../adr/0019-bounded-anonymous-name-admission.md),
[ADR-0020](../adr/0020-authenticate-current-namespace-materialization.md),
[ADR-0022](../adr/0022-bind-name-record-validity.md), and
[ADR-0023](../adr/0023-pending-signed-namespace-successors.md). Its
retirement chain is
[ADR-0090](../adr/0090-retire-name-operator-network-adapters.md) (operator
adapters),
[ADR-0100](../adr/0100-remove-private-resolution-transport.md) (transport
package), and ADR-0105 (the subsystem itself, the unexposed custody
operations, and the generation-2 reachability writers). Historical research
dossiers were retired after their decisions and behavior were promoted
here, into ADRs, and into tests.
