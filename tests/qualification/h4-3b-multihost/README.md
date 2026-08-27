# H4-3B two-host qualification design

Status: **active A8 runner; it is not yet a passing qualification or a release
claim.**

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

The local Windows process accepts the signed alpha corpus into its own retained
floor through the same persistent-floor boundary before starting User
resolution. The H4-6A participant-command and enrollment evidence remains the
separate Linux Docker A5 cell; this Windows-to-VPS C-2 runner neither replaces
nor broadens that claim. The local User then performs the HTTP/1.1 POST,
cookie/redirect follow-up, chunked response, and terminal close against the
exact alpha Target. The remote proof is an independent assertion that the
unmodified Publisher Application saw those normal HTTP facts.

The runner must explicitly signal remote completion, collect each normally
completed remote role's classified result, and remove its generated container
and exactly validated `/tmp/ardents-h4-3b-multihost-*` directory. A failed
cleanup is a failed qualification. The intentional hard-stop Publisher has no
result record; its preceding `publisher-crash-ready` control is retained. The
intentional Publisher Application failures must instead retain their exact
fixture error output.

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

The Windows-to-VPS runner is `make qualification-h4-3b-multihost`. It requires
a literal VPS IP, its matching SSH key, five free public high TCP ports plus
three remote-loopback ports, host-network Docker, and the selected
pre-existing Go image. It also requires local Docker with that image to syntax
check the exact remote shell runner before transfer. The runner refuses a dirty
Git worktree and records a digest of the complete staged bundle. Sending this
temporary test bundle to the VPS is an external transfer and needs the Product
Owner's explicit approval before a live run.

This oracle does not substitute for A4 immutable artifact provenance, A9
participant browser/platform qualification, HTTP Upgrade/WebSocket behavior,
or A11 soak/fault measurements.
