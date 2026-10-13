# Fresh Startup manager observation

`make startup-manager-check` runs on a dedicated Ubuntu24.04 x86-64,
systemd255, cgroup-v2 qualification manager. It requires the fixed Endpoint
and both text activation objects, an inactive root-owned qualification unit,
and the original non-root Endpoint test invocation. It is a manager mechanism
profile, not installed Startup, Service or whole-host qualification.

Build the systemd test package with `startup_manager_native` and the canonical
`-trimpath -buildvcs=false` flags. Install that independently pinned binary at
`/usr/lib/ardents/qualification/installation-systemd.test`, root:root mode0555.
The root-owned mode0644 temporary unit at
`/run/systemd/system/ardents-endpoint.service` selects only
`TestInstallationNativeFreshStartupProperties`, with a twenty-second test
timeout, the dedicated non-root service account and the ordinary Endpoint
protection. Missing or substituted prerequisites are invalid environments.

The root driver receives independently obtained binary and unit digests through
`ARDENTS_STARTUP_MANAGER_TEST_SHA256` and
`ARDENTS_STARTUP_MANAGER_UNIT_SHA256`. The test checks its original MainPID,
reads Version and both complete rounds of fixed properties, and compares stable
required values and original signatures with independent busctl observations.
The original service must terminate successfully and retain its exact PASS
record before the driver accepts. No typed observation supplies a Release
proof, accepting Startup, qualified worker or Service permission.

Ordinary Linux mechanism tests independently encode bounded property bodies
and reject nested length expansion, malformed/truncated/duplicate values,
foreign sender and broadened scalar reference replies before library decoding.
Physical reference-transport cancellation remains with that original owner;
the new property vocabulary does not change its close/join mechanism.
