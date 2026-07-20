# Reference Runtime Flows

## Роль В Наборе Документов

Этот документ не задаёт целевую архитектурную форму `v1`.

Он остаётся non-normative reference summary, который нужен только для:

- проверки системных уроков из legacy runtime;
- сравнения нового slice с reference invariants;
- извлечения proven mechanisms без копирования legacy tree shape.

## Назначение

Этот документ оставляет только актуальное summary runtime flows из legacy `aim-core`, которые важно
помнить при разработке `v1`.

Предыдущая версия ссылалась на абсолютные локальные пути `aim-core`, отсутствующие в текущем
workspace. Битые ссылки удалены; документ оставлен как non-normative reference summary.

## Что считать обязательными runtime уроками reference

### 1. Runtime собирался как один узел

- runtime в reference не был россыпью независимых utilities;
- существовал один orchestration-first entrypoint;
- state directory и startup order были частью обязательного baseline.

### 2. Lifecycle был component-driven

- start шел в фиксированном порядке компонентов;
- stop шел в обратном порядке;
- порядок компонентов влиял на фактическое поведение системы.

### 3. Identity влияла на network participation

- identity continuity была runtime behavior, а не просто persisted profile;
- activation/deactivation identity влияла на transport/network state;
- continuity across restart была частью expected system behavior.

### 4. Waku был реальной network foundation

- network participation строилась вокруг Waku node lifecycle;
- bootstrap trust и manifest verification были частью startup path;
- fallback paths были degradation behavior, а не вторым canonical substrate.

### 5. Discovery включал publication, verification и trust gating

- discovery не сводился к local catalog;
- self-publication, import и verification были единым operational flow;
- trust outcome влиял на usable resolution, а не существовал отдельно от runtime truth.

### 6. Data plane был retrieval-oriented

- local storage был только частью behavior;
- важны были fetch policy, provider selection, cache semantics и failure diagnostics;
- metadata-only interpretation не отражала реальный scope reference behavior.

### 7. Diagnostics агрегировали domain truth

- diagnostics собирали сигналы разных domains в один операторский surface;
- health/explainability были aggregate behavior, а не набором разрозненных snapshots;
- diagnostics нельзя рассматривать как вторичный optional export.

### 8. Control surface сходился в один runtime-backed entrypoint

- даже при перегруженной legacy форме reference сходился в один runtime-backed server;
- эта инварианта важнее конкретной service-wrapper topology;
- для `v1` это подтверждает курс на один canonical local control surface.

## Как использовать этот документ

- Использовать его как checklist при сравнении нового slice с reference invariants.
- Не использовать его как justification для копирования tree shape `aim-core`.
- Если нужен file-level legacy audit, его нужно делать в отдельном workspace с доступным `aim-core`,
  а результаты фиксировать уже без абсолютных локальных ссылок.
