# H4-2 multi-host native Rendezvous qualification

Run `make qualification-h4-2-multihost` from Windows only after explicitly
setting `ARDENTS_H4_2_VPS` to one VPS IPv4 address and
`ARDENTS_H4_2_SSH_KEY` to the matching private-key file. The default account is
`root`; set `ARDENTS_H4_2_VPS_USER` to select another account. The default
public port is `47926`; `ARDENTS_H4_2_VPS_PORT` may choose another unprivileged
port, provided that it and the following three ports are free on the VPS.
The Windows runner resolves the system OpenSSH `ssh.exe` itself and passes its
absolute path to the tagged test.

The qualifier cross-builds the current `ardents` and `ardents-node` commands to
a temporary local directory. It transfers only those two binaries and
ephemeral signed State, materializations, certificates, keys, plans, and a
short runner to one exact temporary `/tmp/ardents-h4-2-multihost-*` directory
on the declared VPS. A detached `golang:1.26.6` container uses host networking
so the product Rendezvous listener can bind the literal public State endpoint;
the two product State Sources bind only to remote loopback. No port is
published through Docker and no project checkout is transferred.

The local tests open the two authenticated, State-authorized direct native legs
to the remote `ardents-node`, carry a fixed byte string, and verify that a
LegBinding whose Initiator certificate claims the Responder role is rejected.
After an independently verified active byte carriage, the loss cell abruptly
stops the remote Node/container and requires both local TLS legs to observe
terminal closure rather than a read timeout. The runner requires the
remote Node to reach `READY`; test cleanup force-removes only its generated
container and exact temporary directory and verifies their absence. A missing
key, remote Docker image, free port, reachable VPS, or cleanup is an invalid
selected environment and fails the target rather than producing a skipped pass.
The verbose result records the exact SHA-256 of both cross-built command bytes,
the native Route profile, State epoch/digest, Docker image ID/version, kernel,
vCPU count, and reported memory. The temporary known-hosts file is test-owned;
the runner never modifies the operator's persistent SSH trust store.

The same target also places a transparent **test-owned local TCP relay** in
front of a real remote Rendezvous. The relay neither terminates nor interprets
TLS and is not an Ardents component. It proves only three bounded outcomes:
an imposed two-way delay retains exact byte carriage, a local TCP RST makes
both active legs reach a terminal error and allows a fresh pair through the
still-running remote Node, and an intentional byte blackhole obeys an explicit
one-second client read budget. The relay does not model link-level packet loss,
reordering, MTU, NAT, active probing, actual network recovery, or an external
adversary.

Finally, the target runs three **isolated kernel-netem sidecars** between the
Windows legs and the same real Rendezvous. A statically cross-built,
qualification-only relay has the single `NET_ADMIN` capability; `tc` and its
host libraries are mounted read-only solely so that the relay can apply one
qdisc to its own Docker `eth0`. It receives neither State, credentials, nor
the product Node directory. One sidecar applies 200-ms delay and must retain
exact carriage; the second applies 100% loss, must expose no authenticated
attachment before the caller budget expires, and must report a nonzero kernel
drop counter; the third fixes 20 ms ±5 ms delay, 5% loss, and 10% reordering
and carries one exact 256 KiB transcript while reporting those declared qdisc
facts. The VPS host qdisc, its existing containers, and persistent files are
not changed. This is container-namespace netem evidence only, not public-path
packet loss, host loss, MTU, NAT, probing, recovery, or availability evidence.

This is one controlled two-host TCP/TLS functionality result. It does not
qualify public deployment, independent operators, true host-loss availability
or recovery, hostile-network resilience, a full C-2 workload, capacity, or the
2 vCPU / 2 GiB NET-01A profile. Abrupt container loss is evidence only for the
remote Node's immediate terminal-closure behavior; it is not a VPS loss or a
fallback test.
