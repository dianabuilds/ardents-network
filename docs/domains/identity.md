# Домен Identity

## Назначение

`Identity` отвечает за идентичность узла и каноническую subject-модель, через
которую остальные части системы получают авторитетное представление о том, кто
выполняет действие и какими возможностями он обладает.

Это не пользовательская учетная система и не общий IAM-слой.

## Границы Ответственности

Домен отвечает за:

- создание и восстановление идентичности узла;
- непрерывность identity-состояния между перезапусками;
- каноническую модель `Subject`;
- нормализацию входного identity-контекста;
- вычисление authorization decisions для control surface;
- выдачу identity-материала соседним доменам, которым нужны подпись или
  identity summary.

Домен не отвечает за:

- transport delivery;
- discovery record ownership;
- policy ownership;
- node lifecycle orchestration;
- boundary DTO и RPC-формы.

## Authoritative Truth

`Identity` владеет следующей истиной:

- principal identity узла;
- device/runtime identity узла;
- состояние continuity и восстановления ключевого материала;
- каноническое представление `Subject`;
- результат нормализации subject из внешнего контекста;
- authorization decision на основе subject и capabilities.

## Не-Цели

- хранить вторую параллельную модель `Caller/Scopes` как primary truth;
- быть местом для произвольных credential adapters;
- дублировать policy-решения, которые не основаны на identity semantics.

## Ключевые Инварианты

- у узла в каждый момент времени есть не более одной канонической identity;
- identity continuity не может silently теряться между restart и recovery;
- `Subject` является канонической моделью локальной авторизации;
- legacy поля не могут быть target-моделью домена;
- authorization result должен быть объяснимым и диагностируемым.

## Входные Контракты

- `EnsureIdentity`
  Гарантирует существование identity и возвращает authoritative result.
- `RestoreIdentity`
  Восстанавливает identity из канонического хранилища.
- `SummarizeIdentity`
  Возвращает стабильное summary для соседних доменов и control surface.
- `NormalizeSubject`
  Преобразует внешний контекст в канонический `Subject`.
- `Authorize`
  Вычисляет `allow/deny` для запрошенной capability/domain action.

## Команды И Запросы

### Commands

- `EnsureIdentity`
  Создает identity при первом старте или подтверждает существование уже
  действующей canonical identity.
- `RestoreIdentity`
  Восстанавливает identity continuity из canonical keystore/persistence path.
- `Authorize`
  Вычисляет identity-owned authorization outcome для локального действия на
  основании canonical `Subject`.

### Queries

- `SummarizeIdentity`
  Возвращает стабильное identity summary для соседних доменов и control
  surface.
- `NormalizeSubject`
  Возвращает canonical `Subject`, полученный из внешнего auth context.

## Выходные Контракты

- canonical `Subject` для local control surface и application-слоя;
- signing identity для publication, discovery и network participation;
- identity summary для `Node Runtime`, `Diagnostics`, transport bindings и
  control read models;
- объяснимый authorization result для local control surface, transport bindings
  и diagnostics.

## Authoritative Results

- `IdentityEnsureResult`
  Возвращает созданную или подтвержденную canonical identity и continuity
  outcome.
- `IdentityRestoreResult`
  Возвращает restored identity state либо объяснимый failure/degraded result.
- `IdentitySummary`
  Возвращает canonical summary для use cases, diagnostics и control reads.
- `NormalizedSubject`
  Возвращает единственную canonical subject projection для локальной
  авторизации.
- `AuthorizationDecision`
  Возвращает allow/deny result с identity-owned reason vocabulary.

## Публикуемые События И State Outputs

### Domain Events

- `IdentityEnsured`
- `IdentityRestored`
- `IdentityRestoreFailed`
- `SubjectNormalized`
- `AuthorizationEvaluated`

### State Outputs

- `IdentitySummary`
  Потребители: `Node Runtime`, `Diagnostics`, canonical local control surface.
