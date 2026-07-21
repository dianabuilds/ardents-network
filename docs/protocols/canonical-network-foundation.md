# Canonical Network Foundation

## 1. Назначение

Этот документ фиксирует единственное допустимое решение по сетевой основе продукта Ardents для `v1`.

Его задача:

- убрать двусмысленность вокруг сетевого substrate;
- не дать разработке снова уйти в произвольную замену несущего механизма;
- зафиксировать, на чём сеть живёт как сеть;
- закрыть вопрос, который нельзя оставлять "на потом".

## 2. Решение Для `v1`

Для `v1` canonical network foundation Ardents - это `Waku`.

Это означает:

- сеть Ardents для `v1` строится поверх `Waku`;
- messaging опирается на `Waku`;
- discovery и service/data announce не могут проектироваться в отрыве от `Waku`;
- любая альтернативная реализация не считается допустимой для `v1`, пока не доказана как полный эквивалент по системной роли.

## 3. Почему Решение Зафиксировано Именно Так

По `aim-core` видно, что:

- `Waku` не был случайным adapter;
- `Waku` был главным сетевым механизмом;
- без него референс переставал быть полноценной сетью;
- discovery, messaging и transport life в референсе держались именно на этой основе.

Следовательно:

- игнорировать `Waku` как несущий reference invariant нельзя;
- заменять его "потом" тоже нельзя;
- оставлять вопрос открытым на стадии разработки продукта недопустимо.

## 4. Что Разрешено И Что Запрещено

### 4.1 Разрешено

- использовать `Waku` как канонический substrate;
- строить product semantics поверх него;
- строить privacy/orchestration layer поверх `Waku`, если он не подменяет
  каноническую foundation, а управляет participation profiles, exposure
  reduction и explainable degraded behavior;
- развивать transport participation variants и privacy semantics как отдельные
  инженерные tracks поверх той же `Waku` foundation, если они не смешиваются в
  новый неявный substrate;
- изолировать интеграцию с `Waku` в адаптерах и runtime boundaries;
- поддерживать несколько transport participation variants внутри `Waku`-backed
  foundation, если они остаются вариантами конфигурации и реализации одной и
  той же канонической сетевой основы;
- улучшать архитектурную форму относительно `aim-core`.

### 4.2 Запрещено

- подменять `Waku` локальными эвристиками;
- подменять `Waku` абстрактным "transport layer" без явного equivalence proof;
- разрабатывать network plane так, будто выбор substrate ещё не сделан;
- писать transport/discovery/data announce логики в нейтральной форме, которая фактически исключает `Waku`.
- строить "privacy layer", который фактически скрывает сам факт того, что
  продукт остаётся `Waku`-backed, и превращается в новую неявную network
  foundation.

### 4.3 Transport Variants Inside Canonical Foundation

`v1` may evolve multiple transport variants, but only under the following
rules:

- transport variants are runtime/configuration variants of the same
  `Waku`-backed network foundation;
- transport variants must preserve the required product roles of
  `relay`, `store`, `filter`, and `lightpush`;
- transport variants must not become a competing product abstraction that hides
  whether the system is still operating on `Waku`;
- transport variants must be validated against the same discovery, messaging,
  publication, diagnostics, and operator-visible invariants as the default
  transport path;
- transport variants may disable or prefer specific underlying transports, but
  they may not redefine the canonical network foundation of `v1`.

## 5. Что Это Значит Для Других Доменов

### 5.1 Network / Messaging

Сетевая плоскость обязана проектироваться как `Waku`-backed.

### 5.2 Discovery

Discovery обязано учитывать, что сеть существует поверх `Waku`, а не поверх произвольных endpoint hints.

### 5.3 Data Substrate

Data Substrate не равен `Waku`, но его announce, retrieval coordination и доступность не должны проектироваться так, будто сетевой основы нет.

### 5.4 Hosted Services

Hosted service publication и reachability должны рассматриваться в контексте `Waku`-based network participation.

## 6. Условие Пересмотра

Это решение может быть пересмотрено только если появится отдельный документ уровня не ниже этого, который доказывает:

- полный системный эквивалент новой основы роли `Waku`;
- сохранение discovery, messaging, publication и network participation без деградации в local-only модель;
- отсутствие потери reference invariants из `aim-core`.

До появления такого документа решение считается закрытым.

## 7. Mandatory Waku Capability Set For `v1`

For `v1`, Ardents must treat the following Waku capabilities as part of the
canonical network foundation:

- `relay`
- `store`
- `filter`
- `lightpush`

This is the default and target capability set for all product-grade network
work. It may be constrained by node configuration, but it may not be removed
from the product model.

## 8. Configuration Limits

The node may expose configuration that effectively disables storage retention,
including:

- `retention_ttl = 0`
- `retention_bytes = 0`

Such configuration disables local retention behavior for that node, but it does
not change the required Waku role mapping of the product.
