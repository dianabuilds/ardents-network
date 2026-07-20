# Chaos Testing

## Суть

Chaos testing проверяет, сохраняет ли система explainable behavior при
неполных отказах, прерывании runtime flow, resource pressure и нарушении
ожидаемых условий среды.

Фокус chaos testing в Ardents:

- не просто "система упала или нет";
- а виден ли оператору реальный degraded state;
- сохраняется ли runtime truth;
- есть ли recovery path;
- переводятся ли проблемы в explainable diagnostics и formal regression gaps.

## Какие классы сбоев проверяются

- потеря или задержка connectivity;
- restart / kill / interruption процесса;
- resource starvation;
- частичная недоступность dependency;
- нарушение startup order;
- publication при деградированном runtime;
- stale state и recovery after interruption.

## Базовый процесс

1. Выбрать runtime flow или non-functional scenario.
2. Зафиксировать intended fault и blast radius.
3. Определить observable signals:
   health, diagnostics, pending operations, recovery status, publication truth.
4. Ввести fault на ограниченном участке.
5. Наблюдать canonical operator-visible surface.
6. Убрать fault и оценить recovery behavior.
7. Зафиксировать findings и перевести их в regression artifact.

## Методики проверки

### Connectivity Disruption

Временное отключение сети, relay path, bootstrap reachability или message flow.

Проверяет:

- видимость network degradation;
- корректность fallback/retry behavior;
- отсутствие ложного "ready", если transport truth потеряна.

### Process Lifecycle Chaos

Kill, forced restart, interruption during startup/shutdown/recovery.

Проверяет:

- restart predictability;
- pending operations fate;
- explainability после прерывания;
- отсутствие silent corruption в runtime truth.

### Resource Pressure

Ограничение CPU, memory, disk, I/O, queue capacity или worker budget.

Проверяет:

- controlled degradation вместо opaque failure;
- сохранение diagnostics visibility;
- корректность admission/backpressure behavior.

### Dependency Degradation

Замедление, временная недоступность или частичный отказ внешней/внутренней
зависимости в рамках реального runtime path.

Проверяет:

- насколько локализован blast radius;
- есть ли operator-visible explanation;
- не продолжает ли система публиковать ложную readiness truth.

## Критерии качества

Chaos experiment считается корректным только если:

- fault bounded и обратим;
- проверяется реальная operator-visible truth;
- результат можно воспроизвести хотя бы на уровне сценария;
- findings конвертируются в canonical non-functional coverage.

## Недопустимые практики

- хаос без rollback path;
- fault injection, который разрушает test environment без возможности интерпретации результата;
- проверка только логов без проверки health, diagnostics и runtime truth;
- chaos как разовое шоу без последующего regression follow-up.
