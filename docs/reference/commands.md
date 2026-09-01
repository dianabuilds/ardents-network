# Command reference

Status: **current maintained command routes.** These commands exercise the
implemented closed-test-network Modules. They are not a public operator API,
installer, supported Node hosting profile, or compatibility promise for old
plans and results.

Every command fails closed on malformed, unavailable, or unqualified input.
Its bounded JSON input belongs to the owning Module; a plan is not an ambient
configuration format or an authority source.

## `ardents`

`ardents` is the Endpoint and Network State command adapter.

| Route | Purpose |
|---|---|
| `accept-offline --state-root PATH --network-id HEX --authorities HEX,... --threshold N --at RFC3339 --epoch PATH --inputs PATH --materialization PATH --profile NAME` | Accept one complete authenticated offline Network State generation under the exact selected State profile. It emits one `ardents-state-event-v1` `generation-accepted` JSON event. |
| `refresh-sources --state-root PATH --source-plan PATH [--once|--resume]` | Run one selected Direct-Origin Source wave, resume from current State, or wait for the plan-owned `ardents-source-plan-v1` refresh interval. `--once` and `--resume` are mutually exclusive. It emits an `ardents-source-event-v1` `source-wave-accepted` event only after actual acceptance. |
| `service-instance initialize --config PATH` | Create or reopen one host-owned Service Instance generation from an `ardents-service-instance-initialize-v1` plan whose `request_file` names one new public output, and emit its stable public request plus `request_sha256` for the independently transferred custody ceremony. The command exposes neither private key, Service Authority, Credential, Target, Route, nor Browser state. |
| `service-instance accept --root PATH --response PATH` | Atomically accept only the exact canonical public Authority response for that pending root. An exact repeat is harmless; malformed or different input terminally rejects/conflicts rather than replacing the generation. |
| `endpoint headless <headless-runtime.json>` | Decode the bounded participant plan and call the Endpoint-owned runtime, which retains accepted State, Entry, separate Introduction/Responder acquisition state, and the local Application transports. An optional `service_instance_root` names an already accepted host Instance generation: the runtime reconciles its public Credential with the publication floor, opens the non-exporting binding, and consumes only State's exact Publisher attachment projection. Without that field it is User-only. It exposes only the Service-Link Connection Interface and separately capability-bound Service Administration socket; Browser presentation is absent. Network Epoch authorities are never substituted for Service Authority. |
| `endpoint open <application-socket> <service-link> <input-file> <output-file>` | Open one Service Link through the local Application Interface, half-close after streaming the exact input bytes, and create one new output file from returned bytes. The command receives no State, Entry, Grant, Target, or Route input. |
| `endpoint publish <administration-socket>` | Request publication through the exact local one-use Service Administration capability and render its bounded receipt. |
| `endpoint withdraw <administration-socket>` | Request withdrawal through the exact local one-use Service Administration capability and render its bounded receipt. A publisher plan must explicitly retain its administration listener after publication for this route. |
| `endpoint enrollment-check <alpha-enrollment.json>` | Diagnose one already-running artifact against an independently pinned closed-alpha inventory. It does not authenticate first execution. |
| `endpoint enroll <alpha-enrollment.json>` | Run the explicit Ubuntu Portable enrollment/start path after pin and Release Decision verification. |
| `endpoint enroll-installed <package-enrollment.json>` | Run the explicit Ubuntu Installed enrollment/start path for one root-owned package artifact and versioned static enrollment root. |
| `endpoint user-unit <alpha-enrollment.json>` / `endpoint installed-user-unit <package-enrollment.json>` | Render, but never write, enable, or start, the matching `systemd --user` unit. |
| `endpoint replace <replacement-bundle>` | Perform one explicit Ubuntu local Release-authorized replacement against the fixed user unit; it neither downloads nor schedules updates. |
| `endpoint replacement-recovery` | Report durable replacement recovery classification only; it never starts, replaces, or rolls back a program. |
| `<journal-bound-recovery-program> endpoint rollback <replacement-bundle>` | Perform the only permitted explicit rollback: the retained predecessor, with a fresh Release authorization for its exact bytes. |
| `entry import <entry-import-plan.json>` | Import one signed State-referenced Entry Invite into Entry-owned durable replay and replacement state. |
| `name encode <name>` | Print one canonical Service Name wire encoding as lowercase hexadecimal. |
| `name resolve <input-file> <name> <context-hex>` | Perform one private resolution exchange through the selected Namespace and State views. |
| `name control <input-file> <operation-file> <context-hex>` | Perform one admitted private Namespace control exchange. |

