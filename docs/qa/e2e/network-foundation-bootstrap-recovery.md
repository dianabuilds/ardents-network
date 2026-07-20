## Scenario ID

`NFE-002`

## Layer

`e2e`

## Domain

`Network Foundation / Messaging`

## Category

Bootstrap peer restart, transport recovery, stable participation identity.

## Goal

Подтвердить, что node-level participation поверх canonical Waku foundation
восстанавливается после рестарта bootstrap peer: локальный узел сначала
становится `ready`, затем явно деградирует после потери seed, после возврата
того же seed снова получает `boot ready`, проходит operator-visible cooldown в
`restricted defense` и затем возвращается в полный `ready` без ручной смены
bootstrap endpoint.

## Preconditions

- bootstrap peer стартует на реальном `transport.Service`;
- bootstrap peer использует persistable transport private key, чтобы advertised
  Waku peer id оставался стабильным между рестартами;
- локальный `node.Node` использует bootstrap endpoints seed как единственный
  network source;
- runtime loop умеет повторять bootstrap dial после объяснимой деградации;
- transport mode controller использует recovery cooldown перед возвратом из
  defense mode в steady participation.

## Steps

1. Запустить seed transport с фиксированным listen port и persisted transport key.
2. Считать его bootstrap endpoints.
3. Запустить локальный `node.Node` с этими endpoints как единственным bootstrap source.
4. Дождаться `Boot.Joined = true`, `Boot.State = ready`, `Trans.State = ready`.
5. Остановить seed и дождаться explainable degraded transport/boot state.
6. Перезапустить тот же seed с тем же persisted transport key и listen port.
7. Подтвердить, что опубликованные bootstrap endpoints не изменились.
8. Подтвердить, что после возврата seed локальный node snapshot показывает
   `Boot.State = ready`, но transport остаётся explainable `degraded` с reason
   `restricted defense mode is active` в течение recovery cooldown.
9. Дождаться возврата локального node snapshot в `Boot.State = ready`,
   `Trans.State = ready`, `Diag.Health.State = ready` после cooldown.

## Expected Result

- bootstrap peer публикует стабильный endpoint identity между рестартами;
- локальный узел не остаётся в ложном bootstrap failure, если bootstrap path
  снова доступен;
- после возврата bootstrap path operator видит explainable cooldown вместо
  бессрочного stuck `degraded`;
- recovery происходит через реальный runtime retry, а не через ручной restart
  локального node.

## Failure/Degraded Variant

- после потери seed локальный узел обязан показать explainable `degraded`
  boot/transport state;
- после возврата seed узел не должен требовать нового bootstrap endpoint, если
  persisted transport identity и listen port сохранены;
- temporary `restricted defense` после восстановления bootstrap допустим, но он
  обязан завершиться возвратом в steady mode после policy cooldown;
- если recovery не происходит, сценарий должен считать это regression в network
  participation truth.

## Related Tests

- `tests/e2e/network-foundation/participation_test.go::TestNodeTransportRecoversAfterBootstrapPeerRestart`

## False Positive Risk

- тест не должен ограничиваться restart без проверки operator-visible recovery;
- совпадение только listen port недостаточно, нужен assert на стабильность всего
  bootstrap endpoint set и на возврат node snapshot в `ready`.

## False Negative Risk

- bootstrap retry и node recovery асинхронны; нужен bounded wait вместо
  single-shot assertion сразу после рестарта seed.

## Notes

- сценарий закрывает реальный multi-node restart path, обнаруженный на Docker
  стенде;
- trust degradation на untrusted remote discovery records сюда не входит и
  проверяется отдельными discovery/diagnostics сценариями.
