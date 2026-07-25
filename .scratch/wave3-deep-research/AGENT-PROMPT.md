# Wave 3 Research Agent Prompt

Copy the text below and replace `<ASSIGNMENT>` with one of `DR-01`, `DR-02`,
`DR-03`, `DR-04`, or `DR-05`.

```text
Ты исследовательский агент Ardents Network.

Assignment: <ASSIGNMENT>

Твоя задача — завершить один R2 deep-research packet и подготовить решение,
которое можно независимо проверить и затем разрезать на вертикальные
implementation issues. Ты не реализуешь выбранный дизайн в рамках этой задачи.

Перед началом
--------------

1. Полностью прочитай:
   - AGENTS.md
   - CONTEXT.md
   - docs/agents/domain.md
   - docs/agents/issue-tracker.md
   - docs/engineering/global-feature-research-plan.md
   - docs/engineering/capabilities.json
   - docs/engineering/capability-evidence-register.md
   - docs/engineering/current-remediation-ledger.md
   - docs/engineering/research/wave3-research-charter.md
   - .scratch/wave3-deep-research/PRD.md
   - .scratch/wave3-deep-research/research-packet-template.md
2. Прочитай все ADR и research packets, перечисленные для твоего assignment в
   Wave 3 charter.
3. Проверь `git status --short --branch`.
4. Возьми frozen product baseline commit из charter. Если там нет полного SHA,
   дерево грязное или `main` расходится с `origin/main`, остановись и сообщи
   preparation blocker. Не выбирай baseline самостоятельно.
5. Не удаляй, не откатывай и не перезаписывай чужие изменения.

Источники истины
----------------

Используй источники в следующем порядке:

1. production-код, protobuf-контракты, composition root и исполняемые команды;
2. текущие tests и commit-bound evidence;
3. принятые ADR и нормативные документы;
4. README/Changelog как продуктовую проекцию;
5. исторический аудит только как источник гипотез.

Если исследование требует внешних фактов о Waku, libp2p, TLS, DNS или
контейнерной платформе, используй только первичные официальные документы и
фиксируй URL, версию и дату обращения. Не заменяй внешний факт памятью.

Общие инварианты
----------------

- Application Interface не раскрывает Operator authority.
- Principal, Credential, Access Grant, Delegation, Channel Grant, Waku Peer ID
  и transport identity остаются разными понятиями.
- Авторизация private messages предшествует durable replay admission.
- Content Reference остаётся immutable payload identity.
- Ошибки, restart и recovery должны сохранять одну авторитетную persisted truth.
- Все множества, очереди, cursors, labels и retries должны иметь явные bounds.
- Локальные тесты не означают `Q=yes`.
- Новые Application features не блокируют release существующего stabilization
  scope без отдельного scope decision.
- Kubernetes, QUIC, WebTransport, WebRTC, non-Go SDK и remote Application
  transport не входят в задачу.

Рабочий процесс
---------------

1. Сначала восстанови фактический caller-to-domain journey на frozen baseline.
2. Отдельно оцени implemented, reachable, operable и qualified.
3. Назови actors, assets, trust roots, exact authority и state owners.
4. Классифицируй зависимости:
   in-process, local-substitutable, remote-owned, true-external.
5. Спроектируй минимум два существенно разных варианта.
6. Сравни их по module depth, caller leverage, change locality, trust fit,
   failure clarity, migration cost и operability.
7. Выбери маленький внешний interface и глубокий внутренний seam.
8. Определи happy path, denial, timeout, retry, restart, recovery, revocation,
   rotation, abuse и mixed-generation behavior.
9. Подготовь acceptance matrix по риску.
10. Определи, нужен ли Proposed ADR до реализации.
11. Подготовь dependency-ordered vertical issue breakdown, но не публикуй issue
    files без подтверждения maintainer.

Границы assignment
-------------------

DR-03 Production Channel Grant authority:
- реши trust root, realm, issuance, protected delivery, acknowledgement,
  recovery, membership, revocation, generation rotation, backup/restore,
  separation discovery/data/application channels и audit workflow;
- не проектируй Messaging API;
- результат обязан разблокировать DR-01 и private multi-host assumptions.

DR-01 Application Messaging:
- начинай только после принятого результата DR-03;
- реши addressing, one-to-one/group membership, online/offline delivery,
  acknowledgement, deduplication, ordering, expiry, receive model,
  backpressure, quotas, large Content References, revocation и audit;
- произвольный `Publish(topic, bytes)` запрещён;
- Waku selectors, encryption, replay, Store query и retry должны остаться
  внутри deep module.

DR-02 Application Hosting:
- реши ownership между Application Principal, workload и published service;
- реши registration versus owned workload, lease, readiness, renewal, drain,
  crash, restart, protocol/ingress policy и publication withdrawal;
- не раскрывай Application три отдельных workload/hosting/publication API;
- не решай Direct Service client contract вместо DR-05.

DR-05 Direct Service Interaction:
- начинай после принятого DR-02;
- реши, является ли Ardents discovery-only plane или также выдаёт client
  adapter;
- реши Principal authentication, service authorization, TLS identity,
  rotation, pinning, limits, retry/error semantics и границу application
  protocol;
- не превращай Ardents в общий service mesh.

DR-04 Multi-host Reachability:
- реши минимальные private-LAN/public-direct topologies первого release,
  bootstrap/DNS availability, NAT/firewall, advertised endpoints, WSS
  certificates, churn, partitions, Store availability, recovery, deployment
  ownership, upgrade order и observability;
- Kubernetes и suppressed transports остаются out of scope;
- не изобретай Channel Grant authority: оформи зависимость от DR-03 и проверь
  совместимость перед финальной рекомендацией.

Выходной артефакт
-----------------

Используй packet template и создай только соответствующий документ:

- DR-01: docs/engineering/research/application-messaging.md
- DR-02: docs/engineering/research/application-hosting.md
- DR-03: docs/engineering/research/channel-grant-authority.md
- DR-04: docs/engineering/research/multi-host-reachability.md
- DR-05: docs/engineering/research/direct-service-interaction.md

Параллельные агенты не редактируют
docs/engineering/research/wave3-decision-register.md. Вместо этого добавь в
packet раздел `Decision-register proposals`; integrator перенесёт принятые
решения после review.

Разрешён небольшой throwaway prototype только для одного явно названного
decision risk. Он не становится production implementation и должен быть
удалён либо явно сохранён как prototype evidence.

Definition of done
------------------

- packet привязан к точному frozen commit;
- все существенные факты имеют source/evidence;
- сравнены минимум два реальных дизайна;
- выбран внешний interface и внутренний seam;
- authority, state, privacy, failure, recovery и migration не неявны;
- acceptance matrix содержит unit/contract/integration/E2E/security и нужные
  deployment/release gates;
- нет открытого вопроса, меняющего interface/trust/persistence/migration у
  implementation-ready issues;
- предложены вертикальные issues и зависимости;
- capability `Q` не повышен;
- рабочее дерево содержит только артефакты твоего assignment.

В финальном сообщении укажи:

- answer-first recommendation;
- изменённые файлы;
- выбранный и отклонённые дизайны;
- proposed ADR;
- issue breakdown;
- cross-stage dependencies;
- выполненные и недоступные проверки;
- точный `git status`;
- что должен проверить integrator.
```
