# 01. Инвентарь репозитория

## Точка аудита

| Поле | Значение |
|---|---|
| Репозиторий | `<repository-root>` |
| Ветка | `main` |
| Коммит | `52af3b2480b62da60ae82c7f1d43f45cd5778230` |
| Дата фиксации | 2026-07-23 |
| Go module | `ardents` |
| Версия Go в `go.mod` | `1.26.5` |

Инвентарь относится к содержимому указанного коммита. Созданные в ходе аудита файлы в `docs/audit/2026-07-23/` не считаются частью проверяемого исходного состояния.

Источники инвентаря: `git ls-files`, `go list ./...`, `go.mod`, `go.sum`, дерево каталогов и выборочное чтение исходников, контрактов, сценариев сборки и документации. Сгенерированные файлы и внешние зависимости учтены как поверхности поставки, но отделены от рукописного production-кода.

## Сводная статистика

| Метрика | Количество |
|---|---:|
| Отслеживаемые Git файлы | 883 |
| Go-пакеты, возвращённые `go list ./...` | 95 |
| Go-файлы | 754 |
| Go test-файлы (`*_test.go`) | 295 |
| Go-файлы в tests/testkit/fixtures/tooling, включая test-файлы | 318 |
| Сгенерированные Go-файлы protobuf/Connect | 28 |
| Рукописные production Go-файлы вне test-support областей | 408 |
| Proto-контракты | 14 |
| Markdown-документы | 57 |
| PowerShell-сценарии | 28 |
| Shell-сценарии | 6 |
| YAML-файлы | 6 |
| JSON-файлы | 5 |
| Dockerfile | 4 |

295 — только файлы с суффиксом `*_test.go`. Ещё 23 рукописных Go-файла являются testkit, fixtures или test tooling, поэтому расчёт production-кода: `754 - 318 - 28 = 408`. Каталога `vendor/` в репозитории нет; исходный код зависимостей не включён в приведённые числа.

## Распределение отслеживаемых файлов

| Каталог или корень | Файлов | Назначение |
|---|---:|---|
| `internal/` | 635 | Основная реализация узла, API, идентичности, сети, контента и workload runtime |
| `tests/` | 101 | Integration/E2E-наборы, fixtures, testkit и тестовые инструменты |
| `docs/` | 42 | Архитектура, продуктовые, эксплуатационные и security-документы на точке аудита |
| `sdk/` | 32 | Публичный Go SDK и сгенерированные протоколы |
| `scripts/` | 19 | Сборка, выпуск, развёртывание, установка и генерация артефактов |
| `api/` | 17 | Публичный identity proto-контракт и его Go/Connect представление |
| `deploy/` | 11 | Docker, Compose и systemd поверхности поставки |
| `.agents/` | 11 | Локальные инструкции и skills для агентов; не runtime-продукт |
| `cmd/` | 3 | Три исполняемые точки входа |
| `.github/` | 1 | CI workflow |
| Корневые файлы | 11 | Манифесты, правила, лицензия, README, changelog и launcher |
| **Всего** | **883** | |

Корневые файлы: `.dockerignore`, `.gitignore`, `AGENTS.md`, `CHANGELOG.md`, `CONTEXT.md`, `LICENSE`, `README.md`, `ardents.ps1`, `go.mod`, `go.sum`, `skills-lock.json`.

## Значимое дерево

```text
.
├── api/ardents/identity/v1/       публичный Identity API
├── cmd/
│   ├── ardentsctl/                операторский CLI
│   ├── ardentsd/                  daemon узла
│   └── ardents-ingress-proxy/     TCP proxy workload-входа
├── internal/
│   ├── applicationapi/            API приложений, admission и binding
│   ├── buildinfo/                 метаданные сборки
│   ├── cli/                       клиент, команды и TUI
│   ├── config/                    загрузка и строгая валидация конфигурации
│   ├── content/                   каталоги, payload и адресуемые объекты
│   ├── daemon/                    композиция и жизненный цикл узла
│   ├── diagnostics/               health, events и operations
│   ├── discovery/                 records, resolution и trust
│   ├── hosting/                   hosting-модель
│   ├── identity/                  principals, sessions, grants и recovery
│   ├── ingressproxy/              ограниченный TCP ingress
│   ├── localapi/                  локальный Operator API
│   ├── messaging/                 подпись, шифрование и replay-защита
│   ├── network/                   peer/routing/Waku adapter
│   ├── observability/             метрики и локальная telemetry surface
│   ├── policy/                    policy-правила
│   ├── provision/                 provision-потоки
│   ├── publication/               публикация
│   ├── replication/               availability и placement
│   ├── storage/                   локальное персистентное хранение
│   ├── transfer/                  передача контента
│   └── workload/                  Docker/process execution и readiness
├── sdk/go/                        Go SDK
├── tests/                         CI, E2E, integration и test tooling
├── scripts/                       release/deploy/install/generate
├── deploy/                        образы, Compose и systemd
├── docs/                          документация проекта
└── .github/workflows/ci.yml       CI pipeline
```

## Go-пакеты

`go list ./...` вернул 95 пакетов:

| Группа | Пакетов |
|---|---:|
| `internal/...` | 74 |
| `sdk/go/...` | 10 |
| `tests/...` | 6 |
| `cmd/...` | 3 |
| Публичный API и generator | 2 |
| **Всего** | **95** |

Три исполняемых компонента:

- `cmd/ardentsd` — основной daemon, который собирает локальные API, identity, network, storage, content и workload подсистемы.
- `cmd/ardentsctl` — операторский CLI, работающий с daemon через локальный транспорт и с удалёнными узлами через предусмотренные клиентские адаптеры.
- `cmd/ardents-ingress-proxy` — отдельный TCP proxy для проксирования разрешённого workload ingress.

