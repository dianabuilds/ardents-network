# Тестирование и качество

## Итог

Unit-слой широкий и один полный обычный прогон прошёл. Выбранный race scope также прошёл. Слабое место — не количество domain tests, а отсутствие детерминированных failure/lifecycle проверок на границах process, Docker, ingress, stream shutdown и deployment transaction. Именно в этих местах статический аудит обнаружил P1.

Canonical CI сейчас не является рабочим quality gate: static job падает до запуска большинства suites (CI-001), а native-install gate имеет отдельную ошибку evidence path (CI-002).

## Структура тестов

| Уровень | Расположение / механизм | Назначение | Фактическая проверка в аудите |
|---|---|---|---|
| Package unit | 295 `*_test.go` рядом с пакетами | Доменные правила, parsing, stores, handlers, adapters | `go test ./... -count=1` — PASS |
| Race | selected high-risk packages | Гонки в messaging, transfer, Waku, identity и workload | PASS; ingress package не имеет tests |
| Coverage | все обычные Go-пакеты | Statement-level ориентир, не критерий корректности | PASS; результаты ниже |
| Integration | `tests/integration` плюс tagged Docker executor tests | Multi-component runtime, discovery, content, diagnostics, Docker | 43/44; один teardown failure |
| E2E | `tests/e2e`, opt-in tag/profile | Process-level пользовательские сценарии | Не запускался |
| CI gates | `tests/ci` | failure contract, security, deployment, native, multinode, release | Статически разобраны; не запускались отдельно |
| Test support | `tests/testkit`, `fixtures`, `tooling` | Process/network harness, scenario catalog, import boundaries | importguard PASS; integration runner использован |

Tracked test surface: 101 файл в `tests/`, включая 44 integration, 12 E2E, 19 testkit, 9 CI, 7 tooling и 3 fixture-файла. Ещё unit tests расположены рядом с production-кодом.

## Выполненные проверки

Повторных прогонов не выполнялось; таблица фиксирует единственный достаточный набор уже полученных результатов.

| Команда / проверка | Результат |
|---|---|
| `go list ./...` | PASS, 95 first-party packages |
| `go test ./... -count=1` | PASS, 82.3 s |
| `go test ./... -covermode=atomic -coverprofile=<external-temp>` | PASS; all-source 44.5%, handwritten runtime 56.7% |
| `go test -race` для `internal/messaging`, `internal/transfer`, `internal/ingressproxy`, `internal/network/waku`, `internal/identity/access`, `internal/workload/...` | PASS; ingress сообщил отсутствие test-файлов |
| `./ardents.ps1 test integration -RebuildContainer -ReportDir tests/.artifacts/reports/audit-20260723-integration` | FAIL: 44 total, 43 pass, 1 cleanup failure; 122.4 s test duration, около 506 s вместе с rebuild/orchestration |
| `go vet ./...` | FAIL: copylock/noCopy в `sdk/go/internal/adapter/enrollment_test.go:140` |
| `go run ./tests/tooling/importguard` | PASS |
| `scripts/generate-api.ps1 -Check` | PASS на canonical LF clone exact SHA; Windows checkout дал только CRLF false positive |
| `tests/check-format.ps1` | PASS на canonical LF clone exact SHA; тот же Windows CRLF false positive |
| CI testcatalog invocation | FAIL: путь `tests/cmd/testcatalog` отсутствует, актуальный tool не имеет `-mode` |
| `govulncheck ./...` | PASS: 0 called/imported vulnerabilities; один module-only exception GO-2026-5932 |
| `staticcheck` / `deadcode` / `gosec` | Не получили пригодного результата: локальные версии несовместимы с Go 1.26.5/source syntax |

Generation/format failure на Windows не зарегистрирован как finding: exact commit в canonical LF checkout проходит оба gate. Это ограничение локального checkout/скрипта, а не доказанный drift generated code.

## Покрытие

Расчёт по сохранённому во внешнем temp coverage profile:

- все source statements: 12 896 / 28 987, **44.5%**;
- runtime API + `cmd` + `internal` + SDK: **45.1%**;
- весь `internal`: **46.0%**;
- handwritten `internal`: **56.3%**;
- handwritten runtime: 12 489 / 22 011, **56.7%**.

Generated code и test tooling меняют знаменатель, поэтому 56.7% — наиболее полезный общий ориентир. Однако единый процент скрывает наиболее важные пробелы:

