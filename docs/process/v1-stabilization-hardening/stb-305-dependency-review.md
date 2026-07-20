# STB-305 Dependency Review — Filter And Lightpush

Date: 2026-07-19

## Affected Domain

`Network Foundation / Messaging`, with the existing network privacy boundary.

## Dependency Role

The direct substrate dependency remains `github.com/waku-org/go-waku` v0.10.3.
Its Waku Filter v2, Lightpush v2, and legacy Store client/server implementations
provide the required mature carrier mechanisms. Ardents supplies product role
selection, opaque capability-derived selectors, private-envelope protection,
provider capability checks, readiness truth, and fallback semantics.

No dependency or parallel transport substrate was added.

## Product Fit

Accepted. Waku's documented light-node model uses Lightpush for provider
submission, Filter for selective online delivery, and Store for missed-message
recovery. Waku explicitly states that Lightpush acknowledgement does not prove
network-wide propagation; Ardents therefore validates Filter delivery and Store
recovery separately.

Primary references:

- https://docs.waku.org/learn/concepts/protocols/
- https://docs.waku.org/build/javascript/light-send-receive/
- https://docs.waku.org/build/javascript/store-retrieve-messages

## Security Posture

- license and maintenance posture are unchanged from the accepted canonical
  go-waku foundation;
- Filter reveals a content topic to the selected provider, so Ardents permits
  only capability-derived opaque topics and encrypted private envelopes;
- constrained nodes do not mount Relay, a Store provider/database, or Filter
  server;
- provider protocol support is observed from libp2p Identify data rather than
  inferred from connectivity or configuration;
- upstream Filter server defaults and Lightpush rate limiting remain in place;
  stricter product-wide quotas and peer penalties are owned by STB-306;
- `govulncheck` residuals remain governed by
  `docs/security-exceptions.md`; this slice adds no affected module.

## Recommendation And Mitigation

Retain go-waku Filter/Lightpush/Store as the only carrier implementation.
Mitigations required by this slice are enforced now: opaque selectors,
encrypted payloads, role-specific protocol mounting, bounded provider inputs,
explicit provider capability checks, no delivery claim from acknowledgement,
and Store fallback evidence. Do not introduce a custom light-client protocol or
re-enable Relay on constrained clients.
