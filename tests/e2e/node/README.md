# Node process tests

These tests build the real Node command and exercise readiness, authenticated
role probes, duty replacement, drain, restart, credential isolation, and
cleanup through separate operating-system processes. The native-duty readiness
cell starts separate Initiator, Introduction, Rendezvous, and Responder
commands from exact materializations of one signed State Epoch; it does not yet
carry a Route through those commands. Its companion `tests/e2e/service`
product-command C-2 cell carries one local Route through that four-duty command
topology and, on Linux, verifies `SIGTERM → DRAINING → WITHDRAWN` after the
completed journey. Its companion State-successor test withdraws all four
commands after a linked signed Epoch refresh. The Linux-specific Rendezvous
test additionally carries one authenticated active pair, then verifies that
`SIGTERM` closes it through `DRAINING → WITHDRAWN`; the ordinary process test
rejects a State-unauthorized leg. Its incomplete-TLS companion proves the
product command holds no more than its configured handshake reservations and
serves an authenticated pair after those incomplete connections release their
slots; it is not a DoS-resilience claim. Fixtures are generated in the test
temporary directory and disappear with the run.

The closed-issuer provisioning cell runs actual `ardents-node` initialization
and restart, transfers its unchanged public JSON export to `ardents-control`
inspection under independently supplied Node/Network pins, and uses the
resulting token-key fields in separate prepare/sign/inspect processes. Its
Epoch and Node Records are canonical signed fixture inputs accepted through
the actual offline command. A signed mismatched Node digest is refused; the
valid profile's reported digest and durable route survive reopening. This does
not qualify installed workers or the complete
acquire-register-publish-refresh-withdraw journey. For each accepted Carrier,
the same cell then accepts the issuer's exact materialization and signed profile
into a fresh Node root, starts two real Sources, and runs the actual issuer
command to READY. After abrupt process termination and joined exit, the same
roots return to READY with the same duty digest. Entry and Interior commands
also start from their exact materializations. On Linux, the real Route client
uses the accepted State to send a blinded batch through these three commands;
the cell requires the credential owner to unblind and verify the token. After
abrupt issuer loss, a new batch must receive a bounded transport refusal with
no response or cleanup failure. The test retains that exact batch, Permission,
blinding state and selected peers, restarts the issuer, and makes one explicit
recovery attempt. It must verify the token without replacing the pending
request. This exercises the same-process reconciliation allowed by
[private admission](../../../docs/technical/private-admission.md), not an
automatic retry or a promise of zero transient refusal after remote READY.
The lost attempt occurs while the issuer is stopped, so this does not prove
reconciliation of an already committed reservation or rollback protection.
Custody authority creation and permission issuance use the actual command in
a real terminal. Password input waits for echo to be disabled; the independently
transferred request digest is entered at its separate prompt. A mismatched
digest must refuse before vault unlock and leave no permission file. Public
receipts are checked against the private permission file, which then supplies
the real network exchange. The Route client is not yet the ordinary Endpoint
command. Other platforms exercise readiness
only. This does not establish graceful shutdown or prove that each Node
contacted both Sources before declaring readiness.

The closed text topology provisioning cell extends the same canonical command
path to sixteen Node Records covering the Source, issuer, resolution,
Introduction, Responder, and JOIN roles. It checks each accepted Node identity,
Record digest, role domain, subrole, and duty generation against the signed
plan, then checks that reopening preserves the route. Both accepted Carriers
are exercised. It then starts two actual Sources and all sixteen Node commands
from their exact accepted materializations, selecting each role through its
local reservation. Each Node must report READY and remain alive after the last
Node starts. The cell does not prove continuous duty readiness, perform an
ordinary Endpoint exchange, or qualify graceful shutdown.

The closed Source cell starts two actual Source commands from independently
accepted copies of that Epoch. A separate State owner acquires the matching
generation through both mutually authenticated Sources. Unsupported or mixed
profile selections and absent or foreign signer pins cannot produce readiness.
This verifies distribution, not closed-profile distribution or Node duty
readiness.