The current State and source event schemas are coordinated C0 command outputs:
there is no H3 reader or compatibility window. Resource observations are
Module diagnostics, not a capacity or hosting claim.

## `ardents-browser`

`ardents-browser run <browser-adapter.json>` runs the optional Browser Adapter
from only an Endpoint Application socket and a Browser-owned state path. It
owns local HTTP presentation and Browser Entry publication; it does not launch
or configure Firefox. It receives no
Network State, Entry, Target, Route, issuer, custody, or Service Administration
authority, and stopping or replacing it does not stop the Endpoint. The known
transparent-origin Browser Entry defect remains a separate security dependency
and is not repaired or qualified by this command.

## `ardents-node`

`ardents-node issuer initialize --config PATH` performs the owner-only bootstrap
of one durable purpose-scoped Transit Grant issuer root. It emits only the
stable public profile receipt; the retained root contains no Network State root
key. Repeating the exact initialization reopens the same public binding.

`ardents-node issuer serve --config PATH` runs only that initialized issuer
under its exact current `transit-issuance` State assignment. It accepts no
parallel native duty reservation, rechecks the State-selected issuer,
Initiator, profile, epoch, and deadline, and withdraws when that binding ceases
to be current.

`ardents-node source --config PATH` runs one selected Direct-Origin Source
server from an `ardents-source-server-v1` input and emits
`ardents-source-event-v1` after its State view is ready.

`ardents-node node --config PATH` runs one separately keyed Node process from
an `ardents-node-plan-v1` input. It
owns one admitted native duty, pressure reaction, drain, withdrawal, and joined
cleanup; lifecycle JSON uses `ardents-node-event-v1`. On Linux, `SIGTERM` and
the foreground interrupt request that local withdrawal before process exit.
An arbitrary Node config is not a supported Node operating profile. A config
file is a bounded Node-owned input, not a general Node configuration contract. Every
native-duty stanza must set its finite `admission_timeout_ms`: it bounds TLS
and binding admission only, is capped by the current State expiry, and has no
implicit default or retry/fallback behavior.

The Rendezvous stanza may additionally set `listen_loopback_override` only to
a literal loopback IP with the same numeric port as the authenticated
State-advertised Rendezvous candidate. This is an operational bind adapter for
a host-owned byte-transparent Carrier relay: State remains the sole owner of
the advertised endpoint, identity, digest, Epoch, and Carrier profile. A
hostname, unspecified or non-loopback address, zero/out-of-range port, or port
mismatch is rejected. Omitting the field retains the State endpoint as the
listener and does not change existing plan behavior.

On Linux, `ardents-node contributor` exposes the complete
`ardents-rendezvous-dedicated-host-v1` dedicated-host lifecycle: `apply`, `diagnose`,
`restart`, `drain`, `withdraw`, and confirmed `remove`. It accepts no other
duty or system-service operation. The exact prerequisites, authenticated
bundle, commands, limits, diagnostics, update recovery, and residue contract
are in the [Rendezvous Contributor runbook](rendezvous-contributor.md). The
surface is accepted only for the project-qualified dedicated-host Functional
Alpha; it is not a public Contributor offer or a capacity/availability claim.

## `ardents-custody`

`ardents-custody` is separate from Endpoint, Release, and Update. It accepts no
Authority material or password through flags, environment, configuration, or an
Application data stream.

