---
id: R-166
title: Exact consumer and dependency closure of uncomposed Namespace resolution
status: proposed
owner: Product Owner
started: 2026-09-23
reviewed: 2026-09-23
---

# R-166 — Does the retained Namespace resolution module have a current consumer?

## Decision this unlocks

[#234](https://github.com/dianabuilds/ardents-network/issues/234) requires one accepted keep/remove disposition for internal/naming/resolution after the old Name network commands were retired. The result must preserve canonical Service Names, Namespace/custody roots and floors, the command's zero-effect refusal, and the separate current Target reachability path. This record proposes removal of the uncomposed module in one later implementation slice; it is not removal authorization by itself.

## Current authority and hypotheses

[ADR-0090](../../adr/0090-retire-name-operator-network-adapters.md) requires resolve/control command refusal before argument parsing or effects. It deliberately retains the resolution module **pending this exact-consumer decision**, while keeping canonical Name encoding, Namespace/custody and persisted evidence. The [Naming owner](../../technical/naming.md#operator-name-network-command-retirement) states that the module has maintained behavior but no production Gateway/Resolver composition. [ADR-0036](../../adr/0036-target-private-reachability-v1.md) keeps Target descriptor reachability separate from Namespace resolution; its historical statement that resolution stays Namespace-only does not create a current caller after ADR-0090. No successor Name authority, wire or public resolver is selected.

- **H1, keep as maintained runtime:** at least one current non-test caller or exact persisted/compatibility reader needs this module's accepting Gateway/Resolver/Control behavior.
- **H2, retire this one module:** only the retired-command test imports it externally; that test can retain its zero-effect oracle against authenticated State and committed Namespace roots without constructing the old OHTTP path.
- **H0, defer removal:** the fixture cannot preserve the accepted refusal oracle, or a current consumer/storage obligation is found.

Falsify H2 with a production import/call path, a module-owned durable format that a current reader must reopen, or a necessary zero-effect observation available only through its accepting old Gateway. Falsify H1 if its only external use is a test-local preflight for commands that now refuse before reading that input.

## Evaluation and primary evidence

Inspected the product/technical owners, accepted ADRs, package map, dependency register and Go source at dev@7e381928dd4bd906a1d623ca78a2dedf3738829c on 2026-09-23. The local checkout's affected Go files match that commit; its dependency register differs, so the dev register was read separately. Reproduce with a production-only search for internal/naming/resolution and resolution package calls under cmd and internal, then repeat including tests; inspect cmd/ardents/name.go and the three name_retirement* tests. A Windows Go 1.26.8 go-list comparison of cmd/ardents's normal and test dependency sets showed internal/naming/resolution only in the test set; go-list emitted a module-cache stat-write warning, so this is supporting evidence, not a clean build verdict.

**Sourced facts:**

1. cmd/ardents/name.go returns the fixed retirement error immediately for recognized resolve/control verbs. It imports only internal/naming for the encode path; it constructs no resolver, State view, HTTP client or Namespace operation.
2. There is no non-test external Go import or call into internal/naming/resolution in cmd or internal. The only external import is cmd/ardents/name_retirement_fixture_test.go. The 25-file module's own tests exercise its Client/Relay/Gateway/Control behavior, but module-only tests do not establish a product caller.
3. The retirement fixture creates authenticated State and committed Namespace roots, starts test HTTPS Gateway/Relay handlers, performs former resolve and control operations through internal/naming/resolution, writes old plan/operation JSON, and then checks that the retired command returns its exact error with zero output, zero default-transport dials and byte-for-byte unchanged root trees. Because dispatch refuses before remaining arguments, neither a live Gateway nor a formerly valid plan is needed to observe the command's accepted zero-effect result. The authenticated roots and before/after comparison **are** useful and should remain.
4. Production files in internal/naming/resolution do not open or write a local durable root. They consume State and Namespace views supplied by callers. Namespace epoch/materialization, authority transitions, record proofs, pending journal and floors belong to separate current packages, and custody still consumes their contracts. Deleting this module must neither delete nor rewrite those roots or accepted bytes.
5. OHTTP/CIRCL remains used outside this module: internal/service/reachability's current private Target descriptor path, internal/route/credential, and historical internal/naming/alpha/private still import OHTTP/CIRCL where applicable. The dependency register's representative cmd/ardents → naming/resolution path is a test-reachable projection, not proof of a production Name command. Removing the Name module alone does **not** justify removing openpcc/ohttp, twoway, bhttp, circl, or their go.mod/go.sum entries.
6. The current text Service's Target reachability is a different owner and opaque target operation. No Name-to-Target translation, DNS fallback, current Name Gateway, or independent operator has been selected. Removal makes no positive anonymity or availability claim: there is presently no ordinary private Name lookup to measure against a Relay, Gateway, observer or colluding operator.

**Limitation:** static import tracing and a dependency listing do not measure deployments holding historical roots; the accepted rule is to leave all roots and floors untouched. A future protected Name path may reuse ideas or algorithms, but cannot acquire authority from an uncomposed old test.

## Exact proposed removal and preserved oracle

Choose H2. After Product Owner acceptance, conditional [#272](https://github.com/dianabuilds/ardents-network/issues/272) is the one bounded Naming implementation card to delete the complete internal/naming/resolution directory, including its module-only tests, and remove only references that became false from the owning Naming, package-map, repository-layout and dependency documents. The historical ADR/research records remain provenance. No new package, accepting fake, successor OHTTP exchange or Name migration is introduced.

In the same buildable change, replace only the retirement test's dependency on the old module. Keep cmd/ardents/name_retirement_test.go's exact refusal string, former resolve/control shapes including missing/short arguments, empty output, zero network attempts, and byte-for-byte State/Namespace root comparison. Keep or reduce the State/Namespace fixture code necessary to construct and close those real roots. Remove test-local Gateway/Relay servers, old OHTTP profile, former resolve/control execution, obsolete operation/plan builders and any now-exclusive helper from name_retirement_fixture_test.go and name_retirement_state_fixture_test.go. A State or Namespace test helper may remain only with an actual caller and one cohesive responsibility; do not retain a synthetic old network stack solely to keep the refusal test elaborate. Existing canonical naming and Namespace/custody tests retain their own behavior oracles.

The code change must re-run the production/test import search, identify any newly revealed consumer before deletion, and keep go.mod/go.sum unchanged unless a separate exact last-use audit proves a dependency is no longer needed by **all** maintained build/test profiles. Do not remove internal/naming/alpha/private, internal/service/reachability, Route credential OHTTP, Namespace, custody or old persisted data as part of this card. A named future Name protocol/ordinary consumer belongs to N16/N17 after its own accepted contract, not to this cleanup.

## Verification and recommendation

Oracles for the later implementation: command refusal precedes parsing and all effects for valid-looking, malformed, missing and short former arguments; roots compare byte-for-byte before/after; transport attempt count remains zero; name encode vectors and Namespace/custody/proof/floor tests pass; actual Target reachability and OHTTP consumers still build and behave; production/test import scans show no resolution package; package map/dependency register describe the new graph; make quick-check and make check pass on the selected complete profile with no skipped prerequisite. A test-only old Gateway cannot substitute for a current Name consumer.

Recommend H2 with high confidence in the current caller graph and moderate confidence in fixture simplification until the actual replacement test is run. The strongest objection is loss of an already coded OHTTP Name implementation that a future design might reuse; keeping an uncomposed accepting runtime and its test-only dependency does not satisfy the selected future Name authority or ordinary path. If a current external consumer is found, stop deletion and name that consumer and its authority instead.

This record and queue row are **proposed**. ADR-0090's pending-removal state and the current Naming owner remain authoritative until the Product Owner accepts the choice and the owner is updated. No runtime or dependency change is made here.