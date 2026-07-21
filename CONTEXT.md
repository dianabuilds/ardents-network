# Ardents Network

Ardents connects managed Nodes, Applications, and Operators in a private capability-governed network.

## Language

**Node**:
A running Ardents participant that owns identity, network participation, retained data, and optional workload execution.
_Avoid_: Server, daemon, peer when referring to the complete product role

**Operator**:
A person or automation identity authorized to administer one Node.
_Avoid_: Application, user

**Application**:
A least-privilege program that consumes capabilities provided by a Node without receiving administrative authority.
_Avoid_: Operator, workload, client

**Application Credential**:
An expiring, Node-bound credential that identifies one Application and limits it to explicit Application actions.
_Avoid_: Operator token, API key

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
