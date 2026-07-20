# Fault Injection And Parameter Perturbation

## Суть

Этот документ описывает активное тестирование, где инженер не меняет
semantic code path напрямую, а вмешивается в operational conditions:

- параметры конфигурации;
- лимиты;
- задержки;
- порядок событий;
- availability отдельных runtime dependencies;
- объемы данных и скорость потока.

Это промежуточный класс между mutation testing и chaos testing:

- mutation ломает кодовую логику;
- chaos ломает runtime-среду целиком или ее части;
- perturbation меняет operational envelope и пороговые условия.

## Когда использовать

Метод обязателен, когда слабое место вероятнее всего связано с:

- threshold logic;
- timeout/retry tuning;
- recovery windows;
- admission limits;
- retention limits;
- batching, buffering и queue depth;
- startup ordering;
- publication timing relative to runtime truth.

## Методики проверки

### Configuration Boundary Sweep

Проверка минимальных, типовых и предельных значений конфигурации.

Цель:

- найти unsafe defaults;
- выявить silent acceptance недопустимых режимов;
- проверить explainability validation failures.

### Timing Perturbation

Изменение latency, ordering, wait windows, deadlines, freshness intervals.

Цель:

- поймать race-like деградации;
- проверить устойчивость recovery/retry logic;
- выявить flaky assumptions в tests и runtime.

### Volume And Pressure Sweep

Изменение payload size, queue depth, retention pressure, number of concurrent operations.

Цель:

- проверить backpressure и bounded behavior;
- убедиться, что система не теряет explainability под нагрузкой;
- выявить места, где resource pressure дает ложный success signal.

### Partial Availability Injection

Частичная недоступность dependency, endpoint family, storage operation,
publication path или discovery source.

Цель:

- проверить partial-failure semantics;
- убедиться, что degraded path различим от complete failure;
- проверить корректность operator-visible state.

## Expected Evidence

После perturbation experiment должны остаться:

- описание tested envelope;
- значения параметров или injected conditions;
- expected signal;
- actual signal;
- conclusion: robust, weak, inconclusive, blocked;
- required follow-up.

## Недопустимые практики

- менять параметры без фиксации tested range;
- считать "не упало" достаточным критерием успеха;
- делать perturbation, который нельзя повторить или откатить;
- оставлять findings без перевода в scenario/test/decision trail.
