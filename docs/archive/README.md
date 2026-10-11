# Documentation archive

This is selected historical evidence, not a competing current specification or
work queue. Start with [current documentation](../README.md). The 26 local
records retain needed decision evidence, unique measurements, unresolved
findings or immutable audit receipts. Their bytes remain unchanged.
The [manifest](manifest.json) records original path, SHA-256, source revision,
disposition, current owner and reason for retaining each record.

The complete earlier archive of 168 records, including the 142 removed checkout
copies, is recoverable from the pinned Git snapshot: [index](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/README.md) and
[manifest](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/manifest.json). That manifest preserves every captured path, hash and
disposition. Historical source references now point to that exact revision;
removing a local copy does not rewrite Git history.

Archived accepted decisions retain explicitly surviving clauses. A later ADR
replaces only its stated scope: notably ADR-0117 changes Service byte identities
and old-state handling, preserving security/lifecycle and Terminal receipt rules.
An unaccepted/open proposal stays unaccepted/open. Retention or pruning closes
no unresolved finding and establishes no new candidate or qualification.
Current owners and GitHub hold today's contract and acceptance boundary.

## Reading historical links

Original texts keep their original relative links and source inventories. Use
the original source link in each table to follow those links in their original
repository layout; their former paths do not promise a current implementation.
Use the current-owner link for maintained work. External receipts retain their
declared locations and hashes, including original failures and fixture limits.

Hashes cover captured bytes, including encoding and line endings. The original
source revision identifies the earlier Git layout and context; a captured
checkout can have CRLF where the earlier Git blob used LF. Archive attributes
preserve captured bytes without conversion. The complete captured bytes are
available from the pinned archive snapshot above.

## Architecture decisions

| Record | Retention reason | Current owner | Original source |
|---|---|---|---|
| [0075-service-connection-v2-terminal-receipt.md](adr/0075-service-connection-v2-terminal-receipt.md) | Surviving Terminal receipt, confirmation and control-tail rules; ADR-0117 replaces byte identities only. | [endpoint-service-runtime.md](../technical/endpoint-service-runtime.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/adr/0075-service-connection-v2-terminal-receipt.md) |

## Research records

