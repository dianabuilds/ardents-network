# Stage 7 direct-binary and browser Adapter specification

Status: **accepted for Stage 7 development on 2026-08-20.** Direct-binary and
generic-browser implementation is authorized by the S7.0 start record; Windows
OS registration remains blocked until the Product Owner gives its separate
artifact- and operation-specific command.

This specification is the single exact development contract for the R-056 O1
topology. The product contract, accepted ADRs, R-002 Connection Interface, and
R-051/R-052 principal/isolation decisions remain authoritative. The accepted
[R-051 principal](stage-7-application-principal-spec.md) and
[R-052 isolation](stage-7-application-isolation-spec.md) profiles are exact for
implementation and falsification. Where a platform mechanism cannot implement
this contract, that profile is unsupported; the implementation MUST NOT weaken
or silently substitute it.

## 1. Product surface and invariants

One authenticated platform executable supplies all three entry points:

```text
ardents connect [--context <local-context>] [--deadline-ms <positive-integer>]
                [--result json|human] <explicit-destination>
ardents accept  [--context <local-context>] [--deadline-ms <positive-integer>]
                [--result json|human] <explicit-service-target>
ardents browse  [--mode generic|isolated] [--result json|human]
                <ardents-service-link>
ardents desktop uri register [--mode generic|isolated] [--result json|human]
ardents desktop uri status [--result json|human]
ardents desktop uri remove [--result json|human]
```

Installed wraps the exact executable digest released as Portable. No command
branches on Distribution Profile. Browser, URI registration, native messaging,
an SDK, system proxy, DNS, route, TUN/TAP, or VPN ownership is not a prerequisite
for `connect` or `accept`.

Every operation crosses the same Application Broker and Connection Interface.
The command owns parsing, stdio/browser adaptation, bounded presentation, and
cancellation only. It owns no destination resolution, Local Grant, retry,
Route, release, update, isolation, publication, or Authority state machine.

## 2. Direct byte-stream commands

### 2.1 Input and authority

`connect` accepts exactly one destination in a canonical explicit Service Link,
Service Target, or Target-Link encoding already selected by the maintained
Stage 6 destination grammar. It accepts no bare DNS/HTTP URL, search text,
filesystem path, environment-derived destination, or alternate-name fallback.

`accept` accepts exactly one explicit Service Target for an already configured
and currently published local Service. It waits for one authenticated incoming
Service Connection, serves that connection, and exits. It cannot create,
publish, migrate, or alter a Service; issue a Credential; access Authority
Custody; or loop over future connections. A shell or supervisor may invoke a
fresh command for a fresh authorized operation.

The optional context selects one local Isolation Context through the Broker and
is never transmitted. Its absence uses the principal's safe default. The
optional deadline can only shorten the Broker's frozen operation deadline;
zero, negative, overflowed, duplicate, or longer values fail before admission.
Unknown, repeated, conflicting, environment-injected, or trailing arguments
fail as `command-invalid`.

### 2.2 Stream behavior

- stdin is the byte-exact outbound Application stream; stdout is the byte-exact
  inbound stream. Neither adds framing, encoding, prompts, progress, logging,
  newline conversion, or diagnostic text.
- stdout MUST be a pipe, file, or inherited non-terminal handle. A terminal
  stdout fails before admission so hostile remote bytes are never rendered as
  terminal control input. stdin may be a terminal, pipe, file, or inherited
  handle and is never prompted.
- Both copy directions run concurrently under the existing ordered reliable
  Service Connection and its resource/backpressure limits. The command adds no
  persistent queue and never converts a short local write into remote receipt.
- stdin EOF half-closes the outbound direction and continues draining inbound
  bytes and the terminal Connection Result. It does not create a second
  connection. A clean Service Connection close ends stdout with EOF.
- local interrupt, deadline, broken output, or owner process termination cancels
  the same operation. After any partial write, remote completion remains
  unknown. There is no Application-operation replay, reconnect, alternate
  destination, public-network fallback, or hidden retry.

### 2.3 Result and exit contract

`--result=json` is the default. After all Application Data, stderr receives
exactly one newline-terminated JSON object of at most `1024` bytes and nothing
else. `--result=human` emits one bounded safe textual projection of the same
fields. Logs, progress, browser output, and child stderr never share this
channel.

The JSON object has this closed field set:

```json
{"schema":"ardents.adapter-result.v1","operation":"connect","class":"clean-close","reason":"","authenticated_target":"64-lowercase-hex-digits","accepted_bytes":0,"received_bytes":0,"claim":"none"}
```

`operation` is `connect`, `accept`, or `browse`. Counts are unsigned 32-bit
Connection Interface counters. The target is empty until authenticated and is
otherwise the exact 32-byte Target as lowercase hexadecimal. `reason` is a
bounded product-safe refinement and never names a Node, hop, Carrier Channel,
socket, resolver, local secret, browser history, or guessed remote cause.
Unknown fields, invalid UTF-8, duplicate JSON names, overflow, and output above
the bound are forbidden.

