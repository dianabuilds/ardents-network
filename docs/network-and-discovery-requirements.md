# Network And Discovery Requirements

## 1. Назначение

Этот документ фиксирует требования к сетевой плоскости продукта.

Он охватывает:

- `Network Foundation / Messaging`
- `Discovery`

Trust в этой плоскости не оформляется как отдельный домен, но должен существовать
как обязательная функция проверки, допустимости и пригодности сетевого знания.

Для `v1` эта плоскость обязана проектироваться в соответствии с
`canonical-network-foundation.md`.

Privacy requirements for opaque topics, selectors, and encrypted network
semantics are fixed in `docs/network-privacy-requirements.md`.
The minimum architecture for implementing those requirements is fixed in
`docs/network-privacy-architecture.md`.
Cross-surface request/response/error contract discipline is fixed separately in
`docs/communication-contracts.md`.
Transport-variant requirements and transport-side architecture are fixed in
`docs/network-transport-variants-requirements.md` and
`docs/network-transport-architecture.md`.

## 2. Network Foundation / Messaging

Система обязана иметь одну каноническую сетевую основу.

Для `v1` эта основа зафиксирована: `Waku`.

Эта основа должна обеспечивать:

- peer connectivity;
- transport exchange;
- message transport;
- operational основу для discovery;
- operational основу для service reachability;
- operational основу для data announce/fetch/re-serving.

Сетевой substrate не должен быть:

- символическим;
- local-only;
- основанным только на endpoint strings и локальных эвристиках.

## 3. Messaging

Messaging обязано существовать как часть сетевого substrate.

Минимально система должна уметь:

- передавать сообщения между узлами;
- передавать сигналы, нужные для discovery;
- передавать сигналы, нужные для data announce и retrieval;
- обеспечивать доставку в рамках реальной сети, а не только локального happy-path.

Messaging must also preserve product privacy:

- readable product semantics must not appear directly in content topics;
- filter and lightpush selectors must not expose readable owner, service,
  conversation, or blob meaning;
- private payloads must enter relay/store/filter/lightpush paths only as
  encrypted envelopes or equivalent opaque network payloads.

## 4. Discovery

`Discovery` отвечает за сетевое знание о существующих узлах, сервисах и доступных источниках.

Discovery обязано:

- публиковать локальные записи;
- принимать remote records;
- различать node records и service records;
- различать свежие и устаревшие записи;
- учитывать источник записи;
- питаться от реальной network participation;
- участвовать в service lookup и data-source lookup.

Discovery не должно быть:

- просто локальным каталогом;
- просто ручным import/export;
- detached от network reality.

## 5. Trust Внутри Сетевой Плоскости

Хотя trust не фиксируется здесь как отдельный домен, система обязана уметь:

- проверять подписи и происхождение записей;
- отличать valid от invalid;
- отличать trusted от untrusted;
- отличать usable от unusable;
- помещать сомненные данные в quarantine-like режим, если это требуется продуктом;
- объяснять outcome проверки через diagnostics и API.

Trust не должен оставаться только справочной информацией.
Он обязан влиять на runtime decisions.

## 6. Связь С Hosted Services И Data

Сетевая плоскость обязана поддерживать:

- публикацию hosted services;
- обнаружение доступных services;
- объявление доступности данных;
- поиск источников данных;
- доступность повторной отдачи.

## 7. Конфигурируемость

Как минимум должны настраиваться:

- bootstrap peers или иные входные точки участия;
- режим сетевого участия узла;
- ограничения на visibility и publication;
- доверенные anchors или иные источники доверия;
- limits для сетевой активности и discovery intake.

Транспортный стек внутри канонической `Waku`-основы также может иметь
варианты конфигурации и участия. Такие варианты допустимы только если:

- они не создают вторую network foundation;
- они сохраняют required Waku role mapping продукта;
- discovery и messaging truth остаются привязаны к реальной network
  participation, а не к локальной конфигурационной декларации;
- degraded и failed transport-specific paths остаются explainable через
  diagnostics и local control surface.

Transport/privacy participation may also use adaptive profiles above the same
`Waku` foundation, if:

- profile switching is automatic or policy-driven only inside the canonical
  transport owner;
- the client continues to use one product-facing transport contract instead of
  raw transport plumbing;
- the active privacy/exposure profile is visible through diagnostics and local
  control surfaces.

## 8. Минимальные Критерии Реализации

Плоскость нельзя считать реализованной, пока одновременно не выполняются условия:

- узел реально участвует в сети;
- messaging работает поверх реального substrate;
- discovery получает и публикует records;
- trust влияет на usable outcomes;
- services и data sources можно реально обнаруживать;
- оператор может понять, почему сетевое знание принято, отвергнуто или ограничено.
- network-visible selectors do not expose readable product semantics to
  non-authorized observers.

## 8.1 Opaque Topic And Filter Contract

For `v1`, Ardents must treat readable topic/filter values as insufficient for
private product traffic.

The required contract is:

- content-topic meaning is opaque to parties that do not hold the relevant
  capability material;
- filter selectors are opaque to parties that do not hold the relevant
  capability material;
