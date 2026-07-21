# Ardents Network

Ardents connects managed Nodes and Principals through Applications in a private capability-governed network.

## Language

**Node**:
A running Ardents participant that owns identity, network participation, retained data, and optional workload execution.
_Avoid_: Server, daemon, peer when referring to the complete product role

**Principal**:
A cryptographically distinguishable Ardents subject that can authenticate, own resources, and receive or delegate authority.
_Avoid_: User, account, client, token holder

**Operator**:
A Principal holding administrative grants for one specific Node. The same Principal may be an Operator of several Nodes with independent grants.
_Avoid_: Administrator account, global admin, user type

**Application**:
A least-privilege program represented by its own Principal that consumes capabilities provided by a Node without receiving administrative authority.
_Avoid_: Operator, workload, client

**Credential**:
Evidence accepted by a Node to authenticate one Principal; a Credential does not itself define the Principal's authority.
_Avoid_: Permission, role, grant

**Application Credential**:
An expiring, audience-bound Credential that authenticates one Application Principal. Its permitted actions come from separate Access Grants.
_Avoid_: Operator token, API key

**Access Grant**:
A signed, time-bounded statement authorizing one Principal to perform exact actions on an explicit resource or scope.
_Avoid_: Credential, role, policy, channel capability

**Delegation**:
An attenuated Access Grant through which one Principal allows another Principal, normally an Application, to act within a subset of its authority.
_Avoid_: Impersonation, shared credential

**Actor Principal**:
The Principal that directly authenticated the current call.
_Avoid_: Client, caller process, token

**Effective Principal**:
The Principal whose authority is exercised by a call; it equals the Actor Principal unless a valid Delegation is present.
_Avoid_: User ID header, impersonated user

**Waku Peer ID**:
The transport identity of a Waku/libp2p participant, distinct from its Ardents Principal.
_Avoid_: Principal, Node ID

**Operator Interface**:
The local administrative interface used by Operators to control and inspect a Node.
_Avoid_: Public API, Application Interface

**Application Interface**:
The versioned, capability-scoped interface through which Applications use a Node.
_Avoid_: Operator API, remote admin API

**SDK**:
A language-specific adapter that presents typed Application operations while hiding the Application Interface transport and wire representation.
_Avoid_: Generated RPC client, daemon

**Content Reference**:
An immutable identifier for content whose payload identity is verified by the Node.
_Avoid_: File path, mutable object ID