| Участок | Coverage | Почему важен |
|---|---:|---|
| `cmd/ardentsd`, `cmd/ardentsctl`, `cmd/ardents-ingress-proxy` | 0% | Process wiring, exit и shutdown |
| `internal/ingressproxy` | 0% | Здесь подтверждены REL-001 и REL-002 |
| `internal/workload` core | 5.6% | External engine lifecycle и mutex/context behavior |
| `internal/hosting` | 9.7% | Publication/workload boundary |
| `internal/network/routing` | 5.1% | Network failure/retry behavior |
| `internal/replication/availability` | 0% | Degraded/placement logic |
| `internal/applicationapi/binding` | 0% | Principal/resource binding |
| `internal/applicationapi/call` | 17.5% | Sealed admitted call propagation |
| CLI content/network/node/workload packages | 0% | Client mapping и error contracts |
| CLI diagnostics/output/TUI | 16.8–21% | Operator-visible failure paths |
| `internal/keyring` | 0% | Local key lifecycle |
| `internal/content/payload` | 0% | Payload boundary |

Порог общего coverage сам по себе не исправит QLT-001. Нужны file/flow-specific tests для high-impact seams.

## Подтверждённая хрупкость integration

`TestDiscoveryDegradesWhenBootstrapPeerIsUnavailable` выполнил assertions, но `testing.TempDir` cleanup получил `directory not empty`. Evidence:

- `tests/.artifacts/reports/audit-20260723-integration/junit.xml`;
- `tests/.artifacts/reports/audit-20260723-integration/summary.json`;
- test source `tests/integration/discovery/degraded_test.go:20-42`.

Четыре более ранних retained reports этого дня показывают pass того же test. Поэтому это не стабильный functional failure, а intermittent teardown/lifecycle symptom (TST-001). Точный background writer не установлен; отчёт не выдаёт гипотезу за подтверждённую причину.

## Пробелы, совпавшие с найденными дефектами

1. Нет cross-owner collision tests для Object/Manifest; существующий B4 проверяет sibling ID, но не одинаковый ID двух владельцев (SEC-001).
2. Нет retained expired credential + valid recovery matrix (IAM-001).
3. Нет commit-to-delivery failure injection для bootstrap/application ticket (IAM-002).
4. Нет capacity/adversarial envelope tests replay ledger (SEC-002).
5. Нет Waku Store retention/growth bound test (SEC-003).
6. Нет multi-responder «malicious error before honest success» test (SEC-004).
7. Нет ни одного ingressproxy unit/integration test для RST, half-close, idle slots, fairness или proxy death (REL-001/REL-002).
8. Нет active stream + SIGTERM bounded shutdown test (REL-003).
9. Нет fake hung Docker transport для query/metrics/shutdown paths (REL-004).
10. Нет rollout failure-injection после каждого mutation и composite readiness test (OPS-001—OPS-003).
11. Нет clean-checkout contract test для CI/native evidence directories (CI-001/CI-002).
12. Нет independent clean-builder test и negative commit/source binding test (SUP-001/SUP-002).

## Диагностика и наблюдаемость

Сильные стороны:

- `internal/diagnostics` хранит typed health/events/operations;
- production observability integration scenario прошёл и подтвердил согласование readiness/metrics и redaction;
- deployment scripts в основном сохраняют operator-visible ошибки;
- proxy logs ограничены Docker local log rotation.

Проблемы:

- ingress proxy превращает connection error в process exit, не сохраняя отдельное stable health state;
- daemon shutdown может не дойти до финальных diagnostics/cleanup при stream или Docker stall;
- rollout acceptance не читает Diagnostics;
- integration cleanup evidence не содержит имя фонового writer/goroutine;
- Waku Store не публикует quota/retention pressure, потому что самой quota нет.

## Рекомендуемый quality gate после исправлений

1. Clean LF static: generation, format, vet, importguard и исправленный catalog validation.
2. Unit + critical-file coverage и selected race.
3. Deterministic negative lifecycle tests для ingress, stream, Docker и ticket handoff.
4. Disposable integration один раз; E2E/multinode только после зелёного static.
5. Native/deployment transaction gates с failure injection.
6. Release build в независимых immutable environments с provenance/material verification.

Повторение одного и того же зелёного unit run не повышает уверенность в этих границах; полезнее добавить отсутствующие failure contracts.
