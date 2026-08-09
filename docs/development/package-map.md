# Package map

This is the factual naming and dependency registry for maintained Go code. It
lists packages that exist, not future intentions. A new entry is an architecture
change: name the responsibility first, add the real Implementation and tests,
then update this table in the same change. The normative growth and dependency
rules are in [repository-layout.md](repository-layout.md).

| Directory | Go declaration | Responsibility | May import |
|---|---|---|---|
| `cmd/carrier-lab` | `package main` | Parse fixed Carrier Lab commands, call the selected Module, and translate results to exit codes. | `internal/directcontrol`, `internal/harness`, `internal/preflight`, standard library |
| `internal/harness` | `package harness` | Own fixed Carrier Lab scenarios: fixtures, data-only role configuration, child/Compose lifecycle, bounded observations, fault injection, verdicts, and cleanup. | `internal/preflight`, standard library |
| `internal/preflight` | `package preflight` | Orchestrate pinned setup, verify the environment, protect one run identity and its owned paths, and write canonical preflight evidence and cleanup verdicts. | standard library |
| `internal/directcontrol` | `package directcontrol` | Execute the lab-only Direct TLS measurement roles and protected-record fault; never act as a Route or fallback. | standard library |
| `internal/architecture` | `package architecture` | Test repository structure, names, dependencies, formatting, and quality wiring. | standard library |

## Carrier Lab command registry

| Command | Owning Module | Fixed responsibility |
|---|---|---|
| `bootstrap` | `internal/preflight` | Orchestrate pinned host/container setup while reserving canonical evidence and verdicts for the pinned verifier. |
| `evaluate` | `internal/preflight` | Verify prepared pinned inputs and write intermediate preflight evidence. |
| `finalize-cleanup` | `internal/preflight` | Verify owned-resource removal and publish the final preflight verdict. |
| `compose-smoke` | `internal/harness` | Run the fixed two-role isolated scenario and own its lifecycle and summary. |
| `smoke-role` | `internal/harness` | Run one data-only role whose config names only its allowed peer. |
| `direct-control` | `internal/harness` | Generate one ephemeral Target/Instance fixture and run the positive and fixed Direct TLS negative cases. |
| `direct-role` | `internal/directcontrol` | Run one User or Service tracer role for the fixed Direct TLS control. |
| `direct-tamper` | `internal/directcontrol` | Modify one TLS-protected record without receiving endpoint or Application knowledge. |

The architecture gate parses this table and rejects an unregistered or stale
package, a mismatched Go declaration, or a current project import absent from
the owning row's `May import` column.