Наиболее крупные участки `internal/` по числу отслеживаемых файлов:

| Подсистема | Файлов |
|---|---:|
| `localapi` | 108 |
| `identity` | 68 |
| `workload` | 56 |
| `cli` | 55 |
| `daemon` | 48 |
| `network` | 47 |
| `content` | 39 |
| `discovery` | 29 |
| `replication` | 28 |
| `diagnostics` | 23 |
| `transfer` | 23 |
| `messaging` | 20 |
| `publication` | 17 |
| `applicationapi` | 15 |
| `policy` | 15 |
| `storage` | 11 |
| `config` | 10 |
| `provision` | 8 |
| `observability` | 6 |
| `hosting` | 4 |
| `ingressproxy` | 3 |
| `buildinfo` | 2 |

## Основные технологии и протоколы

| Область | Реализация |
|---|---|
| Язык и модуль | Go `1.26.5`, module `ardents` |
| RPC | ConnectRPC и Protocol Buffers |
| Локальные интерфейсы | Operator и Application API поверх локальных socket-транспортов |
| P2P | go-waku, libp2p, multiaddr, CID/multihash |
| Локальная БД | bbolt; Waku также приносит собственный SQLite-backed store как внешнюю зависимость |
| Workload runtime | Docker/Moby API и локальное process execution |
| Наблюдаемость | Prometheus client, loopback-oriented метрики |
| CLI/TUI | Bubble Tea |
| Криптография | `x/crypto`, protobuf envelopes, проектные identity/capability/messaging слои |
| Логирование | Zap |
| Поставка | Dockerfile, Docker Compose, systemd, PowerShell и shell scripts |

Прямые зависимости высокой архитектурной значимости на точке аудита: ConnectRPC `v1.20.0`, protobuf `v1.36.11`, go-waku `v0.10.3`, libp2p `v0.48.0`, bbolt `v1.5.0`, Moby API `v1.55.0`, Moby client `v0.5.0`, Prometheus client `v1.24.0` и `x/crypto` `v0.54.0`.

`github.com/ethereum/go-ethereum` присутствует в `go.mod`, однако отдельного first-party smart-contract или on-chain приложения в дереве нет. Аналогично отсутствует отдельный frontend.

## Контракты и публичные поверхности

Репозиторий содержит 14 `.proto` файлов. Контрактные поверхности распределены между:

- публичным identity API в `api/ardents/identity/v1`;
- локальным Operator API в `internal/localapi/protocol`;
- Application API в `internal/applicationapi/protocol`;
- protocol-пакетами Go SDK в `sdk/go/protocol`.

Сгенерированные protobuf/Connect Go-файлы находятся рядом с контрактами, а часть private protocol-представления — рядом с рукописной реализацией. Поэтому при изменении proto необходимо различать:

1. исходный `.proto`;
2. сгенерированный `.pb.go`;
3. сгенерированный Connect-клиент/handler;
4. рукописную авторизацию и доменную реализацию handler.

Сгенерированный код включён в инвентарь и проверялся на наличие и связность, но не считается самостоятельно написанной логикой.

## Runtime и границы доверия

Ключевые runtime-границы, определяющие дальнейший аудит:

- локальный пользователь или CLI → Operator API → identity/access interceptor → domain handler;
- локальное приложение/workload → Application API → application admission/binding → content operations;
- daemon → Waku/libp2p → недоверенные сетевые сообщения;
- daemon → локальные bbolt/файловые хранилища и Waku store;
- daemon → Docker daemon или process runner → workload;
- ingress proxy → workload TCP endpoint;
- scripts/CI → build artifacts, container images, host install и rollout.

Подробная реконструкция этих границ вынесена в [architecture.md](architecture.md).

## Тестовая поверхность

В `tests/` находится 101 отслеживаемый файл:

| Участок | Файлов |
|---|---:|
| `tests/integration` | 44 |
| `tests/testkit` | 19 |
| `tests/e2e` | 12 |
| `tests/ci` | 9 |
| `tests/tooling` | 7 |
| `tests/fixtures` | 3 |
| Корень и вспомогательные файлы | 7 |

Кроме отдельных `tests/` в production-пакетах находятся unit-тесты; суммарно в репозитории 295 файлов `*_test.go`. В инвентаре тесты учитываются как evidence и защитная поверхность, но их наличие само по себе не подтверждает прохождение соответствующего сценария.

## Сборка, поставка и эксплуатация

- `.github/workflows/ci.yml` — единственный GitHub Actions workflow.
- `scripts/` содержит PowerShell и shell пути сборки, release, verification, deploy, rollback/install и генерации identity vectors.
- `deploy/` содержит четыре Dockerfile, Compose-манифесты и unit для systemd.
- `ardents.ps1` — корневая PowerShell entry surface.
- `go.mod` и `go.sum` фиксируют module graph; исходники third-party модулей не vendored.

Отдельного каталога SQL/schema migrations нет. Версионирование и преобразование локального состояния реализуются внутри Go-пакетов хранения и доменных репозиториев.

## Не являющиеся исходным продуктом области

Следующие области не входят в 883 tracked-файла либо не являются runtime-кодом:

- содержимое `.git/`;
- внешние Go module/build caches;
- Docker image и BuildKit caches;
- локальные `var/`, `dist/`, `.artifacts/` и иные игнорируемые runtime/build outputs;
- временные test binaries;
- IDE metadata;
- исходный код third-party зависимостей из module cache;
- `.agents/` и `skills-lock.json`, используемые для работы агентов.

Эти области не должны смешиваться с выводами о проверенном first-party коде. Правила репозитория отдельно запрещают размещать Go caches внутри рабочего дерева.
