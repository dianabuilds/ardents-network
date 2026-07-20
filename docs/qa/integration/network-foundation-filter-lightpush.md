# NFI-005 — Filter, Lightpush, And Offline Recovery

## Scenario ID

`NFI-005`

## Layer

`integration`

## Domain

`Network Foundation / Messaging`

## Scope

Integration scenario for `Network Foundation / Messaging` and the private
envelope boundary.

## Preconditions

- a real go-waku full provider runs Relay, Store, Filter server, and Lightpush;
- a `constrained_light_client` runs TCP with `outbound_only` reachability;
- both peers share valid capability material, but network-visible selectors and
  payloads remain opaque.

## Scenario

1. The constrained client joins a provider that advertises Filter, Lightpush,
   and Store protocols.
2. It verifies that Relay is not active locally and that client capabilities
   reflect observed provider protocols.
3. It subscribes through Waku Filter using a capability-derived content topic.
4. It encrypts a private envelope and submits it through Waku Lightpush.
5. The Filter path delivers the ciphertext; the authorized channel opens it.
6. Waku Store subsequently returns the retained opaque envelope for offline
   recovery.
7. A separate degraded path connects to a peer without Filter-server and Store
   support and confirms that connectivity alone does not make the light client
   joined or ready.

Lightpush success means provider acceptance only. End delivery is proved
separately by Filter reception, and offline recovery is proved separately by
Store fetch.

## Expected Result

- no Relay participation exists on the constrained client;
- readable product meaning does not enter Filter or Lightpush selectors;
- Filter, Lightpush, and Store client capabilities are backed by observed peer
  protocols and real carrier exchange;
- the full provider supplies the required server roles without creating a
  second network substrate.
- missing required provider protocols produce a stable degraded reason and are
  not promoted to active capabilities.

## Automated Test

- `tests/integration/network-foundation/light_client_test.go::TestConstrainedClientFilterLightpushAndOfflineRecovery`
- `tests/integration/network-foundation/light_client_test.go::TestConstrainedClientRejectsPeerWithoutRequiredProviderProtocols`
- `tests/integration/network-foundation/light_client_test.go::TestConstrainedClientCapabilitiesReachCanonicalStatus`

## False Positive Risk

Connection to a provider could be mistaken for protocol readiness. The tests
require observed Filter, Lightpush, and Store protocols plus actual encrypted
publish, delivery, and retained fetch outcomes.

## False Negative Risk

Provider discovery and Filter delivery are asynchronous. Each stage has a
bounded wait and reports its own failure, so temporary scheduling delay is not
confused with an unsupported provider shape.
