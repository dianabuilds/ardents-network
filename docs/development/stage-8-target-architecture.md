# Stage 8 remaining architecture boundaries

Status: **active temporary control.** The current package graph is owned by the
[package map](package-map.md); behavior and limitations are owned by the
technical documents. This file keeps only the architectural constraints that
remain relevant while Stage 8 has active M12/M14 work. It is deleted with the
refactoring plan when those controls are no longer needed.

## Maintained shape

- Thin commands in `cmd/` adapt owner-scoped input to deep `internal/` modules.
  They do not own a durable domain root, protocol selection, or broad plan
  abstraction.
- `internal/naming/namespace` is a composition root only for opaque Resolution
  Gateway/verifier views. `admission`, `record`, `claim`, `recovery`, `epoch`,
  and `authority` own their concrete state and transitions; lower modules do
  not import the Authority orchestrator or Epoch.
- `internal/naming/resolution` receives the root opaque views but concrete
  Namespace facts from the smallest nested module that owns them.
- `internal/network/state` is the accepted current/pending View writer;
  `network/source` is its acquisition port, and `network/duty` is the separate
  durable role-domain owner.
- `internal/endpoint` composes Broker, Publication, Service Connection, and
  opaque Route Attachment inputs. Publication owns Instance material;
  Service Connection owns stream/recovery state; Route owns neither State nor
  Service Connection recovery.
- `internal/custody` is the only Authority-root holder. Release, Update,
  Endpoint, and diagnostics never receive a root, password, or generic signing
  capability.

The checked [package map](package-map.md) is authoritative for exact imports.
The layout has no generic compatibility façade, `planfile`, `serviceconn`,
`applicationipc`, `serviceendpoint`, `routeplan`, `node/probe`, Bridge,
Camouflage, or laboratory runtime package.

## Completed boundaries and honest limits

State, Duty, Namespace, Resolution, Entry, Route, Publication, Service
Connection, Broker/Endpoint, Node, Release, Update, and command-consolidation
ownership transfers are complete for Stage 8. Their current contracts are in:

- [Private naming and Namespace](../technical/naming.md)
- [Endpoint and Service runtime](../technical/endpoint-service-runtime.md)
- [Network State, Entry, Route, and Node](../technical/network-route-node.md)
- [Release, Update, and Authority Custody](../technical/release-update-custody.md)
- [Current command reference](../reference/commands.md)

The selected native Route and its mixed closed-network run are functional
closed-test-network evidence only. They do not establish a peer-facing Route
runtime, public deployment, privacy, independent operation, State/Entry
integration, Service Connection qualification, or a supported Node profile.
R-092 is the open future Node-profile measurement; it is not a Stage 8
capacity blocker.

## Active stop conditions

M12 may not select a supported Windows/Ubuntu storage, permissions, crash,
power-loss, isolation, install, or complete Custody operator profile without a
new accepted product/platform decision and ADR analysis. It also may not add
a local Vault-demotion transition without a selected Name-scoped
predecessor-to-successor proof. The decision route is DA-08/DA-09 in the
[decision-authority register](stage-8-decision-authority-register.md).

M14 may remove only Stage material whose unique current fact is already owned
by a technical/reference/ADR document or whose provenance is recoverable from
Git. A newly discovered laboratory or evidence artifact with a reproduction or
Qualification duty needs the DA-11 disposition first.
