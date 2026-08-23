# Stage 8 remaining decision authorities

Status: **active temporary control.** All earlier Stage 8 decision routes have
either been implemented within their accepted scope or promoted to their
current ADR and technical owner. This register retains only decisions that can
still authorize new Stage 8 work. It is deleted with the refactoring plan.

| ID | Open decision | Required authority before implementation | Stop condition |
|---|---|---|---|
| DA-08 | Is a qualified Application isolation or supported operating profile in scope? | Product Owner scope, platform threat/Adapter design, representative evidence, and an ADR. | The `generic/unqualified` Broker is not a sandbox, process-host, Windows activation, or supported-host claim. |
| DA-09 | Is a supported release/update/Custody lifecycle in scope beyond the technical tracer? | Product Owner scope and lifecycle design, with ADR-0015/ADR-0021 compatibility analysis. | Do not turn tracer behavior or current command receipts into installation, activation, repair, or supported Custody behavior. |
| DA-11 | Does a newly discovered evidence or laboratory artifact have a reproduction or Qualification duty? | A named claim/source identity, retention condition, and Product Owner disposition. | Do not retain an unowned runner or delete an artifact that an accepted current claim still names. |

An open decision blocks only its dependent change. It does not authorize a
placeholder package, an unbounded compatibility adapter, or a weaker fallback.
