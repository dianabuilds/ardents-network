# System Frame

## 1. Назначение

Этот документ задает технический каркас Ardents как системы.

Он отвечает на вопросы:

- из каких продуктовых доменов состоит продукт;
- какие задачи решает каждый домен;
- какие границы между ними должны существовать;
- какие слои являются product domains, а какие нет;
- что считается нормальной формой системы для `v1`.

## 2. Базовая Форма Системы

Ardents состоит из одного управляемого узла, который одновременно является:

- локальной runtime-системой;
- участником сети;
- носителем identity и trust context;
- источником и потребителем discovery knowledge;
- хостом для workloads и services;
- держателем локальных objects и blobs;
- наблюдаемой системой с diagnostics и canonical local control surface.

Но это не означает, что вся эта форма должна жить внутри одного широкого домена.

Для `v1` действует вариант `C`:

- product domains владеют только своей truth;
- runtime assembly не является product domain;
- canonical local control surface не является product domain;
- query/read-model и orchestration слои не должны притворяться продуктовым ownership.

## 3. Главные Плоскости Системы

### 3.1 Node Runtime Plane

Это жизнь узла как управляемой системы.

Сюда входят:

- lifecycle;
- startup/shutdown/recovery;
- node-level readiness;
- node-level degraded/failed semantics;
- authoritative local node continuity state.

### 3.2 Network Participation Plane

Это жизнь узла как участника сети.

Сюда входят:

- network foundation;
- transport presence;
- discovery exchange;
- publication of local presence and services;
- trust-aware use of remote knowledge;
- route usability.

### 3.3 Data Plane

Это жизнь payload-данных, вложений и blob-данных.

Сюда входят:

- blob storage;
- integrity;
- manifests and references;
- fetch/publication rules;
- retention/pinning/cache semantics.

## 4. Домены Системы

Ниже зафиксирован обязательный набор продуктовых доменов `v1`.

### 4.1 Node Runtime

Роль:

`Node Runtime` владеет только node-level truth.

Задачи:

- запускать и останавливать узел;
- восстанавливать node-level state после restart;
- владеть lifecycle и readiness semantics;
- собирать итоговый node status из результатов соседних доменов.

Не должен:

- становиться generic runtime framework;
- становиться владельцем publication truth;
- поглощать query/read-model и runtime assembly.

### 4.2 Identity

Роль:

`Identity` задает устойчивую identity узла и связанных субъектов.

### 4.3 Network Foundation / Messaging

Роль:

`Network Foundation / Messaging` дает реальную сетевую основу поверх `Waku`.

### 4.4 Discovery

Роль:

`Discovery` владеет знанием о существующих узлах и services, а также trust-aware resolution.

Не должен:

- владеть publication truth локального узла;
- становиться local-only catalog.

### 4.5 Publication

Роль:

`Publication` владеет локальной публикацией node presence и hosted services в сеть.

Задачи:

- формировать publication intent;
- принимать readiness inputs из `Node Runtime` и `Hosted Services`;
- учитывать policy и transport/network capability;
- публиковать, обновлять и снимать локальное presence/service state;
- делать publication outcome explainable.

Не должен:

- подменять собой `Discovery`;
- владеть hosted-service model;
- владеть network substrate.

### 4.6 Workload Control

Роль:

`Workload Control` владеет локальным исполнением и контролем workloads.

### 4.7 Hosted Services

Роль:

`Hosted Services` владеет service model, readiness и exposure eligibility.

Не должен:

- владеть publication outcome;
- публиковать service без runtime backing.

### 4.8 Data Substrate

Роль:

`Data Substrate` владеет удержанием, хранением, доставкой и повторной отдачей payload и blob-данных.

### 4.9 Policy

Роль:

`Policy` владеет правилами, меняющими runtime behavior продукта.

### 4.10 Diagnostics

Роль:

`Diagnostics` владеет explainability runtime-состояния системы.

## 5. Не-Доменные, Но Обязательные Слои

### 5.1 Runtime Assembly

`Runtime Assembly` существует, чтобы собирать домены в один рабочий узел.

Это не product domain.

Он может:

- делать wiring;
- координировать ordered start/stop;
- адаптировать command/query flows.

Он не может:

- становиться owner product truth;
- становиться новой runtime facade архитектуры;
- жить внутри `internal/<domain>` как скрытый владелец всей системы.

### 5.2 Local Control Surface

`Local Control Surface` остается канонической поверхностью управления.

Это не отдельный product domain.

Она должна:

- опираться на доменные API;
- показывать operator-visible runtime truth;
- не создавать параллельную vocabulary ownership.

### 5.3 Boundary Transports

`Proto/Connect`, `HTTP`, `CLI` и другие transports остаются adapter-слоем.

Они не владеют product semantics.

## 6. Ownership Clarification

Ниже зафиксировано, кому принадлежит truth:

- lifecycle: `Node Runtime`
- boot participation summary: `Node Runtime`
- authoritative local node continuity state: `Node Runtime`
- identity semantics: `Identity`
- transport presence and messaging: `Network Foundation / Messaging`
- discovery knowledge and trust-aware resolution: `Discovery`
- local presence/service publication intent and outcome: `Publication`
- hosted-service spec, readiness, exposure eligibility: `Hosted Services`
- workload execution truth: `Workload Control`
- data retention and fetch truth: `Data Substrate`
- operator-visible explainability truth: `Diagnostics`
- behavior-changing rules: `Policy`

## 7. Architectural Consequences

- `Node Runtime` координирует систему, но не должен становиться владельцем всей логики.
- `Publication` больше не прячется внутри `Node Runtime`.
- `Hosted Services` и `Publication` связаны жестко, но это разные ownership зоны.
- `Discovery` больше не является скрытым владельцем publication policy.
- runtime assembly и local control surface остаются обязательными, но не становятся
  отдельными продуктами или доменами.

## 8. Недопустимые Формы

Система не должна вырождаться в:

- runtime facade как центр владения;
- blob-package с размазанным ownership;
- publication как helper-detail внутри `node` или `discovery`;
- query/projection layer как ложный продуктовый домен;
- local control surface как второй product domain.
