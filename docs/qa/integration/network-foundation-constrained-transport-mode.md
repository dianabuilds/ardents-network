## Scenario ID

`NFI-002`

## Layer

`integration`

## Domain

`Network Foundation / Transport Variants`

## Category

Transport-profile endpoint exposure truth for constrained and expanded
`Waku` transport profiles.

## Goal

Подтвердить, что активный constrained runtime path остаётся вариантом canonical
`Waku` foundation и не публикует отключённые transport surfaces как доступные.

## Preconditions

- transport runtime стартует через canonical `Waku` foundation;
- constrained `tcp_only` profile uses only TCP transport enablement;
- тест наблюдает опубликованные transport endpoints реального runtime.

## Steps

1. Запустить `transport.Service` в активном constrained `tcp_only` profile.
2. Считать опубликованные transport endpoints после старта.
3. Проверить, что endpoints содержат только TCP multiaddrs.
4. Проверить, что endpoints не содержат QUIC, WebTransport, WebRTC или UDP-only
   surfaces.

## Expected Result

- transport стартует и остаётся `ready` внутри canonical `Waku` path;
- опубликованные endpoints состоят только из TCP-backed multiaddrs;
- constrained profile не маскируется под full transport surface и не публикует
  отключённые transport endpoints.

## Failure/Degraded Variant

- если runtime публикует `/quic-v1`, `/webtransport`, `/webrtc`, `/webrtc-direct`
  или UDP transport surface, сценарий обязан падать;
- если transport не публикует ни одного usable endpoint после старта, сценарий
  обязан падать.

## Related Tests

- `tests/integration/network-foundation/transport_endpoints_test.go::TestTransportConstrainedModeExposesTCPOnlyEndpoints`
- `tests/integration/network-foundation/transport_endpoints_test.go::TestTransportTCPWSSExposesTCPAndWSSOnly`

## Secure WSS Extension

The WSS variant uses a test-CA-issued server leaf, an explicit listener port,
and a certificate-covered advertised DNS address. The scenario verifies that:

1. TCP and WSS are the only published carrier families.
2. The WSS multiaddr contains the configured advertised host rather than the
   local bind host.
3. Replacing the certificate/key pair and restarting the same transport causes
   the files to be reread and revalidated.
4. The restarted transport returns to `ready` and republishes its WSS endpoint.

Missing, mismatched, expired, hostname-mismatched, and self-signed material is
covered by fail-fast unit tests and must never create a partial Waku runtime.

## False Positive Risk

- проверка только факта старта transport без inspection endpoint set может
  пропустить drift, где constrained mode всё ещё экспортирует disabled
  transports.

## False Negative Risk

- endpoint enumeration асинхронна только в пределах старта runtime; сценарий
  должен читать endpoints после успешного старта, а не до завершения bootstrap
  setup.

## Notes

- сценарий закрывает transport-profile endpoint obligation для `tcp_only` и
  `tcp_wss` из `docs/network-transport-variants-requirements.md`.
