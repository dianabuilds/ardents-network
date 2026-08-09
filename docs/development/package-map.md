# Package map

This is the factual naming and dependency registry for maintained Go code. It
lists packages that exist, not future intentions. A new entry is an architecture
change: name the responsibility first, add the real Implementation and tests,
then update this table in the same change. The normative growth and dependency
rules are in [repository-layout.md](repository-layout.md).

| Directory | Go declaration | Responsibility | May import |
|---|---|---|---|
| `cmd/carrier-lab` | `package main` | Parse fixed Carrier Lab commands, call the selected Module, and translate results to exit codes. | `internal/directcontrol`, `internal/harness`, `internal/harness/tooling`, `internal/preflight`, standard library |
| `internal/harness` | `package harness` | Own the fixed two-role Carrier Lab isolation scenario: role configuration, Compose lifecycle, bounded observations, fault injection, verdict, and cleanup. | `internal/preflight`, standard library |
| `internal/harness/tooling` | `package tooling` | Verify the exact external tool supply and own the shaping, capture, tracer, fail-closed smoke, and evidence lifecycle behind `VerifyInputs`, `RunSmoke`, and `RunRole`. | `internal/preflight`, `internal/qualification`, standard library |
| `internal/preflight` | `package preflight` | Orchestrate pinned setup, verify the environment, protect one run identity and its owned paths, and write canonical preflight evidence and cleanup verdicts. | `internal/qualification`, standard library |
| `internal/qualification` | `package qualification` | Bind maintained code, tests, Docker/Compose, Make, hooks, and CI inputs into one final Carrier Lab qualification source digest behind `SourceSHA256`. | standard library |
| `internal/directcontrol` | `package directcontrol` | Own the complete lab-only Direct TLS fixture, control lifecycle, roles, protected-record fault, evidence, and cleanup behind `RunControl`, `RunRole`, and `RunTamper`; never act as a Route or fallback. | `internal/preflight`, standard library |
| `internal/architecture` | `package architecture` | Test repository structure, names, dependencies, formatting, and quality wiring. | standard library |

## Carrier Lab command registry

| Command | Owning Module | Fixed responsibility |
|---|---|---|
| `bootstrap` | `internal/preflight` | Orchestrate pinned host/container setup while reserving canonical evidence and verdicts for the pinned verifier. |
| `evaluate` | `internal/preflight` | Verify prepared pinned inputs and write intermediate preflight evidence. |
| `finalize-cleanup` | `internal/preflight` | Verify owned-resource removal and publish the final preflight verdict. |
| `compose-smoke` | `internal/harness` | Run the fixed two-role isolated scenario and own its lifecycle and summary. |
| `smoke-role` | `internal/harness` | Run one data-only role whose config names only its allowed peer. |
| `tooling-verify` | `internal/harness/tooling` | Verify the exact external `.deb` set against the committed tool lock. |
| `tooling-smoke` | `internal/harness/tooling` | Bind the runnable image to base/lock/source/binary identity, then own real shaping/capture, exact topology evidence, fail-closed verdict, and cleanup. |
| `tooling-role` | `internal/harness/tooling` | Run one fixed synthetic tracer, shaper, or capture role. |
| `direct-control` | `internal/directcontrol` | Generate one ephemeral Target/Instance fixture and run the positive and fixed Direct TLS negative cases. |
| `direct-role` | `internal/directcontrol` | Run one User or Service tracer role for the fixed Direct TLS control. |
| `direct-tamper` | `internal/directcontrol` | Modify one TLS-protected record without receiving endpoint or Application knowledge. |

The architecture gate parses this table and rejects an unregistered or stale
package, a mismatched Go declaration, or a current project import absent from
the owning row's `May import` column.