For `connect`/`accept`, counts describe that one Service Connection. For
`browse`, the target is the one bound Service Target and counts are the checked
sum across that session's same-Service request Connections; R-054 bounds the
session below unsigned-32-bit overflow, and overflow would be
`indeterminate-failure`, never saturation or wraparound.

`claim` is exactly `none`, `application-networking-unverified`, or
`network-isolated-application-boundary`. Direct streams use `none`; generic
browsing always uses the unverified value. Stage 7 has no selected isolated-
browser profile, so `browse --isolated` returns `isolation-unsupported` and the
isolated claim value is forbidden for browser output.

The sole terminal class maps to process exit as follows:

| Exit | Class |
|---:|---|
| `0` | `clean-close` |
| `20` | `invalid-destination` |
| `21` | `local-denial` (authorization, policy, resource, unsupported profile, or local I/O precondition) |
| `22` | `service-unavailable` when evidenced |
| `23` | `route-unavailable` when evidenced |
| `24` | `target-authentication-failure` |
| `25` | `local-timeout-or-cancellation` |
| `26` | `abrupt-loss` |
| `27` | `indeterminate-failure` |
| `64` | `command-invalid` before an Application operation exists |

An authenticated-established Broker event opens the stream; it is not a second
stderr record or terminal success. Exit `0` means only clean stream closure,
never that the peer Application processed bytes. If no trustworthy class can be
emitted, exit `27` is used and the absence/corruption is retained as evidence.

## 3. Browser handoff

### 3.1 Entry and topology

`browse` accepts one strict `ardents://<Service Name>` link of at most `2048`
input bytes and no ambient or URI-supplied options. Direct invocation works in
both Distribution Profiles without OS registration. The `generic` mode is the
default. `isolated` is selected only by an explicit local option or trusted
local preference; link content cannot select or downgrade the mode.

The same Ardents executable validates the link, asks the Broker for only a
Connection Grant to that exact Destination Binding and Isolation Context, and
starts an ephemeral reverse HTTP Adapter. It does not install an extension,
native-messaging host, proxy/PAC setting, certificate authority, DNS rule,
route, network interface, browser policy, or background browser service.

The Adapter binds an OS-assigned port on the numeric loopback address
`127.0.0.1` or `::1`, never `localhost`, a wildcard, LAN address, fixed port, or
public listener. One browser session receives one unique port, at least 128 bits
of unpredictable bootstrap capability, and one unique Isolation Context. The
listener validates the exact IP-literal `Host`, method, origin, capability,
request count, header/body bounds, and session lifetime; sends no permissive
CORS response; sets `Referrer-Policy: no-referrer` and `Cache-Control: no-store`;
and closes on browser/session exit, cancellation, or the frozen idle/total
deadline. These checks are defense in depth, not a same-user isolation claim.
The `browse` process remains the listener/browser owner until that terminal
event, then emits its one result and performs cleanup; it never reports launch
as completed browsing.

Stage 7 browser compatibility is a bounded same-Service HTTP profile, not a
general proxy or clearnet exit:

- the initial and relative same-origin `GET`/`HEAD` requests are translated to
  the exact bound Ardents Service; request-line plus headers are at most
  `16 KiB`, there are at most `64` headers and no request body;
- response headers are at most `16 KiB`; response data, concurrency,
  backpressure, and total lifetime inherit the frozen Connection Interface and
  R-054 Application resource parent rather than creating a second budget;
- same-origin relative resources stay on the same Service Link and Isolation
  Context. Absolute public-web, alternate-origin, WebSocket, WebRTC, CONNECT,
  proxy, file, helper, download, and automatic secondary Ardents destinations
  are not translated by this Adapter;
- only same-origin relative redirects are followed inside the profile.
  Unsupported redirects/resources fail or remain visibly outside the generic
  privacy claim; they never become an Ardents-to-clearnet fallback; and
- each browser session uses a new origin. No cookie, cache, history, permission,
  session resumption, or Service Connection pool is deliberately shared across
  Isolation Contexts.

### 3.2 Generic mode

Generic mode launches the current default browser through its documented OS
mechanism and leaves that browser, ordinary tabs, DNS, Internet, proxy, VPN, and
kill switch untouched. The session visibly reports
`application-networking-unverified`. Public absolute navigation or malicious
browser/content networking may use the browser's normal network stack; Ardents
makes no Application-level Endpoint Location Privacy claim for this mode.

Absence or launch failure of the default browser is `local-denial` with reason
`browser-unavailable`. It does not open another browser, search, HTTP fallback,
or direct socket and cannot affect `connect`/`accept`.

### 3.3 Isolated browser disposition

Stage 7 does not select an isolated-browser candidate. `browse --isolated`
performs bounded profile/precondition parsing, returns `local-denial` with
reason `isolation-unsupported`, starts no listener or browser, changes no host
state, and never substitutes generic mode.

The exact reason is platform-specific but the outcome is shared. Windows would
need an administrative unpackaged-loopback exemption that is not private to
this Application tree. Ubuntu can isolate networking with bubblewrap, but a
visible unmodified Firefox tree also needs desktop/display IPC outside the
accepted Ardents-only allow-list. Firefox flags, a fresh profile, preferences,
loopback randomness, or process-tree ownership do not close either gap.

