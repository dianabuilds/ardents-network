# Node

Start with [the role command](command/role_command.go). It selects the declared
role, opens Network State, then hands the original invocation to [runtime](runtime).

| If you need to follow… | Start here |
| --- | --- |
| The exact role declaration and its independent roots | [plan/declaration.go](plan/declaration.go) and [plan/root_paths.go](plan/root_paths.go) |
| Admission, supervision and shutdown for an installed role | [runtime/start.go](runtime/start.go), then the file named for the role. [runtime/lifetime_unix.go](runtime/lifetime_unix.go) retires new work and joins the original borrowers before releasing their resources. [runtime/invocation.go](runtime/invocation.go) delivers the joined result to the command. |
| The development `route receive` command | [receive/command_linux.go](receive/command_linux.go). This command owns its original receiving lifetime. |
| The Network profile used by Admission and the reservations transferred into Route | [admission/network_profile.go](admission/network_profile.go), [forwarding_refill_unix.go](admission/forwarding_refill_unix.go) and [issuer_channel_reservation_unix.go](admission/issuer_channel_reservation_unix.go) |
| Offline role declarations, public artifacts or signing | [provisioning/doc.go](provisioning/doc.go) |

[The Node contract](../../../docs/technical/network-route-node.md#node-and-resource-lifecycle)
describes the supported host protections and the original invocation boundary.
[The package map](../../../docs/development/package-map.md) records callers and imports.
