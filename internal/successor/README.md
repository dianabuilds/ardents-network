# Successor implementation zone

This temporary source zone separates the new implementation from existing
product packages. It is a grouping directory, not a Go package, domain term,
wire identity or persisted format identifier. The Product Owner selected this
organization on 2026-10-02. Final responsibility-based paths may be restored
after replacement and removal of the corresponding old implementation.

No component packages or executable are created until a real bounded behavior,
its contract, tests and non-test consumer are implemented together.

## Каркас доменов

По поручению Product Owner подготовлены каталоги будущих владельцев с README.
Оставшиеся README-only каталоги — каркас без Go-пакетов, API и заглушек.
Наличие каталога или отдельного proof-кодека не означает приёмку всего домена.
Полный архитектурный инвентарь, включая вспомогательные обязанности, ведётся в
[карте доменов](../../docs/development/domain-map.md). Точные импорты появятся
в package map вместе с реализацией и реальным потребителем.

| Каталог | Назначение | Содержимое |
|---|---|---|
| `network/` | Аутентифицированные факты сети и их актуальность | Существующая реализация |
| `admission/` | Выдача, хранение, предъявление и необратимое расходование прав | Существующая реализация |
| `hosting/` | Физический бюджет провайдера и резервирование | Существующая реализация |
| `route/` | Защищённые маршруты, ролевые каналы и завершение транспорта | Существующая частичная реализация |
| [execution/](execution/README.md) | Локальные полномочия Application, сессия, Job и завершение worker | Только каркас |
| [publication/](publication/README.md) | Жизненный цикл публикации Service Instance | Публичная проверка подписанных proofs; живой Instance и readiness ещё отсутствуют |
| [reachability/](reachability/README.md) | Descriptor Store и проверка достижимости точного Target | Проверка private Descriptor, отдельные долговечный Store и локальная история поиска; реальные receiving и holder потребители через Route |
| [connection/](connection/README.md) | Неизменная начальная привязка logical/Attachment context | Ограниченная привязка к Publication и исходной Execution operation; Service TLS, continuity, ordered stream и recovery остаются будущими обязанностями |
| [enrollment/](enrollment/README.md) | Первичное закрепление доверенного комплекта | Отдельная проверка portable v3 комплекта по независимому pin, закрытый snapshot и реальный read-only consumer; не разрешение Release или запуска |
| [release/](release/README.md) | Разрешение на точные программные артефакты | Только каркас |
| [installation/](installation/README.md) | Установка, замена и восстановление поколения | Только каркас |
| [custody/](custody/README.md) | Хранение корневых полномочий и подпись по назначению | Только каркас |
| `nodeidentity/` | Узкое назначение импортированного материала Node | Существующий вспомогательный владелец |

Enrollment, Release и Installation составляют семейство Software acceptance,
но сохраняют разные полномочия и транзакции. Для общего `software`-агрегата
или универсального `identity`-домена каталог не создаётся.

Внутри будущих доменов пока нет каталогов слоёв, репозиториев, DTO или событий.
Состав реальных пакетов определяется конкретными инвариантами и потребителями.
Существующие Text Application, IPC, наблюдение ресурсов, диагностика,
Qualification и композиция Endpoint/Node сохраняют места и обязанности,
указанные в карте. Naming остаётся будущей границей вне выбранного перехода;
каркас не возвращает удалённый Namespace и не выбирает публичное хранилище,
consensus, blockchain или governance.

Каждый последующий перенос должен принести поведение, реальные новые
потребители, проверки границ и регресс в среде новых доменов. Только тогда
добавляются `doc.go`, регистрация пакетов и разрешённые импорты. Старый код
служит инвентарём для разбора обязанностей, но не подключается к новым владельцам.

The `network` package owns Candidate View, acquisition, local participation,
Epoch/profile history, bound membership, time confidence and coherent
accepted-state observations. Its `state` application orders authentication,
durable decisions and publication. Epoch/profile authentication, Source TLS,
the global root and the separate local restriction journal are registered
adapters; retained byte mechanisms preserve their recovery contracts. The old
`internal/network` tree remains until replacement is completed. New command
composition calls the new application; its integration regression uses new
Admission and Hosting. Exact adapters consume pure Network decisions. Imports
are checked against the package map and import-isolation policy.

## Isolation

All Go files here, including tests and platform-specific files, follow the exact
package imports in [the package map](../../docs/development/package-map.md).
Directory nesting grants no implicit dependency. There is no shared legacy
product allowlist. The command's exact OTel imports and test-only OTLP decoding
imports are enumerated in the isolation test and dependency register.
`admission/issuance` and `admission/token` may consume reviewed CIRCL blindrsa
for issuer signing and holder/receiver operations respectively. New dependencies
require explicit review and the existing dependency acceptance process.

[Admission](admission/README.md) owns its public `issuerprofile` contract, quota
ledger, `issuance` storage/signing owner and `issuer` operation composition.
`issuerprofile` and Hosting use only the standard library. Node Identity depends
only on `issuerprofile` to accept a purpose-bound signing request. Admission's
ledger imports that public grammar; it cannot import identity, issuance or its
operation coordinator.

The reserved command path is cmd/ardents-next. It may compose successor packages,
the standard library and its exact registered OTel imports, but no existing
product packages. Future domain replacements use Network's explicit observations
and retained-duty contracts. Moving old runtime consumers is not a prerequisite
for the new-domain regression. Successor imports remain confined to registered
consumers, with no implicit physical-root or private signing access.
The architecture test package may
inspect source files without importing successor code.

The finite executable's grammar and lifecycle belong to
docs/technical/successor-permission-inspection.md and
docs/technical/successor-hosting-budget.md. The latter also defines command
growth and extraction rules.
Offline issuance-right accounting belongs to
docs/technical/successor-admission-ledger.md; its debit never grants network
permission or consumes/refunds Hosting capacity.
Offline immutable issuer material and unsigned public inventory belong to
docs/technical/successor-issuer-key-material.md. No private key leaves that API.
No automatic reads, conversion or reuse of old state are authorized.
The confirmed offline issuance cycle and its result journal are owned by
docs/technical/successor-token-issuance.md. No arbitrary signing API is exposed.

internal/architecture/successor_isolation_test.go enforces import isolation
across build profiles. It does not establish runtime confinement, correctness,
secret separation or qualification. Dynamic execution and state access require
their own contract checks.
## Operation composition

`admission/issuer` owns the ordered offline Admission/key/result lifecycle. It
also composes pinned Nodeidentity/key/profile provisioning; its exact domain
imports are Admission, its quota, issuance and issuerprofile children, and Node Identity. Command
adapters own configuration, export and telemetry.

Signed profile and existing Node key import contracts belong to
docs/technical/successor-issuer-profile.md.

The independent holder/allocation/issuer/receiver command contract belongs to
docs/technical/successor-admission-commands.md. Its local supplied authority facts
do not qualify Network authenticity or encrypted Custody storage.