This does not remove browser integration: default-browser generic mode remains
supported in both Distribution Profiles and keeps normal Internet/VPN behavior.
It does prevent the implementation from turning the previously investigated
Firefox shape into a privacy claim by inertia. A future candidate needs new
research, exact desktop/loopback confinement, version/update ownership, and an
accepted scope decision before this section can change.

## 4. Optional desktop registration

URI integration is a reversible convenience, not part of Portable delivery or
direct operation. The three `ardents desktop uri` commands above are available
from the same executable in both Distribution Profiles and modify only the
current user's Ardents association. `register` records the trusted local mode,
defaulting to `generic`; `status` performs no mutation; `remove` removes only a
matching Ardents-owned association and trusted mode. Each prints one bounded
machine-readable or safe human lifecycle result, never Application Data.
Installed may offer `register` as an unchecked explicit install/repair choice.
No default action registers a URI, changes the default browser, or claims to
override the operating system's/user's handler choice.

The registered command enters a private `browse --handoff` parser that accepts
exactly one URI argument, rejects all flags/trailing input after activation, and
uses the trusted local browser-mode preference (default `generic`). Windows uses
a quoted executable path and quoted `%1`-style URI argument. Ubuntu uses one
desktop-entry `%u` field code as one argument and a declared `x-scheme-handler/
ardents` association. Platform quoting and injection tests are mandatory; a
string-built shell is forbidden.

Installed owns and removes only its declared registration objects during normal
uninstall. Portable registration is allowed only from an authenticated
Owner-chosen stable executable path; its explicit action records/inventories
that exact path and its removal deletes only a matching Ardents-owned per-user
association. Moving or deleting the executable does not pretend to clean a
stale registration; `status` reports it and `remove` remains available from an
authenticated Ardents executable. Conflicting handlers or OS default-selection
refusal are visible and do not disable direct `browse`.

Desktop commands default to JSON and write exactly one result of at most `1024`
bytes to stdout with stderr empty. Their closed fields are `schema` =
`ardents.desktop-result.v1`, `action` = `register|status|remove`, `class` =
`registered|status|removed|local-denial|command-invalid`, `mode` =
`generic|isolated|none`, `association` =
`absent|registered-not-selected|selected|conflict|stale`, and `path_matches` as
a Boolean. Normal registered/status/removed observation exits `0`, local denial
exits `21`, and invalid command input exits `64`. Registration success does not
claim that the OS selected Ardents as default; that fact appears only as
`association=selected` when the platform reports it.

## 5. VPN and fallback contract

Endpoint Carrier Channels continue to use ordinary unprivileged host
networking. An active full/split VPN, proxy, firewall, or kill switch may carry,
filter, or block them. Ardents neither detects a preferred bypass nor creates
one. A block maps to the narrowest evidenced Service/Route-unavailable class or
`indeterminate-failure`; it never edits host policy, disables the VPN, chooses a
physical interface, or opens a public direct path.

Generic-browser public traffic remains subject to its existing host/VPN policy.
The native R-052 claim-bearing Application has no public path at all; only the
Endpoint process uses the host's already effective Carrier policy. Thus
coexistence means non-interference, not a promise that every VPN permits
Ardents.

## 6. Required evidence and acceptance

The historical R-056 state-model result checks logical fallback and
Distribution-Profile invariants only. The disposable source is C0 retired; the
research record retains the result. It is not platform evidence.

During S7.6 evidence, the scheduled current-Windows/Ubuntu-26.04-Docker
development subset MUST exercise every
observable item below. Native Ubuntu Desktop/default-browser facts, pristine-
Windows facts, and install-associated Windows registration remain respectively
`environment-deferred` or `authorization-pending`, never passed:

- byte-exact `connect`/`accept`, stdio preconditions, half-close, counts, every
  exit/result class, interrupt, pressure, and Installed/Portable parity with all
  browser/registration state absent;
- direct and OS Service-Link handoff, strict URI parsing/quoting/injection,
  conflicts, registration status/repair/removal, moved Portable path, and zero
  undeclared residue;
- default-browser generic behavior under no VPN, full/split VPN, kill switch,
  and blocked Carrier states, plus side-effect-free `isolation-unsupported` for
  isolated browser on both platforms;
- hostile response/content attempts covering public and loopback probes, DNS,
  redirects, absolute/relative subresources, WebSocket, WebRTC/STUN, downloads,
  helper launch, same-user attachment, inherited handles, cross-context origin/
  cookie/cache/storage, restart, browser update, and cleanup; and
- authoritative process, socket, DNS/packet, route/proxy/VPN, registry/desktop,
  filesystem, profile, and survivor observations. A candidate escape is `fail`;
  missing or unreliable required observation is `invalid`.

The exact cells and predicates live in `stage-7-platform-evidence.md`; R-054
freezes their serialization, resource parent, episode counts, and campaign
identity. Maintained code follows the readiness start record and package-map
rules. No browser artifact is selected, and package/URI registration requires
the separately authorized lifecycle operation.