- `NormalizedSubject`
  Потребители: canonical local control surface, transport bindings,
  application use cases.
- `AuthorizationDecision`
  Потребители: canonical local control surface, `Diagnostics`, соседние
  application flows.

## Фасад Домена

Фасад должен быть узким и прикладным:

- `EnsureIdentity`
- `RestoreIdentity`
- `GetIdentitySummary`
- `NormalizeSubject`
- `Authorize`

Фасад не должен выдавать внутренние структуры хранилища или legacy compatibility
формы.

## Что Не Должно Протекать Через Фасад

- keystore layout, key material handles и persistence records;
- внутренние subject normalization steps и authorization rule objects;
- legacy `Caller/Scopes` shapes и compatibility fallback DTO;
- инфраструктурные adapters для keystore, signing и persistence;
- внутренние continuity/lifecycle state machine детали.

## Привязка К Boundary Слоям

- canonical local control surface получает canonical `Subject`, identity
  summary и authorization decisions только через `Identity` facade.
- `Proto/Connect`, `HTTP`, `CLI` и другие transport bindings конвертируют
  auth context в те же identity-owned контракты и не используют keystore
  adapters или domain internals напрямую.
- transport bindings не получают прямой доступ к key continuity, storage
  records или legacy auth shapes.

## Целевая Структура Каталогов

```text
internal/identity/
  api/
  subject/
  principal/
  continuity/
  authorization/
  lifecycle/
```

## Правила Внутренней Структуры

- `internal/identity/api` является обязательным каноническим публичным
  контрактом домена.
- `subject`, `principal` и `continuity` хранят только канонические identity
  сущности и value objects.
- `authorization` и `lifecycle` содержат только identity-owned semantics и не
  становятся отдельным общим contract world.
- `internal/identity` не использует обязательную целевую форму
  `domain/application/infrastructure`; дополнительные subpackages допустимы
  только внутри `internal/identity` и только по реальному ownership.

## Правила Зависимостей

- `internal/identity` не зависит от `boundary/*` и не использует transport-local
  DTO как доменную модель;
- transport bindings и local control surface зависят только от
  `internal/identity/api`;
- соседние домены получают только `internal/identity/api`, а не прямой доступ
  к внутренней модели identity.

## QA Этапы

Unit:

- создание новой identity;
- восстановление существующей identity;
- отказ при поврежденном keystore или invalid state;
- нормализация subject;
- матрица authorization allow/deny.

Integration:

- `Node Runtime` получает восстановленную identity до начала network
  participation;
- canonical local control surface использует только канонический `Subject`;
- publication и discovery получают signing identity через доменный фасад.

E2E:

- оператор запускает узел, перезапускает его и видит continuity identity;
- невалидный local auth context приводит к объяснимому отказу, а не к silent
  fallback.

## Правила Миграции

## Актуальное Состояние

`Identity` уже владеет:

- subject normalization;
- identity lifecycle;
- key continuity;
- authorization semantics, которые действительно принадлежат identity.

В домен не должны возвращаться:

- legacy `Caller/Scopes` как primary path;
- compatibility fallback, который делает legacy-поля равноправными с
  каноническим `Subject`;
- старые структуры identity state, если они не соответствуют новой
  authoritative модели.

## Private Capability Authority

For `ardents-private/1`, Identity additionally owns:

- canonical binding of a capability to its subject principal;
- validation of the issuing authority, grant signature, scope, permissions,
  lifetime, generation, and revocation state;
- encrypted persistence of imported grants and local capability references;
- authenticated delivery of a grant to a node through an attested HPKE delivery
  key;
- resolution of a local opaque reference into short-lived capability material
  for an admitted use.

Identity does not expose capability secrets, raw selectors, or derived envelope
keys through local API or diagnostics. Network endpoints and public identifiers
are not capability authority and cannot derive private selectors by themselves.
