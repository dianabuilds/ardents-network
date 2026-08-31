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
