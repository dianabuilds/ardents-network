# H4-3B constrained Docker qualification

## Question

Do the exact current Linux command and fixture bytes preserve the selected
application-transparent HTTP/1.1 Service contract under the bounded H4-3B C-2
topology, including each declared Publisher terminal outcome?

## Run from Windows Docker

```powershell
make qualification-h4-3b-docker
```

The runner cross-builds current Linux `ardents`, `ardents-control`,
`ardents-node`, the `reference-c2` fixture, and the Service E2E test binary to
one temporary directory outside the repository. It mounts only those bytes
read-only in a disposable `golang:1.26.6` container with no external network,
1 vCPU, 1 GiB memory, 128 PIDs, and a private executable `/tmp`.

The four independent cells are:

1. an ordinary dynamic Publisher request: POST body/headers, cookie, redirect,
   follow-up, and chunked response;
2. explicit withdrawal of the exact Target/generation after the active
   connection drains;
3. Publisher Application reset after a partial response; and
4. abrupt Publisher Endpoint loss during an active request.

Every cell starts the current separate-process C-2 fixture. On Linux that
fixture builds an enrolled v3 bundle and accepts its separately manifested
alpha-control corpus before the User resolves the selected Service. The tests
also refuse unregistered alpha names, ordinary Internet names, and any fallback
to another live Target.

The runner also starts two independent fresh H4-6A control observations from
the same enrolled input. Each has its own floor and must accept the same pinned
catalog, Release, and Network result; a cached restart is checked separately.
The artifacts remain synthetic evidence generated inside the test. They do not
replace concrete release component identities or an independent participant
handoff.

## Scope

This is release-candidate Docker evidence for the exact current bytes. It does
not prove a published participant artifact, independent enrollment contact,
two physical hosts, VPS execution, selected desktop browser, HTTP Upgrade or
HTTP/2 support, capacity, availability, browser isolation, naming privacy, or
Public Beta readiness. Missing Docker or the pinned image is an invalid selected
environment and fails the target.