| Record | Retention reason | Current owner | Original source |
|---|---|---|---|
| [r-092-native-node-operating-profile.md](research/records/r-092-native-node-operating-profile.md) | Evidence for selected Node duties and operating limits. | [network-route-node.md](../technical/network-route-node.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-092-native-node-operating-profile.md) |
| [r-093-voluntary-endpoint-contribution.md](research/records/r-093-voluntary-endpoint-contribution.md) | Unresolved deferred co-resident Contributor question, criteria and limits. | [questions.md](../research/questions.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-093-voluntary-endpoint-contribution.md) |
| [r-094-hostile-network-carrier-profiles.md](research/records/r-094-hostile-network-carrier-profiles.md) | Unrepeated Carrier, MTU, NAT and loss measurements with their applicability limits. | [protected-route-protocol.md](../technical/protected-route-protocol.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-094-hostile-network-carrier-profiles.md) |
| [r-131-headless-publisher-authority-acquisition.md](research/records/r-131-headless-publisher-authority-acquisition.md) | Evidence for the current Service Authority/Instance separation under ADR-0064. | [endpoint-service-runtime.md](../technical/endpoint-service-runtime.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-131-headless-publisher-authority-acquisition.md) |
| [r-132-headless-publication-readiness.md](research/records/r-132-headless-publication-readiness.md) | Evidence for current Publication readiness, ACK and refresh boundaries under ADR-0065. | [endpoint-service-runtime.md](../technical/endpoint-service-runtime.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-132-headless-publication-readiness.md) |
| [r-137-c0-stress-test.md](research/records/r-137-c0-stress-test.md) | Incomplete stress result and unresolved causal uncertainty; no successor qualification exists. | [questions.md](../research/questions.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-137-c0-stress-test.md) |
| [r-140-quic-go-v0-62-c0-requalification.md](research/records/r-140-quic-go-v0-62-c0-requalification.md) | Unresolved exact-version external QUIC qualification retained by the dependency owner. | [dependencies.md](../development/dependencies.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-140-quic-go-v0-62-c0-requalification.md) |
| [r-146-non-administrative-network-consensus.md](research/records/r-146-non-administrative-network-consensus.md) | Rationale for the accepted public autonomy direction under ADR-0074. | [0074-target-non-administrative-public-operation.md](../adr/0074-target-non-administrative-public-operation.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-146-non-administrative-network-consensus.md) |
| [r-147-contributor-resource-controls.md](research/records/r-147-contributor-resource-controls.md) | Unresolved resource policy and co-resident contribution question. | [questions.md](../research/questions.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-147-contributor-resource-controls.md) |
| [r-148-shared-resource-contributor-benefits.md](research/records/r-148-shared-resource-contributor-benefits.md) | Unresolved incentives and future product question, without implementation selection. | [questions.md](../research/questions.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-148-shared-resource-contributor-benefits.md) |
| [r-149-autonomy-transition.md](research/records/r-149-autonomy-transition.md) | Unresolved public autonomy, influence, admission, finality and privacy gates. | [questions.md](../research/questions.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-149-autonomy-transition.md) |
| [r-149-voting-core-design.md](research/records/r-149-voting-core-design.md) | Unresolved B1-B5 architecture gates and evidence assessment for the future voting core. | [questions.md](../research/questions.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-149-voting-core-design.md) |
| [r-149-voting-work-packages.md](research/records/r-149-voting-work-packages.md) | Unique conditional V01-V13 handoff and selection blockers; remains draft-not-ready. | [questions.md](../research/questions.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-149-voting-work-packages.md) |
| [r-150-common-protection-baseline.md](research/records/r-150-common-protection-baseline.md) | Rationale for ADR-0077 and its source-matched dependency observation. | [0077-evolve-one-common-protection-baseline.md](../adr/0077-evolve-one-common-protection-baseline.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-150-common-protection-baseline.md) |
| [r-152-closed-scheme-contract.md](research/records/r-152-closed-scheme-contract.md) | Evidence for the selected closed Service scope and domain limits under ADR-0081. | [protected-route-protocol.md](../technical/protected-route-protocol.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-152-closed-scheme-contract.md) |
| [r-152-common-privacy-design.md](research/records/r-152-common-privacy-design.md) | Comparison and evidence behind the accepted split-circuit architecture under ADR-0078. | [protected-route-protocol.md](../technical/protected-route-protocol.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-152-common-privacy-design.md) |
| [r-153-bootstrap-child-restriction.md](research/records/r-153-bootstrap-child-restriction.md) | Evidence for the current authority-bearing bootstrap restriction under ADR-0082. | [protected-route-protocol.md](../technical/protected-route-protocol.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-153-bootstrap-child-restriction.md) |
| [r-154-join-data-lane-transition.md](research/records/r-154-join-data-lane-transition.md) | Evidence for the current JOIN result, data and credit transition under ADR-0083. | [protected-route-protocol.md](../technical/protected-route-protocol.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-154-join-data-lane-transition.md) |
| [r-168-service-v3-format-reset.md](research/records/r-168-service-v3-format-reset.md) | Exact reset/refusal scope and surviving clauses under ADR-0117. | [0117-reset-closed-service-formats-to-v3.md](../adr/0117-reset-closed-service-formats-to-v3.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-168-service-v3-format-reset.md) |
| [r-169-source-duty-lifetime.md](research/records/r-169-source-duty-lifetime.md) | Current Source guard lifetime and withdrawal evidence under ADR-0118. | [network-route-node.md](../technical/network-route-node.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-169-source-duty-lifetime.md) |
| [r-170-unified-protected-endpoint-artifact.md](research/records/r-170-unified-protected-endpoint-artifact.md) | Current exact-artifact, installation and initial-pin boundary under ADR-0119. | [enrollment-verification.md](../technical/enrollment-verification.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/research/records/r-170-unified-protected-endpoint-artifact.md) |

## Previous architecture and audit evidence

| Record | Retention reason | Current owner | Original source |
|---|---|---|---|
| [architecture-reconstruction-dev-merge-qualification.md](development/audit-receipts/architecture-reconstruction-dev-merge-qualification.md) | Immutable source/artifact hashes, failures, results and fixture limits for earlier reconstruction findings. | [deep-audit.md](../development/deep-audit.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/development/audit-receipts/architecture-reconstruction-dev-merge-qualification.md) |
| [c0-run-2-gate-a-remediation.md](development/audit-receipts/c0-run-2-gate-a-remediation.md) | Immutable remediation evidence and its original candidate/scope limits. | [deep-audit.md](../development/deep-audit.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/development/audit-receipts/c0-run-2-gate-a-remediation.md) |
| [c0-run-2-track-a.md](development/audit-receipts/c0-run-2-track-a.md) | Immutable qualification receipt, failures and candidate/scope limits. | [deep-audit.md](../development/deep-audit.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/development/audit-receipts/c0-run-2-track-a.md) |
| [repository-reconstruction-findings.md](development/repository-reconstruction-findings.md) | Finding dispositions still require reconciliation with the new owners; no newer closure is recorded. | [domain-map.md](../development/domain-map.md) | [source](https://github.com/dianabuilds/ardents-network/blob/17f98b669cea8c518c027a8528b77e2ec48fee44/docs/development/repository-reconstruction-findings.md) |

## Retired owner and product proposals

No retired owner snapshot or superseded product proposal needs a local archive
copy. Their exact editions, hashes and current-owner dispositions remain in the
[complete historical manifest](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/manifest.json).
They do not select work or restore retired product routes.
