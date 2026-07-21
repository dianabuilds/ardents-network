# Workload, Services And Publication Requirements

## 0. Role In Docs Set

This is a supporting deep-dive for the interaction between:

- `Workload Control`
- `Hosted Services`
- `Publication`

It complements `system-properties.md` and `workload-execution-platform.md`.
The names below group product capabilities only; they do not prescribe packages,
modules, layers, or repository directories.

## 1. Назначение

Этот документ фиксирует требования к:

- `Workload Control`
- `Hosted Services`
- `Publication`

## 2. Workload Control

`Workload Control` отвечает за локальное исполнение и контроль workload.

Он обязан:

- принимать desired state;
- хранить observed state отдельно;
- выполнять admission;
- запускать и останавливать workload;
- восстанавливать workload after restart;
- поддерживать restart semantics;
- объяснять runtime outcome через diagnostics и local control surface.

`Workload Control` не должен:

- быть только registry конфигураций;
- считать declared intent фактом исполнения;
- публиковать service capability без runtime backing.

## 3. Hosted Services

`Hosted Services` описывает capability узла, доступные другим участникам сети.

Он обязан:

- владеть service model;
- владеть service readiness;
- владеть exposure eligibility;
- зависеть от runtime truth;
- быть explainable через diagnostics и local control surface.

`Hosted Services` не обязан владеть publication outcome.

## 4. Publication

`Publication` обязан:

- иметь явную publication model;
- зависеть от node readiness и service readiness;
- зависеть от policy и network capability;
- снимать публикацию при потере runtime backing;
- быть explainable через diagnostics и local control surface.

`Publication` не должен:

- быть скрытым helper-слоем внутри `Node Runtime`;
- быть detail `Discovery`;
- жить как ad-hoc coordination между workload и network без собственного ownership.

## 5. Жесткая Связь Между Ими

Правила жесткие:

- workload и hosted service не независимы;
- hosted service readiness не существует без runtime backing;
- publication существует только если есть реальный runtime backing и transport/network eligibility;
- потеря workload может снять publication;
- потеря policy permission может снять publication;
- потеря network capability может снять publication или перевести его в degraded.

## 6. Admission И Policy

Workload admission должен учитывать:

- локальную конфигурацию;
- policy rules;
- допустимость capabilities;
- ограничения ресурса и режима узла.

Hosted-service readiness и publication также должны учитывать policy.

## 7. Наблюдаемость

Оператор должен видеть:

- какие workloads зарегистрированы;
- какие workloads реально работают;
- какие workloads failed или degraded;
- какие services готовы к публикации;
- какие services реально опубликованы;
- какие services сняты и почему;
- требуется ли operator action.

## 8. Минимальные Критерии Реализации

Плоскость нельзя считать реализованной, пока одновременно не выполняются условия:

- workload реально исполняются;
- observed state отделен от desired state;
- restart behavior предсказуем;
- hosted services имеют канонический readiness state;
- publication truth отделена от service readiness truth;
- services публикуются только при реальном runtime backing;
- остановка или failure workload меняют readiness и publication;
- оператор может понять текущее состояние и причины отклонений.
