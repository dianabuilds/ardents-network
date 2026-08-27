# Service command end to end

These tests build the Service and Application commands and exercise their
public Unix-socket boundaries. They cover readiness timeout and cleanup plus an
Exact-Target stream that retains one Service Connection when its current Route
fails and the first replacement also fails. The recovery scenario mocks only
the external Route socket; Service, publication, Application IPC, continuity,
and workload processes are real commands.

The Reference C-2 scenario starts eleven bounded fixture roles — Publisher,
its one-shot local Reference Site Application, User, an independent
alpha-resolution Observer Endpoint, alpha OHTTP Gateway, alpha Relay,
Destination Resolution Gateway, Introduction, Initiator, Rendezvous, and
Responder — as separate processes.
The Publisher admits that local Application only over one token-bound loopback
stream, then supplies its byte stream to the C-2 `Accept` handoff; the
Application has no Route, Target, State, or browser-selection authority.
Its transit duties reopen accepted signed native-route State, require one valid
refresh wave from two mutually authenticated State Source processes, and then
drain through their maintained
Node lifecycle. On Linux, its harness constructs an exact enrolled v3 bundle,
accepts the fixture-only signed `ardents-alpha://reference` corpus through the
separately manifested `ardents-control accept-alpha-corpus` processes, one per
distinct Endpoint-owned persistent corpus floor. The User and independent
Observer then resolve the same link through their alpha OHTTP Clients, the
separately started Relay and Gateway, and their own retained floors; neither
trusts corpus bytes or corpus authority carried by the Publisher publication
envelope. Other
platforms retain the same floor-consumer boundary but do not qualify that
control-command procedure.
`TestReferenceC2CarriesOneRouteThroughProductNodeCommands` is the narrow
product-command variant: it replaces the four fixture transit roles with
separate `ardents-node` Initiator, Introduction, Rendezvous, and Responder
processes and completes one local C-2 journey. Its remaining C-2 roles are
still fixtures and State is accepted during startup. Its Linux Docker run then
sends `SIGTERM` to each Node process and requires `DRAINING` then `WITHDRAWN`;
the Windows compatibility harness retains forced cleanup.
`TestReferenceC2RefreshesStateAndWithdrawsProductNodeCommands` replaces both
authenticated State Sources with a signed linked successor and requires all
four Node commands to drain and withdraw. The product-transit offline case
returns `service unavailable`, with no Reference URL or browser request. These
are functional topology, State-change, and bounded-unavailability evidence.
`TestReferenceC2RefreshesStateAndWithdrawsProductNodesWithHeldRoute` further
holds an established Publisher Application-to-Service path, replaces State at
both sources, observes `DRAINING` then `WITHDRAWN` for every product Node, and
releases both endpoint fixtures only to their classified terminal outcomes. It
does not qualify hostile failure, multi-host C-2, capacity, or independent
operation.
`TestReferenceC2HardStopsRendezvousWithHeldRoute` is the H4-2 local Docker
full-system fault emulator. It uses the same held route, waits for independent
Publisher and User setup acknowledgements, hard-stops the actual product
Rendezvous command, and releases the endpoint fixtures only to their classified
terminal outcomes. The qualification runner cross-builds and mounts exact
Linux bytes read-only in one resource-bounded container with no external
network. It proves the selected process composition and simulated fault
semantics, not a physical-host outage, public-path recovery, capacity,
availability, or independent operation.
The exact product-command C-2 test also passed from one temporary Docker 29.4.1
container on the project Ubuntu VPS on 2026-08-26: current source was mounted
read-only, no port was published, exit status was `0`, and the run took
`22.51s`. Its container and temporary source directory were removed after
capture. This is one-host functional evidence only.
The selected exact Target then crosses the private
lookup Entry/Initiator/Gateway carrier and C-2, and the tracer verifies the
bounded static Reference Site document, declared stylesheet, and declared SVG. In its
explicit-browser mode it waits for the Publisher to observe all three requests;
opening Firefox alone is not accepted as evidence.
Its separate H4-3B dynamic-application scenario keeps the same eleven-process
topology but selects the payload-neutral HTTP/1.1 bridge instead of the static
Reference Site: a form POST/body/header/cookie, redirect, cookie-bearing
follow-up, and chunked response pass through one authenticated Service
Connection. The Publisher checks its normal `reference.ard` Host and request
facts; unregistered alpha and ordinary Internet names fail at the local proxy.
After the fixture Publisher Application closes, its authenticated terminal
closes the User presentation and a later name request cannot remain usable.
Three local process cells additionally prove the selected failure contract:
explicit Service Administration withdrawal returns `unpublished` for the exact
Target/generation after drain, and a fresh repeat withdrawal proves the live
publication is gone; resetting the Publisher Application after a
partial response gives both Endpoints `abrupt connection loss`; and hard-stopping
the Publisher Endpoint during an active request gives the User `abrupt
connection loss` while the local Publisher Application sees only its closed
handoff. The process cells refuse the failed name, an unregistered alpha name,
and ordinary Internet as fallback. A Reference behavior cell additionally
keeps a distinct registered Target live, proves it receives no request when
the selected Target fails, and then proves it remains explicitly reachable.
These cells do not qualify Docker/VPS, selected platform/browser,
actual multi-host operation, HTTP Upgrade, or a general participant Browser
Entry; the latter remains outside H4-3B's critical path.
The complete local eleven-process C-2 set, including independent alpha
Observer, alpha OHTTP Gateway, and Relay, passed after the persistent-floor
boundary was introduced on 2026-08-26.
The joined eleven-process Linux Docker command-to-floor-to-alpha-OHTTP-to-C-2
scenario also passed on that date, as did one ephemeral project-VPS Docker
container with a read-only current-source mount and no published ports. Both
cells set `1` vCPU, `1 GiB` memory, and `128` PIDs. This is functional
low-resource evidence only; it is neither multi-host nor a capacity,
performance, or minimum-supported-profile result.
Its separate offline case first retains an authentic published descriptor, then
removes the Publisher's local Introduction slot. The User must report bounded
`service unavailable`, receive no Reference URL, and make no browser request.
Its local-handoff refusal case supplies a distinct Application token. Publisher
must reject it before C-2 admission, create no Application-ready signal, and
leave every started transit/Gateway process drained.
For an explicitly cross-compiled Linux qualification only, a
test runner may provide the matching fixture through
`ARDENTS_E2E_FIXTURE_REFERENCE_C2`; ordinary local and CI runs leave it unset
and rebuild the fixture from the current checkout. All three C-2 scenarios,
including the Publisher-local Application handoff, declared document,
stylesheet, SVG, exact Target authentication, static response-policy headers,
offline result, and local token-refusal result, passed from a clean local Linux
Go container on 2026-08-25. This is process and Linux-runtime evidence, not a
multi-host qualification claim.
The same three scenarios then passed from archived source commit `ab78f257` in
one separate constrained Go 1.26.6 Docker container on the project Ubuntu VPS:
Docker 29.4.1, `2 GiB`, `1 CPU`, and `128` PIDs, no published ports, read-only
source mount, exit code `0`, and `45.350s` total on 2026-08-25. The temporary
container and source archive were removed after capture. This qualifies only a
project-controlled VPS Docker profile; it does not establish host independence,
public deployment, capacity, or browser privacy.
On a Windows qualification host with an explicitly selected Firefox executable,
run `make qualification-h4-4a-firefox` with `ARDENTS_REFERENCE_C2_FIREFOX` set
to its absolute path. The selected target fails if that prerequisite is absent;
it is not an ordinary-process skip. Its first leg starts at visible
`http://reference.ard/`; Endpoint resolves that name from its accepted local
alpha corpus and opens only the exact selected Service. The scenario has no
browser profile, proxy, DNS, VPN, or trust-store configuration authority. Its
second C-2 leg gives the User Endpoint a temporary Browser Entry state path and
opens the visible `http://reference.ard/` route only after C-2 authenticated
that exact Service.
The temporary Firefox page performs the Publisher's ordinary document, form
POST, redirect, cookie, chunked-response, and close navigation; the fixture's
Go HTTP client is disabled for that leg. Firefox 154 passed this dynamic
Browser Entry scenario on 2026-08-26, including removal of the Browser Entry
state after the selected Service withdrew. It is one local compatibility and
payload-transparency scenario, not a proof for arbitrary applications, H4-7
browser isolation, location privacy, public DNS, HTTPS, or a final
participant-browser address.
