# Alpha control transition contract (retired)

Status: **retired by [ADR-0127](../adr/0127-retire-aca1-inspection-and-portable-enrollment-diagnosis.md)
(2026-11-19, #647).** This document formerly owned the project-controlled ACA1
control-transition disclosure contract consumed by `ardents-control
inspect-transitions`. The ACA1 reader, its inspection Modules, and the
`ardents-alpha-control-report-v1` / `ardents-alpha-transition-report-v1` report
schemas are removed; `inspect-bundle` and `inspect-transitions` return typed
before-effect refusals, and no unified replacement report is selected.

The authority separation this contract described remains binding in its
current owners:

- Release Safety belongs to the Release owner's own verification and floors;
- accepted Epoch successors and duty withdrawal belong to the Network State
  owner;
- the disclosed Compatibility cross-binding was an ACA1 disclosure artifact:
  the successor enforces Release/State coherence as a runtime invariant of its
  own owners, exercised behaviorally by the selected native harness, not by a
  static disclosure-consistency report;
- Namespace materialization remains absent (ADR-0105); no close, release,
  reclaim, or current Namespace state exists.

Inspection never authorized an Endpoint action, and its retirement waives no
protection or qualification case: acceptance outcomes come only from the
qualification harness bound to the exact candidate (ADR-0120;
[privacy qualification](../development/privacy-qualification.md)). Existing
ACA1 inspection floor roots and catalog/component bytes stay on disk
byte-for-byte as retained evidence with no maintained reader, converter, or
deleter (ADR-0124). The enrollment v3 companion inventory that pinned the
disclosure keys is unchanged grammar (ADR-0112/ADR-0119).

The complete former contract text, transition matrix, and verification record
remain in Git history at this path (last maintained revision before the
retirement change). ADR-0004, ADR-0006, ADR-0038, ADR-0043, ADR-0053, ADR-0054,
ADR-0110, and ADR-0127 govern the disposition; R-123 and F-49 remain historical
evidence. The retired `accept-alpha-corpus` / `inspect-alpha-corpus` refusals
and their retained-floor boundary are unchanged (ADR-0110, ADR-0113).