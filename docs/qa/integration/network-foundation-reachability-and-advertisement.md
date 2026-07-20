# Network Reachability And Advertisement

## Scenario ID

`NFI-004`

## Layer

`integration`

## Domain

`Network Foundation / Messaging`, with Publication consumption.

## Goal

Prove that joining peers or binding a listener does not make a node publicly
reachable, and that direct-public endpoint publication follows AutoNAT ingress
observation.

## Preconditions

- a real Waku `service_node` starts in `public_direct` mode;
- the operator supplies a valid public TCP multiaddr;
- no public reachability observation exists at startup.

## Steps

1. Start the node and confirm `reachable=false` with no endpoint in its local
   node record.
2. Apply the deterministic integration equivalent of a libp2p AutoNAT `Public`
   event.
3. Confirm the status becomes `public` and the address is published.
4. Apply a `Private` event representing failed public dialback/NAT ingress.
5. Confirm `nat_blocked`, `reachable=false`, and withdrawal from the node record.

## Expected Result

Only verified public ingress makes the advertised address usable and
network-published. A later negative observation withdraws it immediately.

## Failure/Degraded Variant

`Private` and `Unknown` never retain a public endpoint. Diagnostics expose a
bounded reason without raw dial or resolver errors.

## Related Tests

- `tests/integration/network-foundation/reachability_test.go::TestPublicReachabilityGatesAndWithdrawsNodeAdvertisement`
- `internal/network/transport/reachability_integration_test.go::TestPublicIngressObservationWithdrawsAndRecoversRealWakuEndpoints`
- `internal/network/transport/reachability_integration_test.go::TestPublicAddressChangeRequiresRestartAndFreshObservation`

## False Positive Risk

The repository integration injects the same typed libp2p reachability outcome
consumed by production so it is deterministic on one host. Multi-host external
dialback and deployment ingress evidence remain mandatory in STB-307.
