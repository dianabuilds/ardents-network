# Authority Custody

Custody owns encrypted purpose-specific Authority records, original exclusive
operations, secret erasure, encrypted successors and monotonic recovery floors.
The real consumer is `cmd/ardents-custody`: Admission Authority creation and
Permission issuance, public inspection, authenticated verification, Recovery
Bundle export/test restore, quarantine restore and explicitly confirmed purge.

Admission's `allocation.Prepare` and `Request.Decide` alone validate holder proof,
Network/Authority/hour, finite quota, journal transition and exact retry. Custody
accepts no external Decision or private root. It signs that exact internal
purpose-specific decision, persists and reads back its encrypted successor,
advances the floor, then returns the public Permission. An output refusal never
refunds allocation. Admission retry has no post-expiry exception.

The fixed `ardents-authority-envelope-v1` Argon2id/AES-GCM grammar and existing
floors remain compatibility obligations. Only independently born development
roots receive Admission signing provenance; existing records/floors without
that provenance cannot acquire it. This is no predecessor migration/adoption.
Generic Service and retired Name records remain encrypted evidence, with no
new signing operation. Restored records remain in separate quarantine and can
be verified, exported and purged, never activated or signed. Purge retains floors.

One original operation serializes terminal and durable effects. Close denies
new effects synchronously, cancels the original user, joins actual work, and
returns its retained failure after secret erasure and native lease release.
State locks never cover foreign terminal I/O or physical join. The accepted
single authenticated pending successor can finish its floor on a fresh reopen;
malformed, missing or skipped state cannot reconstruct fresh allocation history.

`doc.go`, implementation responsibilities and behavior tests form one cohesive
package. Canonical envelope encoding, floor grammar and native file/lease
mechanisms were inspected from the current `internal/custody` source and adapted
without imports or backend delegation. Admission policy and ownership use the
new Allocation owner. `make custody-check` selects actual Linux race/filesystem/
PTY and official Argon2 vectors; native Windows process checks are development
evidence too. Neither establishes platform permissions, crash/power-loss,
weakest-host performance or independent security qualification.

Service Credential issuance remains separately with the current predecessor
command path. State/Node/Release signing, runtime Instance keys, Publication,
Connection, Execution, Route and Installation remain outside this package.
Future Service issuance requires its separately accepted pure public seam;
this owner never imports live Publication/Instance merely for a parser.

[Technical contract](../../../docs/technical/release-update-custody.md),
[domain map](../../../docs/development/domain-map.md), and
[package map](../../../docs/development/package-map.md) own the current boundaries.
GitHub owns readiness, acceptance and integration status.