- lightpush addressing inputs follow the same privacy rule;
- diagnostics may explain profile and switching state, but must not reveal
  recoverable selector secrets.

## 9. Waku Role Mapping For `v1`

For `v1`, the required Waku role mapping is:

- `relay` for message propagation;
- `store` for offline retrieval and bounded retention;
- `filter` for mobile and other light clients;
- `lightpush` for clients that do not maintain full relay participation.

The product must be designed assuming that mobile/light clients are real
first-class participants. This means `filter` and `lightpush` are not optional
"future" capabilities in the product model.

`constrained_light_client` implements this mapping without local Relay
participation. It may report Filter, Lightpush, or Store client capability only
after connected peers are identified as supporting the corresponding Waku
protocol. Lightpush provider acknowledgement proves provider acceptance, not
network-wide propagation. End delivery and offline recovery require separate
Filter and Store evidence.

Node configuration may reduce local retention to zero, including `0d` TTL or
`0` retention bytes, but this only changes the local node contribution. It does
not remove `store` from the required network role mapping of the system.
## 10. Trust Ownership Clarification

Trust evaluation is mandatory inside the network/discovery plane, but it is not a standalone product domain and must not be implemented as a separate top-level ownership package.

## Abuse And Resource Admission

Every supported node uses finite defaults for carrier message size, total peer
connections, connections per IP, concurrent product network operations,
operation rate/burst, Filter subscribers, and Store results. Invalid negative
or unsafe operator values fail configuration; zero means the documented safe
default rather than unlimited capacity.

Protocol enforcement remains in go-waku/libp2p where peer and stream identity
are known. Ardents adds bounded aggregate inputs and outbound operation
admission at its domain boundary. Repeated outbound provider failures produce
a temporary local penalty with automatic expiry and successful-retry recovery.
This penalty is not a global trust verdict.

Restricted defense is a real protocol-shape transition for full nodes: the
running Waku node is rebuilt as Relay-only, removing Store, Filter-server, and
Lightpush-server exposure. Recovery rebuilds the steady full-provider shape.
The active and reduced capabilities must describe the rebuilt node, not merely
the desired mode.

RLN is not active in `v1`. The bounded operated-realm admission path is not
cryptographic anonymous spam resistance and must never be reported as RLN.
Public-realm operation requires a separately accepted membership and proof
lifecycle.

The owning domain for trust evaluation in `v1` is `Discovery`, coordinated with transport reality and consumed by policy/runtime decisions.

This ownership includes:

- signature and provenance verification for remote records
- trusted/untrusted and usable/unusable outcomes
- anchor-backed trust evaluation inputs
- explainable trust outcomes exposed through diagnostics and local surfaces

## 11. Bootstrap Source And Replenishment Contract

The supported `service_node` bootstrap contract combines explicit static
libp2p multiaddrs with Waku-compatible signed DNS ENR trees. The signed tree URL
is an operator-provided trust root; unsigned DNS inputs are invalid.

Runtime discovery must:

- bound configured trees and returned addresses;
- accept only addresses compatible with the active TCP/WSS profile;
- refresh and replace signed DNS knowledge instead of accumulating it;
- remove stale DNS-only observations and close their connections;
- keep static peers independent from DNS replacement;
- replenish toward three live Relay peers with bounded retry;
- expose stable, distinct source-discovery, peer-dial, and Relay-readiness
  failure reasons without leaking resolver details.

DNS-discovered peers are not durable state. The signed tree is re-evaluated after
restart so that removed knowledge does not remain automatically usable.
`local_development` rejects remote DNS discovery. Peer Exchange and Discv5 are
not enabled by this contract; their current dependency and exposure decision is
recorded in
`docs/process/v1-stabilization-hardening/stb-303-dependency-review.md`.

## 12. Reachability And Address Advertisement Contract

Network participation and inbound reachability are separate runtime facts:

- `joined` means the node has a live Waku Relay path;
- `reachable` means the node is reachable within its configured ingress scope;
- an outbound peer connection, a bound listener, or an operator-provided
  address alone must not be interpreted as proof of public ingress.

The supported `v1` reachability modes are:

- `local_only` for `local_development`;
- `private_lan` for LAN-scoped service-node ingress without a public claim;
- `outbound_only` for Waku participation without any published inbound
  endpoint;
- `public_direct` for operator-managed public ingress with explicit advertised
  TCP or WSS addresses.

In `public_direct`, configured addresses remain withheld until libp2p AutoNAT
peer dialback reports `Public`. A later `Private` or `Unknown` observation must
withdraw those addresses and expose an explicit degraded reason through the
local status surface. Changing an advertised address requires a restart and a
fresh reachability observation; a previous observation must not authorize the
new address. Unexpected closure of the observation stream is equivalent to an
`Unknown` observation: it withdraws the public claim, remains visible in
status, and must not turn the runtime reconciliation loop into an unbounded
retry or CPU spin.

Automatic UPnP/NAT-PMP router mutation, Circuit Relay reservations, hole
punching, and browser inbound participation are not supported by this contract.
They must fail explicitly or remain unavailable rather than creating a false
reachability claim. The dependency and exposure decision is recorded in
`docs/process/v1-stabilization-hardening/stb-304-dependency-review.md`.
