# Development documentation

This directory owns engineering policy and factual architecture maps. Start
from the [current documentation route](../README.md) and the affected technical
owner. GitHub owns task selection, implementation progress and acceptance.

## Engineering route

- [Documentation ownership](documentation.md) and [contributing](../../CONTRIBUTING.md).
- [Domain map](domain-map.md): exclusive rules, state, resources and collaborations;
  [interaction contract](domain-map.md#inter-domain-interaction-contract) and
  [interface checklist](domain-map.md#interface-checklist-for-development-plans)
  for preparing cross-domain work.
- [Package map](package-map.md) and [repository layout](repository-layout.md): actual packages and exact imports.
- [Agent execution](agent-execution.md) and [Route migration contract](route-migration-contract.md): authorized execution, isolation and handoff.
- [Testing](testing.md), [privacy acceptance](privacy-qualification.md) and [deep audit](deep-audit.md): evidence profiles and candidate qualification boundaries.
- [Dependencies](dependencies.md) and [scoped risk exceptions](scoped-risk-exceptions.md).
- [Command surface](command-surface.md), [command reference](../reference/commands.md) and [local diagnostics](local-diagnostics.md).
- [Privacy coverage and dependency map](privacy-anonymity-map.md).
- [Route integration boundaries](route-delivery-join-boundary.md): retained cross-domain transport and acceptance obligations.

## Historical material

The [pinned archive index](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/README.md)
lists former decomposition proposals, source inventories, alternative
network-core/wire candidates and immutable audit receipts, retained in Git
history. The current domain map replaces the original strategic proposal; the
technical owners replace completed component design snapshots. An unresolved
finding remains unresolved after retirement. A receipt qualifies only its
exact candidate and stated boundary, never the current tree by inheritance.
