# NFI-006 — Network Abuse And Resource Controls

## Scenario ID

`NFI-006`

## Layer

`integration`

## Domain

`Network Foundation / Messaging`

## Category

Abuse resistance, resource bounds, and degraded-path observability.

## Goal

Prove that the real Waku-backed network remains bounded under oversized
messages, Lightpush floods, Store amplification, Filter subscriber exhaustion,
and same-IP connection churn.

## Preconditions

- real go-waku service providers and constrained clients run over TCP;
- explicit Ardents limits are passed into Waku node and protocol options;
- no alternative carrier or fake protocol server is used.

## Steps

1. Reject a payload larger than the configured carrier limit before publish.
2. Burst Lightpush calls past the configured operation token bucket.
3. Retain six messages and query Store with a result limit of three.
4. Fill a Filter server with one allowed subscriber and attempt another.
5. Start five clients from one IP against a provider capped at two connections
   per IP.
6. Restart a service node into Relay-only restricted defense, then recover it
   to the steady provider shape.

## Expected Result

- oversized and rate-limited operations fail explicitly and increment
  operator-visible counters;
- Store returns at most the configured result count;
- Filter rejects subscriptions beyond its Waku-enforced subscriber bound;
- connection churn cannot exceed the go-waku/libp2p per-IP gate;
- restricted defense removes Store, Filter-server, and Lightpush-server stream
  handlers by rebuilding the real Waku node, and recovery restores them;
- allowed traffic remains functional after bounded rejection.

## Failure/Degraded Variant

Repeated failures against one outbound provider produce a temporary local ban.
The ban expires automatically; successful retry clears the penalty. A ban is a
degraded provider state, not a claim that the remote peer was globally abusive.

## Related Tests

- `tests/integration/network-foundation/abuse_controls_test.go::TestNetworkAbuseControlsBoundOversizedAndLightpushFlood`
- `tests/integration/network-foundation/abuse_controls_test.go::TestNetworkAbuseControlsBoundStoreAndFilterResources`
- `tests/integration/network-foundation/abuse_controls_test.go::TestNetworkAbuseControlsBoundConnectionChurn`
- `tests/integration/network-foundation/abuse_controls_test.go::TestRestrictedDefenseRemovesAndRestoresProviderServices`
- `internal/network/transport/limits_test.go::TestProviderPenaltyTemporarilyBansAndRecovers`

## False Positive Risk

Testing only Ardents counters could miss a missing Waku server limit. Therefore
Filter and connection assertions run against real go-waku nodes.

## False Negative Risk

Loopback tests do not measure distributed attack capacity, NAT diversity, or
public-realm RLN economics. Multi-host resource measurements remain STB-307
work.

## Notes

The current admission path is resource-bounded but not anonymous
cryptographic rate limiting. RLN requires realm membership provisioning and is
tracked explicitly in the STB-306 dependency decision.
