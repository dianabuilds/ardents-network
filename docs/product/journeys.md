# Network product journeys

These journeys define observable behavior of Ardents as a network. They avoid
selecting protocols, libraries, implementation languages, or application
semantics.

## J-01 — Start and join Ardents

**Actor:** User or Developer

**Start:** A newly installed local Ardents endpoint

**Flow:** Endpoint Owner starts the endpoint → verify software/network state →
obtain current bootstrap information → join through an available entry path →
report ready or an exact degraded state

**Done when:** a local Application can use the Application Interface without a
phone, email, wallet, central User account, network administrator approval, or
manual routing configuration.

## J-02 — Open an Unlisted Service

**Actor:** User

**Start:** An exact human-readable Service Name already known by the User

**Flow:** enter exact name → resolve and verify Name Record → obtain current
Service reachability → establish an Interactive Route → authenticate the Service
Target → expose the authenticated target in the result → open a Service
Connection

**Done when:** the Application reaches the intended live Service or receives an
explicit failure. No directory search occurs, and possession of the name is not
shown as authorization or secrecy. The Interactive Route is not a direct path
or single proxy, and no one ordinary Node links the User's location to the
Service Name, Service Target, or Service Instance location. The Service may
still recognize identity disclosed by Application Data, credentials, client
fingerprinting, timing, or behavior.

## J-03 — Publish a local Service

**Actor:** Developer

**Start:** A local application server and an Ardents endpoint

**Flow:** Endpoint Owner grants Authority Custody to an administration tool →
create or securely import Service Authority → obtain its Service Target → grant
per-Service administration → choose one active local listener → publish
authenticated, expiring reachability without exposing raw authority → bind or
update Service Name → accept a test Service Connection

**Done when:** a remote Application can connect while neither the User nor any
one ordinary Node can link the Service Instance's public origin address to its
Service Name or Service Target outside the declared Route Profile. Stopping the
local Service produces an explicit unavailable result, not implied offline
delivery. A routine migration can stop the old Instance, import the encrypted
authority on a new host, and republish the same Service Target.

## J-04 — Integrate an Application

**Actor:** Developer

**Start:** Existing client/server application logic

**Flow:** receive a narrowly scoped Local Grant → separately authorize Service
administration when publishing is needed → use the least-privileged local
Connection Interface → receive a safe default Isolation Context or deliberately
select an additional one → supply either exact Service Name or Service Target →
resolve the name when needed → authenticate and expose the exact target → connect
or accept → read and write opaque bytes → handle close, timeout, backpressure,
and classified failure

**Done when:** the Application can use its own protocol without treating a Node
ID as an application address, embedding a mandatory Ardents SDK, or importing
routing internals. The Application remains responsible for User identity,
authorization, persistence, semantic retry, and data format. Access to connection
traffic alone does not expose Service Authority or Service administration.
Failed name resolution or target authentication never falls back to another
destination or the ordinary network. After a partial write or connection loss,
the network never claims that the remote Application processed the bytes. The
Isolation Context remains local and cannot become an application or network
identity. No Endpoint Owner or Local Grant becomes an authority over the Ardents
network. The journey remains within its declared setup-latency, throughput,
memory, CPU, fairness, and overload budgets under both honest and adversarial
load.

## J-05 — Use the Named Unlisted Site tracer

**Actors:** Developer and User

**Start:** A local HTTP server and a desired Service Name

**Flow:** publish HTTP server as Service → bind name → enter exact name in
reference client → resolve → connect → exchange HTTP bytes → migrate the
authority to a new host without changing the target → simulate compromise by
creating a replacement target and rebinding the same name

**Done when:** the site opens through the generic Service Connection; routine
migration preserves both target and name; compromise preserves only the name;
and route failure remains visible. No replicated Site Bundle, Ardents runtime,
or built-in application identity is required.

## J-06 — Recover from a failed or blocked path

**Actor:** User or Developer

**Start:** An active or attempted Service Connection whose entry or route fails

**Flow:** authenticate target and protocol state → reject detected modification,
replay, redirection, or downgrade → classify only supported facts → obtain
alternate network state or Bridge when required → attempt bounded safe route
recovery within the same Service Connection → restore it or return a
product-level failure class or honest indeterminate result → let the Application
decide whether to open a new connection

**Done when:** connectivity resumes without manual protocol configuration, or
the Application receives enough information to make a safe retry decision. No
claim is made that an interrupted Application operation completed, and no Node
identity or route topology is exposed.

## J-07 — Contribute network resources

**Actor:** Network Contributor

**Start:** A host with bounded bandwidth and possibly other resources

**Flow:** install → choose explicit network role and limits → self-check → join
→ observe privacy-safe health → update → withdraw gracefully

**Done when:** the Node helps the carrier without reading Application Data,
becoming a Service or User identity, or silently retaining an unbounded duty
after exit.

## Cross-journey failure cases

Every implementation proposal must exercise at least these cases:

- bootstrap information is stale, conflicting, blocked, or malicious;
- one ordinary entry, relay, discovery, or rendezvous Node is malicious, slow,
  or absent;
- one Node modifies, injects, replays, redirects, delays, drops, or tags traffic;
- nominally different Nodes, including both endpoint-adjacent roles, share one
  operator, network, software supply chain, or jurisdiction;
- a Name Record is stale, expired, rolled back, or equivocating;
- a Service Descriptor is unavailable or points to no reachable Service
  Instance;
- both an old and a new host publish with copies of one Service Authority;
- a Service Authority is lost, corrupted, or suspected compromised;
- a Service goes offline before connect, during handshake, or mid-operation;
- a route fails after the Application has written some bytes;
- an Application reuses one Isolation Context across identities or contexts that
  should not be linked;
- an Application creates Services or Isolation Contexts to evade its parent
  resource budget;
- a local Application attempts to exceed connection, bandwidth, or queue limits;
- a slow reader attempts to create unbounded buffering or starve other grants;
- a censor blocks known entry addresses and protocol fingerprints;
- an official endpoint or protocol update channel is compromised or unavailable.
