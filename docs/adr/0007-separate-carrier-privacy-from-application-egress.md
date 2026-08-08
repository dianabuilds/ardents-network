---
status: accepted
date: 2026-08-08
---

# Separate carrier privacy from Application networking

Ardents provides privacy only for traffic submitted to its local Application
Interface. A claim-bearing private-site or application experience additionally
uses a **Network-Isolated Application Boundary at both endpoint Applications**:
ordinary ingress/listeners plus DNS and direct socket egress are denied by default,
origin/cache/storage state is separated by Isolation Context, and every
secondary destination either uses an explicit Ardents destination or fails.
There is no transparent clearnet fallback. This includes malicious requests
that try to make a Publisher perform a DNS lookup, callback, webhook, or SSRF.

The V1 Named Unlisted Site qualification therefore uses a controlled
single-response client and deterministic HTTP Service whose only listener is
scoped local IPC/loopback through Ardents and whose complete process tree has no
ordinary network ingress or egress. Generic HTTP, SOCKS, stream, and browser
adapters remain useful compatibility surfaces, but must report unverified
Application networking and receive no Application-level Endpoint Location
Privacy claim unless their complete process tree, listener, egress, and storage
profile pass the same supported-platform isolation tests, including external
connection/scan attempts.

This keeps Ardents a network rather than a mandatory browser or application
runtime while preventing a correct Route claim from being misapplied to an
Application that can reveal an endpoint through an ordinary listener, DNS,
fetch, WebSocket, WebRTC, QUIC, external resources, or arbitrary sockets.
Content safety, fingerprints,
credentials, and Application semantics remain outside the carrier claim.
