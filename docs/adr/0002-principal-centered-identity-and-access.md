# Principal-centered identity and grant-based access

Status: proposed

Ardents will represent every portable actor as a self-certifying `Principal`, authenticate it with replaceable Credentials, and derive contextual roles such as Operator from signed, resource-scoped Access Grants. Node, human, Application, and realm-authority Principals share this identity primitive; Waku Peer IDs, resource identifiers, and display names remain separate. Root Ed25519 keys establish Principal identity, delegated device keys handle routine authentication, Node-bound and interface-bound short-lived sessions carry successful authentication, and an Application acting for another Principal must present an explicit attenuated Delegation. Existing Operator and Application bearer tokens remain only as explicit, time-bounded migration schemes; the target retains at most separate one-use bootstrap and narrowly scoped break-glass mechanisms, none of which become portable Principals.

One deep `identity/access` owner will implement authentication, sessions, Node-issued Access Grants, revocation, and request admission behind narrow consumer-owned interfaces. Product Policy remains outside that owner. Normal sessions require a root-signed device Credential; the offline root participates only in typed enrollment proof. Clients reconstruct typed, domain-separated challenges instead of signing opaque server-selected bytes, and sessions are bound to the server-derived transport peer as well as Node/interface. Version 1 permits exact actions, typed resource scopes, and at most one non-redelegable Principal-to-Application Delegation; it does not implement a general role or grant-chain engine. The full-length `p1_` identifier is a domain-separated SHA-256 digest of the canonical Ed25519 root public key.

## Considered Options

- A mandatory X.509/realm-CA account model was rejected because it makes identity existence depend on one authority and couples product identity to transport certificates. Realm authorities may still attest membership or attributes, and mTLS may still protect a remote transport.
- Public-key login with permissions embedded in certificates or sessions was rejected because key rotation, permission revocation, and role changes would become coupled to authentication.
- Reusing the private-channel `CapabilityGrant` for all authorization was rejected because that type carries channel secrets and Waku permissions; an Access Grant is a separate signed statement with resource/action semantics.
- Signing every RPC was rejected for the first version because it complicates Connect streaming and canonical request encoding. Short-lived audience-bound sessions are accepted only over the private local transports or a future explicitly authenticated remote transport.
- Arbitrary nested grant/delegation chains were rejected for version 1 because the required use case is a single explicit user-to-Application handoff. General chains would add cycle, ancestry, revocation, and explanation complexity without a current product requirement.

## Consequences

Principal identity, authentication, authorization, and product policy remain separate checks. The same Principal can hold independent grants on several Nodes, while Applications require their own Node-issued authority, the effective Principal's authority, and any Delegation needed to act for that Principal. A new full-length versioned Principal identifier must replace the current truncated `p_` form through a coordinated migration before user identities are issued. Root-key rotation changes Principal in version 1; identity recovery requires a later explicit decision.
