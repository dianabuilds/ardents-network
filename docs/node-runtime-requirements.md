# Node Runtime Requirements

## 0. Role In Docs Set

This is a supporting deep-dive for `Node Runtime`, `Runtime Assembly`, and the
canonical local control surface.

It does not override:

- `system-concept.md`
- `system-frame.md`
- `system-properties.md`
- `development-contract.md`
- `docs/domains/node-runtime.md`

Key, retained-state, and backup/restore behavior is additionally constrained by
`persistent-state-security.md`.

If this document drifts from the core or domain documents, the core and domain
documents win first and this file must be aligned afterward.

## 1. Назначение

Этот документ фиксирует требования к:

- `Node Runtime`
- `Local Control Surface`
- `Runtime Assembly`

В варианте `C` эти вещи больше не считаются одной широкой product-плоскостью.

- `Node Runtime` остается продуктовым доменом.
- `Local Control Surface` остается канонической поверхностью управления, но не отдельным доменом.
- `Runtime Assembly` остается application/runtime concern, но не продуктовым доменом.

## 2. Node Runtime

`Node Runtime` отвечает за:

- lifecycle;
- startup;
- shutdown;
- recovery after restart;
- node-level readiness;
- node-level degraded/failed semantics;
- authoritative local node continuity state;
- итоговую node status картину.

`Node Runtime` не должен:

- становиться runtime facade для всей логики продукта;
- забирать domain logic соседних доменов;
- владеть publication truth;
- владеть local control transport mechanics;
- владеть query/read-model слоем boundary-поверхности.

## 3. Runtime Assembly

`Runtime Assembly` отвечает за:

- wiring доменов в один рабочий узел;
- ordered start/stop orchestration across domains;
- command/query adaptation для canonical local control surface;
- integration-level runtime flows.

`Runtime Assembly` не является product domain.

Она не должна:

- становиться владельцем lifecycle truth;
- становиться владельцем publication truth;
- порождать новую параллельную control surface vocabulary;
- жить внутри `internal/node` как скрытый domain owner.

## 4. Local Control Surface

У продукта должна быть одна каноническая локальная поверхность управления.

Она обязана:

- принимать команды;
- отдавать queries;
- выдавать snapshots;
- выдавать events;
- объяснять состояние системы оператору и локальным клиентам;
- опираться на доменные API, а не на внутренние subpackages.

Она не должна:

- содержать product logic;
- владеть собственной domain truth;
- обходить доменные API и обращаться к внутренним пакетам напрямую;
- становиться отдельным продуктовым доменом.

## 5. Node Runtime Minimal Ownership

К минимальному обязательному ownership `Node Runtime` относятся:

- lifecycle state machine and transitions;
- startup, shutdown and recovery progression;
- operator-visible boot/join summary;
- authoritative local node status;
- lightweight persisted node facts, нужные для continuity across restart.

Эти concerns не должны жить как отдельные top-level product packages вроде generic
`process`, `runtime`, `boot` или `state`.

## 6. Publication Clarification

Hosted-service specification и service readiness не принадлежат `Node Runtime`.

Local presence publication, local service publication outcome и withdraw/refresh semantics
также не принадлежат `Node Runtime`.

Для варианта `C` publication фиксируется как отдельный продуктовый домен, который:

- потребляет node readiness;
- потребляет hosted-service readiness;
- использует discovery record model;
- использует transport reality;
- владеет publication truth.

## 7. Diagnostics

Diagnostics обязаны быть explainability layer всей системы.

Они должны включать:

- health summary;
- primary reason;
- subsystem reasons;
- recent events;
- pending operations;
- operator-facing explanation degraded и failed состояний.

Diagnostics не должны быть просто string log collector.

## 8. Policy

`Policy` должен реально влиять как минимум на:

- workload admission;
- publication;
- data retention;
- network participation;
- допустимость отдельных действий local API.

Policy не должен становиться giant generic policy engine.

## 9. Minimal Implementation Criteria

Плоскость нельзя считать реализованной, пока одновременно не выполняются условия:

- узел реально стартует, останавливается и восстанавливается;
- node lifecycle truth канонична и explainable;
- local control surface канонична;
- runtime assembly не выдает себя за продуктовый домен;
- publication truth не спрятана внутри `Node Runtime`;
- diagnostics объясняют состояние системы;
- pending operations сохраняются и объяснимы;
- policy реально влияет на поведение продукта.
