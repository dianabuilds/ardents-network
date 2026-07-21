# Distribution Model

Ardents has two public executable roles, independent of deployment mechanism.

| Artifact | Role | Required where |
| --- | --- | --- |
| `ardentsd` | Node daemon, native bootstrap, network and optional workload control | Every node host |
| `ardentsctl` | Operator CLI/TUI over the authenticated local control API | Operator workstation or node host |
| `ardents/node` image | Optional packaging of `ardentsd` and `ardentsctl` | Docker deployments only |
| workload ingress proxy image | Optional isolated forwarding adapter | Only Docker workloads that publish admitted ingress |

The proxy is not a third public installation prerequisite. It belongs to the
Docker workload adapter and is acquired as an image only when an admitted
hosted service needs isolated ingress. Nodes with workload execution disabled,
native applications that expose their own endpoints, and workloads without
published ingress do not need it.

## Runtime Selection

`workloads.executor=disabled` is the default and keeps the daemon independent
of Docker. `docker` is an explicit operator choice and requires a configured
Docker Engine. `trusted-process` is restricted to local development and is not
a production substitute for container isolation.

The daemon must remain useful in all three shapes: network-only node, native
node plus a separately managed application, and Docker node controlling Docker
workloads. Packaging may add convenience but must not change the node's core
protocol or require an additional public executable.

## Bootstrap

`ardentsd init` owns first-node initialization. It creates or restores the node
identity, protected capability and replay state, an operator configuration, and
an API token. Native paths default to the supplied data and secret directories;
container provisioning explicitly maps those paths to the runtime mount points.

Remote operator access is provided by `ardentsctl --ssh`, using the system
OpenSSH client to reach the daemon's loopback control API without exposing that
API on the network. Same-host automation may use the private Unix socket emitted
by `ardentsd init`. A public application SDK remains a separate delivery layer;
it should build on versioned application protocols rather than introducing
another daemon-shaped binary.