| Route | Required flags | Result |
|---|---|---|
| `create-service-authority` | `--vault-root PATH` plus exact environment, Network, and Authority-root public commitments | Reads a new password and confirmation only from the terminal, generates the Service Authority inside custody, and emits its opaque record ID, public Authority, derived Target, and identity commitment. |
| `issue-service-credential` | `--vault-root PATH --record ID --request PATH --response PATH` plus the exact public Service Authority binding | Before opening the Vault or asking for its password, requires the Custodian to type the lowercase `request_sha256` transferred independently from the requesting host. A mismatch fails without password entry, response, record, or floor mutation. The supported issuer additionally limits one Credential to 24 hours and its terminal horizon to 48 hours. It then writes the monotonic public response to the explicit new file and emits its deterministic encrypted successor record ID. An exact retry returns the same response; a different request cannot advance the stale record. |
| `inspect-envelope` | `--envelope PATH` | Validates and prints only canonical public envelope facts as `ardents-custody-inspection-v1`. It owns no Vault root and cannot create custody state. |
| `verify-record` | `--vault-root PATH --record ID --environment-commitment HEX --network-commitment HEX --root-commitment HEX --kind service|name --id-commitment HEX` | Reads one password from an interactive no-echo terminal, verifies one active encrypted record against exact public commitments, and prints bounded non-secret facts as `ardents-custody-verification-v1`. |
| `export-recovery-bundle` | Exact vault record, public Authority commitments, and output Bundle path | Reads the record secret only from the terminal, writes a separately passworded Bundle, and test-restores it before success. |
| `restore-recovery-bundle` | Empty destination Vault, public commitments, Bundle path | Reads the Bundle password from the terminal and writes only an `authority-locked` quarantine record. |
| `purge-record` | Exact vault record, public commitments, and terminal confirmation | Deletes only the exact verified encrypted record after explicit confirmation while retaining the Authority floor. |

The custody command deliberately exposes no Service recovery activation,
reconciliation without a Namespace witness, or Namespace signing route. A
restored Service Authority remains locked and issuance-unavailable; a restored
Name Authority remains locked until a separate fresh opaque Namespace witness
is implemented and verified.

The completed `ardents-release-custody` and `ardents-state-custody` local
ceremony commands have no maintained routes. ADR-0067 retires their current
reader/writer contracts; R-119 through R-121 and Git history retain the exact
historical evidence.

## `ardents-control`

`ardents-control` is a separate alpha-control program. It never starts an
Endpoint, downloads bytes, changes Release/Network State roots, or turns an
alpha name into canonical Namespace state.

| Route | Required flags | Result |
|---|---|---|
| `inspect-bundle` | `--enrollment PATH --artifact PATH --state-root PATH --at RFC3339` | Participant closed-alpha route. It first verifies the exact enrolled bundle and artifact, then runs the maintained Release Decision, Network State, ACA1 catalog, and Release/Network/Compatibility component verifiers. The named standalone inspection root owns separate `catalog`, `release`, and `network` floor children and is physically distinct from Endpoint state. Repeating the same accepted input against the same inspection root reports the cached/no-update outcome; a second absent inspection root supplies an independent fresh observation. |
| `inspect-transitions` | `--enrollment PATH --artifact PATH --state-root PATH --at RFC3339` | Participant transition diagnostic. It runs the same enrollment-pinned inspection and emits `ardents-alpha-transition-report-v1`: nested exact closed-alpha control evidence plus independent Release Safety, Network Epoch, Compatibility, and Namespace-materialization outcomes. It advances only the explicitly named standalone inspection floors; it never mutates Endpoint state. `not-selected` for Namespace never creates a close, release, reclaim, or current Namespace state. |
| `inspect-alpha-corpus` | `--catalog PATH --corpus PATH --disclosure-key HEX --corpus-key HEX --network HEX --at RFC3339` | Read-only diagnostic for one explicit ACA2 catalog and separately signed Alpha Name Corpus under independent keys. It accepts no Endpoint state root, never opens or observes a persistent floor, and reports `ardents-alpha-corpus-report-v1`; invalid, expired, or wrong-network input fails without state mutation. |
| `accept-alpha-corpus` | `--enrollment PATH --artifact PATH --control-state-root PATH --corpus-state-root PATH --catalog PATH --corpus PATH --at RFC3339` | First verifies the exact enrollment-v3-or-later bundle, its enrolled Endpoint executable, and that the running platform-specific `ardents-control` file is the exact separately manifested companion. It then accepts the fixed ACA1 Release/Network/Compatibility evidence and verifies the independently pinned ACA2/corpus component before advancing only the named Endpoint-local corpus floor. An exact repeat is harmless; a higher serial replaces the retained corpus, while a lower or same-serial-different input fails. It reports `ardents-alpha-corpus-acceptance-v1`. |

