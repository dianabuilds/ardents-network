# H4-3B two-host qualification design

Status: **planned A8 oracle; it is not yet a passing qualification or a
release claim.**

This runner must prove one H4-3B journey with the Publisher Endpoint and the
User Endpoint on different hosts. It is purpose-named rather than an extension
of the single-host `h4-3b-vps` Docker repetition: moving one Docker container
to a VPS does not create a two-host route.

## Fixed test topology

```text
local Windows qualification host                 project Ubuntu VPS
--------------------------------                 ------------------
local User Endpoint                              Publisher Endpoint
independent local alpha corpus floor              Publisher-local Application
local Browser Adapter / HTTP client               State Sources
                                                  Introduction / Initiator
                                                  Rendezvous / Responder
                                                  Destination Resolution Gateway
                                                  alpha Gateway / public alpha Relay
```

The Publisher-local Application shares the VPS with its Publisher Endpoint.
That preserves the tested token-bound loopback handoff and gives the
Application no route, target, State, or test-control authority. The User has a
separate local process and a distinct, independently accepted alpha corpus
floor. The route peers and State Sources are intentionally grouped with the
Publisher for this first A8 tracer; the result therefore does **not** claim
independent operators, fault-domain independence, capacity, public deployment,
or a general multi-host network.

The remote runner exposes only the five authenticated peer endpoints needed by
the local User (Introduction, Initiator, Rendezvous, Destination Resolution
Gateway, and the alpha Relay). It uses fixed generated high ports, validates
they are free before start, and records the VPS host envelope. The alpha
Gateway remains remote-loopback behind the public Relay.

## Controlled handoffs

The test creates all credentials, State inputs, and fixture binaries locally,
then stages a temporary, mode-restricted bundle on the declared VPS. The
bundle has no product credentials or participant data. Its ephemeral test
credentials are limited to the generated test Network and expire with the
test deadline.

After the remote Publisher has created its signed publication, the runner
copies only these fixture outputs back to the local User workspace:

1. publication envelope;
2. Destination Resolution Gateway profile;
3. alpha Relay readiness profile; and
4. the Publisher-side dynamic-workload proof.

The local process accepts the signed alpha corpus into its own floor through
the H4-6A control command before starting User resolution. The local User then
performs the HTTP/1.1 POST, cookie/redirect follow-up, chunked response, and
terminal close against the exact alpha Target. The remote proof is an
independent assertion that the unmodified Publisher Application saw those
normal HTTP facts.

The runner must explicitly signal remote completion, collect each remote
role's classified result, and remove its generated container and exactly
validated `/tmp/ardents-h4-3b-multihost-*` directory. A failed cleanup is a
failed qualification.

## Required cells

The initial A8 runner has four separate cells:

1. normal dynamic HTTP/1.1 transaction;
2. explicit Publisher publication withdrawal, classified as `unpublished`;
3. Publisher Application reset, classified as `abrupt connection loss`; and
4. hard Publisher Endpoint loss during the declared request, also classified
   as `abrupt connection loss` for the User.

Each cell must reject an unregistered alpha name and ordinary Internet name;
neither may become a fallback destination. A run records source revision,
digests of staged binaries and test inputs, local and VPS platform envelopes,
ports, exact scenario name, and cleanup outcome.

## Preconditions and limits

The eventual Windows-to-VPS runner requires a literal VPS IP, its matching SSH
key, five free high TCP ports, Docker with host networking, and the selected
pre-existing Go image. Sending this temporary test bundle to the VPS is an
external transfer and needs the Product Owner's explicit approval before a
live run.

This oracle does not substitute for A4 immutable artifact provenance, A9
participant browser/platform qualification, HTTP Upgrade/WebSocket behavior,
or A11 soak/fault measurements.
