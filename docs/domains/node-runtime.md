# Домен Node Runtime

## Назначение

`Node Runtime` отвечает за узел как за управляемую runtime-систему.

В варианте `C` этот домен намеренно сужен. Он больше не является контейнером для
общего runtime wiring, read-side фасадов, publication coordination и local
control surface mechanics. Он владеет только node-level truth.

## Границы Ответственности

Домен отвечает за:

- lifecycle узла;
- boot и recovery progression узла;
- node-level readiness;
- node-level degraded и failed semantics;
- lightweight persisted node facts, необходимые для continuity узла через restart.

Домен не отвечает за:

- identity semantics;
- discovery ownership;
- transport ownership;
- hosted-service publication ownership;
- local control surface как отдельную поверхность;
- runtime assembly и process wiring;
- query/read-model фасады для boundary слоя.

## Authoritative Truth

- lifecycle state узла;
- startup, shutdown и recovery phases;
- boot participation summary на уровне узла;
- node readiness outcome;
- node degraded и failed reasons;
- node-owned persisted continuity facts.

## Не-Цели

- быть god-package для всей локальной runtime-сборки;
- владеть workflow соседних доменов;
- содержать application orchestration как часть доменной truth;
- содержать projection/read-model слой как часть продуктового домена;
- подменять собой canonical local control surface.

## Ключевые Инварианты

- у узла один authoritative lifecycle state;
- узел не может считаться `ready`, пока обязательные доменные зависимости не достигли
  требуемого observed state;
- node degraded/failure semantics должны быть explainable;
- orchestration соседних доменов не делает `Node Runtime` владельцем их truth;
- persisted node facts должны быть минимальными и относиться только к continuity узла.

## Входные Контракты

- `StartNodeLifecycle`
- `StopNodeLifecycle`
- `RecoverNodeLifecycle`
- `GetNodeRuntimeSnapshot`
- `ListNodePendingOperations`

## Команды И Запросы

### Commands

- `StartNodeLifecycle`
  Запускает lifecycle узла и переводит его в startup progression.
- `StopNodeLifecycle`
  Выполняет node-owned shutdown progression.
- `RecoverNodeLifecycle`
  Запускает node-owned recovery progression после restart или interrupted startup.

### Queries

- `GetNodeRuntimeSnapshot`
  Возвращает authoritative node runtime snapshot.
- `ListNodePendingOperations`
  Возвращает node-owned pending operations и их текущую fate.

## Выходные Контракты

- node lifecycle transitions для runtime assembly;
- node runtime snapshot для canonical local control surface;
- node lifecycle/readiness facts для diagnostics и operator-visible surfaces;
- node lifecycle gates для соседних доменов, которым нужен runtime readiness signal.

## Authoritative Results

- `NodeStartResult`
- `NodeStopResult`
- `NodeRecoveryResult`
- `NodeRuntimeSnapshot`
- `NodePendingOperationsSnapshot`

## Публикуемые События И State Outputs

### Domain Events

- `NodeStarting`
- `NodeReady`
- `NodeDegraded`
- `NodeFailed`
- `NodeStopping`
- `NodeStopped`
- `NodeRecoveryStarted`
- `NodeRecoveryCompleted`

### State Outputs

- `NodeRuntimeSnapshot`
  Потребители: runtime assembly, canonical local control surface, diagnostics.
- `NodeLifecyclePhase`
  Потребители: runtime assembly и домены, которым нужен lifecycle gate.

## Фасад Домена

Фасад должен предоставлять:

- start lifecycle;
- stop lifecycle;
- recover lifecycle;
- runtime snapshot;
- pending operations.

Фасад не должен раскрывать:

- process wiring;
- goroutine supervision;
- boundary transport details;
- cross-domain orchestration internals;
- compatibility read models.

## Привязка К Boundary И Runtime Слоям

- canonical local control surface не является отдельным продуктовым доменом;
  она читает node-owned truth через `internal/node/api` и API других доменов;
- runtime assembly не является продуктовым доменом;
  она координирует `Node Runtime` и соседние домены, но не владеет их truth;
- текущие non-domain runtime layers живут в `internal/runtime/{assembly,authority,orchestration,process}`
  и не должны возвращаться внутрь `internal/node` как hidden owner-пакеты;
- boundary transports не зависят от внутренних subpackages `internal/node/*`
  сверх `internal/node/api`.

## Целевая Структура Каталогов

```text
internal/node/
  api/
  lifecycle/
  readiness/
  recovery/
```

## Правила Внутренней Структуры

- `internal/node/api` является единственным публичным контрактом домена;
- `lifecycle`, `readiness`, `recovery` содержат только node-owned truth;
- внутри `internal/node` не должны жить `process`, `projection`, `authority`,
  `publication` как постоянные owner-пакеты;
- runtime assembly, command/query adaptation и boundary wiring живут вне product-domain
  каталога `internal/node`.

## Правила Зависимостей

- `internal/node` может зависеть от API соседних доменов для вычисления node-level
  readiness;
- `internal/node` не забирает vocabulary и truth соседних доменов;
- boundary transports и local control adapters зависят только от `internal/node/api`.

## QA Этапы

Unit:

- lifecycle transitions;
- readiness/degraded/failed transitions;
- recovery progression;
- запрет ложного `ready`.

Integration:

- node lifecycle корректно координируется с identity, network, discovery, workload,
  publication, data и diagnostics;
- broken dependency не приводит к ложному `ready`.

E2E:

- оператор может запустить, остановить, восстановить и объяснить состояние узла через
  canonical local control surface.

## Правила Миграции

## Актуальное Состояние

`Node Runtime` уже владеет:

- lifecycle ownership;
- recovery ownership;
- node continuity facts.

В домен не должны возвращаться:

- `internal/node/process` как owner runtime facade;
- `internal/node/projection` как постоянный read-side слой внутри домена;
- `internal/node/authority` как write-side owner чужих доменов;
- `internal/node/publication` как смешанная ownership-зона.
