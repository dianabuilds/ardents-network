# Functional map

Status: **product hypothesis**

Phases describe dependency order, not delivery dates.

## Ardents Client

| Phase | Function | User-visible outcome |
|---|---|---|
| V1 | Create Identity Vault | A Person starts without phone, email, wallet, or central registration and verifies a recovery method. |
| V1 | Add or revoke Device | A replacement Device receives only selected Personas; a lost Device loses future authority. |
| V1 | Create Persona | The Person creates a separate identity for one service, relationship, or context. |
| V1 | Open Unlisted Service by exact name | Anyone who already knows a human-readable Service Name can resolve and open the authenticated service without an opaque address or network directory. |
| V1 | Inspect name trust | The Client shows Namespace, expiry, recovery policy, and resolution status. |
| V1 | Open and cache Site Bundle | A verified site or client application runs in isolation and may retain an approved offline copy. |
| V1 | Control Service permissions | Mailbox, local storage, files, and Credential proofs are separate bounded Capabilities. |
| V1 | Add Contact | QR, Invite, or verification phrase creates a pairwise relationship without a public directory. |
| V1 | Handle Message Requests | Unknown senders are accepted, rejected, blocked, or quarantined before reaching normal conversations. |
| V1 | Send and receive offline text | End-to-end protected messages survive temporary sender or recipient absence. |
| V1 | Select safe Route Profile | The Client chooses the operation-appropriate default and exposes the latency/privacy contract. |
| V1.5 | Send encrypted attachments | A file has explicit retention and download policy without a central file server. |
| V1.5 | Use a small private Space | Members share one bounded collaboration context with scoped Personas and names. |
| V1.5 | Recover connectivity | The Client detects blocking and restores entry through a Bridge without protocol configuration. |

## Developer Studio

| Phase | Function | Developer-visible outcome |
|---|---|---|
| V1 | Register, renew, and recover Service Name | A Developer controls a human-readable name without a mandatory wallet. |
| V1 | Build Site Bundle | Static assets or client code become an immutable reproducible release with declared Capabilities. |
| V1 | Run privacy lint and emulator | Fingerprinting, storage, network, and permission risks are visible before publication. |
| V1 | Sign and publish release | Name resolution and Replicas expose an authenticated version without a stable public origin. |
| V1 | Update, pin, and roll back | Clients follow a signed update policy; a Replica cannot impose a downgrade. |
| V1 | Select independent Replicas | The service remains available when one ordinary storage operator disappears. |
| V1 | Inspect privacy-safe health | Resolution, version availability, and Replica diversity are observable without visitor tracking. |
| V1.5 | Create Namespace and subnames | A team or community defines registration and delegation inside its own naming boundary. |
| V2 | Publish stateful Service Instance | Replaceable hidden instances serve protected state behind the same Service Name. |

## Space Console

| Phase | Function | Steward-visible outcome |
|---|---|---|
| V1.5 | Create private Space | Membership, recovery, naming, and service policy form one explicit collaboration boundary. |
| V1.5 | Invite, revoke, and assign bounded authority | Membership changes do not expose or replace Personas used elsewhere. |
| V1.5 | Install Private Service | A Service receives only Space-scoped Capabilities and data. |
| V1.5 | Set local admission policy | Invites, quotas, puzzles, or optional Credential predicates address local abuse without global identity. |

## Contributor Node

| Phase | Function | Contributor-visible outcome |
|---|---|---|
| V1 | Launch and self-check Node | A Contributor can join safely without access to protected content. |
| V1 | Choose roles and resource limits | Relay, Replica, or Bridge participation is explicit and independently bounded. |
| V1 | Observe health, update, and leave | Churn and software updates do not silently strand retained data or active responsibilities. |
| V2 | Offer sandboxed execution | A Contributor may run confined Service Instances without becoming the Service owner. |

## Network Transparency

| Phase | Function | Community-visible outcome |
|---|---|---|
| V1 | Show control roots | Release, bootstrap, naming, and emergency powers and their quorum are explicit. |
| V1.5 | Show operator diversity | Concentration by operator, network, family, and jurisdiction becomes measurable. |
| Later | Review, appeal, and expire emergency change | Governance actions have evidence, bounded scope, review, and expiry. |

## Enabling capabilities

These are not separate products, but the user-facing functions above depend on
them:

- global and scoped naming with verifiable, privacy-aware resolution;
- independent endpoint-selected paths and temporary rendezvous;
- Interactive and Shielded Route Profiles with separate claims;
- rotating discovery and mailbox identifiers;
- protected replicated storage with bounded retention;
- recovery-root, Device, Persona, relationship, session, Credential, and
  Capability separation;
- replaceable Bridges and entry transports;
- local and anonymous resource-admission controls;
- reproducible releases, constrained applications, and transparent updates;
- observable control-plane and infrastructure concentration.

## Scope rule

A function enters a release only when its user journey, adversary, failure modes,
observable result, privacy limitation, and recovery path are all specified. A
protocol primitive without a complete user journey is research, not product
functionality.
