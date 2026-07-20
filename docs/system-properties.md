# System Properties

## 1. Назначение

Этот документ фиксирует обязательные свойства готового продукта Ardents.

Он отвечает не на вопрос "из каких пакетов это собрано", а на вопрос
"каким обязан быть продукт, если он действительно готов".

## 2. Обязательные Свойства Системы

### 2.1 Система Является Сетью, А Не Локальной Имитацией

Ardents считается сетью только если:

- у узлов есть реальное сетевое участие;
- transport и messaging работают поверх реального substrate;
- discovery питается от реальной connectivity;
- publication сервисов и presence зависит от реального runtime и сетевой доступности.

### 2.2 Один Узел - Одна Управляемая Система

Каждый узел обязан быть:

- запускаемым;
- останавливаемым;
- наблюдаемым;
- восстанавливаемым после restart;
- объяснимым в состояниях `ready`, `degraded`, `failed`.

### 2.3 Runtime Truth Важнее Декларации

Фактами считаются:

- observed state;
- реальное сетевое участие;
- реальное состояние workload;
- реальная публикация service presence;
- реальная доступность данных;
- реальные diagnostics outcomes.

### 2.4 Explainability Обязательна

Оператор обязан понимать:

- что сейчас происходит с узлом;
- почему он degraded или failed;
- какие workload работают, а какие нет;
- какие services готовы к публикации, а какие реально опубликованы;
- какие publication, policy или network причины ограничивают поведение;
- какие операции pending.

### 2.5 Ownership Должен Быть Честным

Продуктовая truth должна принадлежать продуктовым доменам.

Недопустимо:

- держать product truth в runtime assembly;
- держать publication truth внутри `Node Runtime` как convenience detail;
- держать domain ownership в read-side projection;
- выдавать local control surface за отдельный продуктовый домен.

### 2.6 Dependency-Backed Там, Где Речь О Substrate

Продукт не должен строить с нуля:

- network substrate;
- database engine;
- wire protocol framework;
- observability substrate.

### 2.7 Security И Policy Не Декоративны

Policy и security boundaries считаются существующими только если они:

- реально меняют runtime behavior;
- видны в diagnostics и API;
- влияют на admission, publication, retention или network use.

## 3. Обязательные Runtime Guarantees

Система обязана гарантировать:

- устойчивую identity узла;
- предсказуемый startup/shutdown;
- восстановление состояния после restart;
- сохранение explainability через restart;
- сохранение pending operations или их явного terminal fate;
- отделение desired state от observed state;
- отделение hosted-service readiness от publication outcome;
- невозможность чтения чужих удерживаемых данных владельцем relay-узла.

## 4. Недопустимые Свойства

Система не должна вырождаться в:

- transport-driven architecture;
- набор параллельных control surfaces;
- runtime facade как центр всей логики;
- metadata-only substitutes вместо реальных доменов;
- publication helper layer без собственного ownership;
- ложную доменную инфляцию, где application layer выдается за product domains.

## 5. Минимальные Условия `done`

Ardents считается завершенным продуктом только если одновременно выполняются условия:

- узел реально стартует, останавливается и восстанавливается;
- сеть реально существует как сеть;
- discovery работает поверх реальной network participation;
- messaging и transport operational;
- publication отделена в собственную ownership-зону и работает по runtime truth;
- workload реально исполняются;
- hosted services имеют правдивый readiness state;
- data substrate реально удерживает и переотдает данные;
- diagnostics объясняют состояние системы;
- local control surface канонична и usable;
- policy и security меняют runtime behavior.
