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
| `accept-offline --state-root PATH --network-id HEX --authorities HEX,... --threshold N --at RFC3339 --epoch PATH --inputs PATH --materialization PATH` | Accept one complete authenticated offline Network State generation. It emits one `ardents-state-event-v1` `generation-accepted` JSON event. |
| `refresh-sources --state-root PATH --source-plan PATH [--once] [--resume]` | Run one selected Direct-Origin Source wave, or wait for the plan-owned refresh interval. It emits an `ardents-source-event-v1` `source-wave-accepted` event after acceptance. |
| `endpoint run <endpoint-plan.json>` | Run one bounded local Endpoint process. The plan remains Endpoint-owned and temporary; it is not a supported service-management format. |
| `entry import <entry-import-plan.json>` | Import one signed State-referenced Entry Invite into Entry-owned durable replay and replacement state. |
| `name encode <name>` | Print one canonical Service Name wire encoding as lowercase hexadecimal. |
| `name resolve <input-file> <name> <context-hex>` | Perform one private resolution exchange through the selected Namespace and State views. |
| `name control <input-file> <operation-file> <context-hex>` | Perform one admitted private Namespace control exchange. |

The current State and source event schemas are coordinated C0 command outputs:
there is no H3 reader or compatibility window. Resource observations are
Module diagnostics, not a capacity or hosting claim.

## `ardents-node`

`ardents-node source --config PATH` runs one selected Direct-Origin Source
server and emits `ardents-source-event-v1` after its State view is ready.

`ardents-node node --config PATH` runs one separately keyed Node process. It
owns one admitted duty, private probe implementation, pressure reaction, drain,
withdrawal, and joined cleanup; lifecycle JSON uses
`ardents-node-event-v1`. The private probe is not a supported peer runtime or
a selected Node operating profile. A config file is a bounded Node-owned input,
not a general Node configuration contract.

## `ardents-custody`

`ardents-custody` is separate from Endpoint, Release, and Update. It accepts no
Authority material or password through flags, environment, configuration, or an
Application data stream.

| Route | Required flags | Result |
|---|---|---|
| `inspect-envelope` | `--vault-root PATH --envelope PATH` | Validates and prints only canonical public envelope facts as `ardents-custody-inspection-v1`. |
| `verify-record` | `--vault-root PATH --record ID --environment-commitment HEX --network-commitment HEX --root-commitment HEX --kind service|name --id-commitment HEX` | Reads one password from an interactive no-echo terminal, verifies one active encrypted record against exact public commitments, and prints bounded non-secret facts as `ardents-custody-verification-v1`. |

The custody command deliberately does not expose Bundle export, restore,
reconciliation, activation, or Namespace signing routes. Those transitions
remain internal custody lifecycle operations until their complete operator and
platform contract is selected.

## Process and support limits

All three commands return a non-zero process status when their bounded input or
owned lifecycle fails. The repository's current process tests cover retained
source and Node lifecycles; they do not qualify public deployment, platform
durability, privacy, or Node capacity. See the current technical contracts for
[Network, Route, and Node](../technical/network-route-node.md),
[Endpoint and Service](../technical/endpoint-service-runtime.md), and
[Release, Update, and Custody](../technical/release-update-custody.md).