The caller-keyed low-level `inspect` route and the always-unqualified future
`inspect-public-control` projection are not maintained command routes. Their
underlying verification Modules and historical evidence remain available to
the actual enrolled flows and Git history; no participant or qualification
profile called either route.

The four completed planning-campaign `simulate-*` routes were retired by
[ADR-0060](../adr/0060-retire-completed-planning-campaign-generators.md).
Their historical command lines, JSON schema identities, and receipts remain
unchanged in the accepted ADRs, research records, external evidence, and Git
history; they are not current command compatibility promises.

None of these routes launches a browser, opens a Service, chooses a Relay/Gateway, or
makes an `ardents-alpha://` link a public DNS/HTTPS address. The acceptance
route retains only supplied bytes after its checks; it does not fetch or
install an Endpoint.

## `ardents-browser-entry`

`ardents-browser-entry native-host --state ABSOLUTE_PATH` is the fixed Firefox
native-messaging adapter for the retained optional compatibility trace. It is
not a selected participant Browser Entry. Firefox invokes
it through an exact-ID native manifest, not an operator shell. It consumes one
bounded `loopback-proxy-port` or `loopback-proxy-authentication` native frame
and returns its port (and, only for the latter, a one-process proxy password)
only after the
current Endpoint-owned AlphaProxy answers a random local liveness probe. It
never receives a Service Name, Target, Service credential, route, or browser
URL; its bounded authentication answer is only the separate current local proxy
password. It cannot install the extension, open a browser, resolve a name, or
act as a proxy.

The separately invoked participant route
`ardents-browser-entry install --enrollment PATH --endpoint-artifact PATH`
accepts only an enrollment-v4 bundle. It first verifies the exact enrolled
Endpoint artifact, the running native-host executable, and the fixed-ID XPI
companion against one independently delivered manifest pin. It then creates or
replaces only the current user's exact native-messaging registration. On
Windows this is the one `HKCU\Software\Mozilla\NativeMessagingHosts` value;
on Ubuntu it is the one per-user Firefox native-manifest file. The command
prints the XPI path and `manual-required`: the participant still installs the
Mozilla-signed unlisted XPI explicitly in Firefox. `ardents-browser-entry
remove` withdraws only that native-manifest registration; it cannot remove the
XPI, Endpoint state, Authority, corpus, or Release floors. The installer also
rejects an XPI without the current Mozilla COSE signature metadata, but leaves
cryptographic signature acceptance to Firefox when the participant explicitly
installs that exact XPI.

The matching add-on source and manifest template are in
`packaging/firefox-alpha-browser-entry`. A Mozilla-signed XPI and a real
enrollment-v4 release bundle remain historical promotion artifacts; repository
source or a GitHub download alone is not an installed Browser Entry profile.

## Process and support limits

All maintained commands return a non-zero process status when their bounded input or
owned lifecycle fails. The repository's current process tests cover retained
source and Node lifecycles; they do not qualify public deployment, platform
durability, privacy, or Node capacity. See the current technical contracts for
[Network, Route, and Node](../technical/network-route-node.md),
[Endpoint and Service](../technical/endpoint-service-runtime.md), and
[Release, Update, and Custody](../technical/release-update-custody.md).