Run them with `make e2e` or `go test ./tests/e2e/node`.

The purpose-named `make qualification-native-rendezvous-multihost` target is separate from
ordinary process tests. It cross-builds the current Node command, starts only a
temporary real Rendezvous plus its two authenticated product State Sources on a
declared project VPS Docker host, and opens the two native direct legs from the
local Windows test process. It proves exact State-authorized byte carriage,
unauthorized-leg refusal, terminal closure of an active pair after abrupt
remote Node/container loss, and three container-namespace netem outcomes across
that public two-host path. The netem relay applies kernel delay, 100% loss, or
fixed delay/loss/reordering only to its disposable Docker interface and proves
respectively exact carriage, bounded refusal with observed drops, or exact
256 KiB carriage with declared qdisc facts; it never changes the VPS host
qdisc. The target does not claim a full C-2 Route, true VPS-loss recovery,
capacity, public-path hostile-network resilience, or independent operation.
The current project-VPS runs passed on 2026-08-26; their exact temporary
containers and remote directories were removed after each oracle completed.

## Installed ordinary text commands

`TestInstalledClosedTextCommandsThroughNodeProcesses` is selected only by the
Linux `text_worker_installed` build tag. Run its prebuilt test binary as root
outside the Endpoint service on the dedicated preinstalled Ubuntu host, with
`ARDENTS_TEXT_COMMAND_QUALIFICATION=1` and `ARDENTS_E2E_COMMAND_ROOT` pointing to
independently hashed, root-owned product commands from the same candidate.
The ordinary worker and its manifest must match that candidate. Missing setup
is an invalid environment; the profile has no fixture fallback.

The test provisions canonical State and all sixteen real Node commands, creates
and accepts a Service Instance through Custody and Instance commands, then runs
the ordinary `ardents endpoint headless` as the unprivileged Endpoint service's
MainPID. Its Source plan uses the two live Sources and retains actual exposure
history in the Endpoint-owned local-role root. The fixture shares a Source client
identity with its Node setup; this is not evidence of independent participants.
The orchestrator prepares private Instance/token directories and launches text
commands through user-owned Bash pipelines with pipefail so their pollable stdio
can be reopened by that same user and command failures remain failures.
It checks permission-request commitments from that exact invocation
against the request files and uses real Custody responses. The ordinary text
commands publish and read a 64 KiB document through this network on both Carriers.
The test observes the real resolution Store through bounded reads of its atomic
public record. It verifies signatures, unchanged publication, a next revision and
fresh slot/key after elapsed refresh, then reads again after a further 65 seconds.
The observation allows command completion through 330 seconds after the first
Descriptor start; it does not extend runtime validity or independently prove
the exact ACK time, old recipient erasure or the overlap cutoff. Run the two
Carrier cases with a twenty-minute test timeout; each Endpoint unit has a
ten-minute watchdog. No clocks or scheduler fields are advanced.
An explicit request through the real Administration socket withdraws publication;
the ordinary Link command must then refuse while the same Endpoint invocation
remains active and no Reader or Publisher worker unit remains active or stopping.
The process collector continuously drains periodic resource samples; only
lifecycle and non-periodic events enter its bounded event queue. This avoids
stalling Nodes while the test waits for publication refresh, and does not
provide resource-measurement evidence.
This uses one Endpoint for both local roles, not two independent Endpoint hosts.
Compilation alone does not prove this installed journey works.

Preserve the prior ordinary worker, manifest and existing temporary Endpoint
unit before a run. The test requires an inactive service without drop-ins and
restores its prior temporary unit during cleanup. An external runner must also
restore those files after test failure or timeout and verify their hashes,
MainPID zero and absence of live workers. Retain the candidate hashes, invocation
journal and terminal result outside Git. This cell does not qualify continuous Node readiness or full recovery.