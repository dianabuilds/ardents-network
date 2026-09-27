# Naming: fully retired surface

Status: **retirement record, not a maintained contract.** This document
records the boundaries of the former Naming surface. ADR-0113 deleted
`internal/naming` — the canonical Name input gate, the frozen Stage 6 wire
grammar, and the `ardents name encode` command behavior — together with the
retained `internal/naming/alpha` compatibility surface (corpus parser,
alpha-only Service Link grammar, and the persistent/session floor readers
kept under ADR-0088). The whole `internal/naming/namespace` control
subsystem was removed earlier by ADR-0105; the former
`internal/naming/resolution` transport package was removed by ADR-0100.
Nothing here is a supported public Namespace, a public resolver, or a claim
of independent Network Epoch operation.

The [closed successor workload](../product/protected-service-workload.md)
uses Target Link first. Canonical Names remain a possible future stage that
would require an entirely new scoped design; ADR-0081 does not authorize an
alpha alias or simulated close to satisfy this boundary.

## Name command family retirement

Every `ardents name` verb refuses at command dispatch, before validating
remaining arguments, decoding context, reading an input or operation file,
opening Network State, constructing or using transport, writing output, or
changing any durable state:

- `name resolve` and `name control` retired under ADR-0090 (network
  adapters), with their transport and subsystem removed by ADR-0100 and
  ADR-0105.
- `name encode` retired under ADR-0113 together with its package: the local
  canonical Stage 6 wire encoder died with its final consumer. Its frozen
  behavior vector (`alice` → `000105616c696365`) survives only as inlined
  bytes inside the retirement fixture, which never executes an encoder.

The operator result explains that the name network command is retired and
protected Service Name access is not selected; it supplies no Target-Link,
AAI3, DNS, encoding, or other fallback. The command enforces this boundary
with the exact refusal `name network command is retired; protected Service
Name access is not selected`. No plan/context, State-view, receipt,
operation, or wire-encoding adapter remains.

The command regression executes all three recognized verbs against a bounded
fixture that materializes an authenticated State root, a synthetic root
shaped like the retired Namespace store, and plan/operation files shaped
like the former adapter inputs. No working-tree code composes a Gateway,
Resolver, or Namespace reader for these files. The regression proves zero
transport attempts, no output, and byte-for-byte unchanged contents of both
durable roots after every retired command invocation.

## Alpha corpus and floor compatibility: retired

The Alpha Name Corpus was a separately authenticated historical local
overlay; it was never canonical Namespace state. Fresh corpus intake retired
before any floor effect under ADR-0088/#232, the ACA2 inspection command
retired under ADR-0110, and ADR-0113 then deleted the retained parser and
floor readers outright:

- Existing corpus-floor and corpus bytes on disk stay byte-for-byte. No
  maintained code can read, convert, or delete them; the absence of any read
  path is itself the incompatibility error (the ADR-0105/ADR-0109
  precedent). No migration, conversion, or grace reader is provided.
- The retired intake evidence survives as refusal-before-effects plus
  byte-for-byte floor preservation, rebuilt in the e2e test with a
  test-local historic corpus builder and synthetic floor-shaped bytes.
- The historical Alpha Service Link prefix is still recognized by the
  Endpoint solely for its final refusal before any floor read or resolver
  work; the link grammar itself is deleted.

[ADR-0088](../adr/0088-retire-alpha-service-links-and-corpus-intake.md)
retires fresh intake and live Alpha Service Links without grace or
conversion; its retained reader obligation is superseded by
[ADR-0113](../adr/0113-retire-retained-alpha-compatibility-surface.md). The
[Endpoint contract](endpoint-service-runtime.md#alpha-destination-retirement)
owns the refusal order and unchanged-floor boundary.

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

No existing State root, Namespace root, Record, journal, floor, proof, or
authority material is converted, reset, deleted, or promoted into successor
authority. No successor wire, topology, authority, governance, migration, or
AAI3 Name path is selected. A future confidential Name exchange or protected
Service Name access requires a separately selected authority/wire design and
a fresh dependency review, not revival of any removed adapter, grammar, or
module.

## Verification

The maintained local gate is `make quick-check`; `make check` is required
before integration. The retirement boundary is covered by the zero-effect
refusal oracle (`go test ./cmd/ardents/ -run
TestNameNetworkCommandsRetireBeforeEffects -count=1`) and the Linux e2e
retired-intake refusal test. The package map and test-profile registry check
current packages and selected test execution.

## Governing decisions

The retirement chain is
[ADR-0090](../adr/0090-retire-name-operator-network-adapters.md) (operator
adapters),
[ADR-0100](../adr/0100-remove-private-resolution-transport.md) (transport
package), ADR-0105 (the Namespace subsystem, the unexposed custody
operations, and the generation-2 reachability writers), ADR-0098 (the
unwired Service-Link tracer), and
[ADR-0113](../adr/0113-retire-retained-alpha-compatibility-surface.md) (the
canonical wire encoder, the `name encode` behavior, and the retained Alpha
corpus/link/floor compatibility surface). The removed Namespace subsystem
historical choices are recorded by
[ADR-0017](../adr/0017-authenticated-name-claim-ordering.md),
[ADR-0018](../adr/0018-threshold-recovery-multisignatures.md),
[ADR-0019](../adr/0019-bounded-anonymous-name-admission.md),
[ADR-0020](../adr/0020-authenticate-current-namespace-materialization.md),
[ADR-0022](../adr/0022-bind-name-record-validity.md), and
[ADR-0023](../adr/0023-pending-signed-namespace-successors.md). Historical
research dossiers were retired after their decisions and behavior were
promoted into ADRs and tests.
