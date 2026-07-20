# Reference Extraction Notes

## Роль В Наборе Документов

Этот документ не является normative source of truth для `v1`.

Его роль:

- помогать извлекать механизмы из legacy/reference;
- не давать копировать legacy package shape как целевую архитектуру;
- оставаться вспомогательной картой для extraction/rewrite/retire решений.

## Назначение

Этот документ фиксирует, что именно из legacy `aim-core` считается источником механизмов, а не
архитектурной формой для копирования в root `v1`.

Предыдущая версия содержала абсолютные локальные ссылки на файлы `aim-core`, которых нет в текущем
workspace. Эти ссылки удалены как невалидные. Ниже остается только extract/rewrite/retire summary,
который можно использовать даже без checkout legacy repo.

## Правило использования

Если legacy код доступен в отдельном workspace, использовать его нужно как reference source для:

- инвариантов;
- vocabulary;
- runtime flows;
- trust/diagnostics/policy heuristics.

Если legacy код недоступен локально, этот документ остается high-level map того, что вообще стоит
искать и переиспользовать концептуально.

## Классификация

Каждый legacy механизм должен попадать ровно в одну категорию:

- `Extract And Reuse`
- `Rewrite Under New Boundary`
- `Retire`

## Summary By Area

### Runtime Assembly

- `Extract And Reuse`: единый runtime entrypoint, ordered start/stop, один собранный runtime node.
- `Rewrite Under New Boundary`: facade/builder layering и capability registry как structural driver.
- `Retire`: service/facade multiplication как базовую форму продукта.

### Identity

- `Extract And Reuse`: create/restore/activate semantics, continuity across restart, identity event vocabulary.
- `Rewrite Under New Boundary`: wiring identity into current `Node Runtime` and local control surface.
- `Retire`: transport-oriented wrapper duplication вокруг identity.

### Waku-Backed Network Foundation

- `Extract And Reuse`: факт canonical роли `Waku`, real messaging/discovery/store flows, status vocabulary.
- `Rewrite Under New Boundary`: ownership внутри current root domains и diagnostics exposure.
- `Retire`: transport abstractions, скрывающие canonical роль `Waku`.

### Bootstrap Trust And Manifest Flow

- `Extract And Reuse`: manifest verification, trust bundle verification, baked/cache/manifest fallback order.
- `Rewrite Under New Boundary`: ownership между transport, policy и diagnostics.
- `Retire`: legacy-specific path layout assumptions.

### Discovery

- `Extract And Reuse`: record verification rules, self-publication signing, trusted/quarantine gating.
- `Rewrite Under New Boundary`: discovery как один домен, а не смесь catalog/runtime subservices.
- `Retire`: route shaping как отдельный архитектурный центр.

### Data Substrate

- `Extract And Reuse`: provider/fetch/cache policy semantics и diagnostics vocabulary.
- `Rewrite Under New Boundary`: encrypted retention and local lifecycle under current product requirements.
- `Retire`: metadata-only interpretation data plane.

### Policy

- `Extract And Reuse`: validation-first policy updates, signer gating, quarantine semantics.
- `Rewrite Under New Boundary`: policy как реальный owner enforcement outcomes.
- `Retire`: trust/policy drift без явного ownership.

### Diagnostics

- `Extract And Reuse`: diagnostics as aggregated domain signal surface.
- `Rewrite Under New Boundary`: structured reasons, pending operations, redaction-aware visibility.
- `Retire`: export-only diagnostics без product health semantics.

### Control Surface

- `Extract And Reuse`: один runtime-backed control entrypoint.
- `Rewrite Under New Boundary`: grouped local contract и current error/result/event model.
- `Retire`: protobuf-frame dispatch как primary internal form.
