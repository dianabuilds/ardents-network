---
status: accepted
date: 2026-08-08
---

# Delegate bounded credentials to online Service Instances

Service Authority remains the durable Service Target root but the online V1
runtime generates a private Service Instance Key. The root signs only a public,
bounded, monotonic Service Instance Credential binding that public key, Target,
generation, validity, network, and capabilities. Routine migration creates a
new key and higher generation rather than exporting the old runtime secret. V1
still permits only one active Instance generation. Co-locating the root remains
possible with an explicit custody warning, while hardened operation can keep it
offline. This extra key hierarchy and freshness machinery is accepted because
hostile Service host compromise is normal for Ardents and making every online
compromise a permanent Target compromise would be costly to repair after
deployment.

Copying the public Credential grants no power; Instance Key compromise is bounded
only by credential validity and generation convergence. Service Authority
compromise still requires a new Target. Root backup therefore uses an Authority
Recovery Bundle containing monotonic authority state, and a restored root may
not sign until it reconciles current authenticated state. This is not
multihoming, instant revocation, or a claim that copied secrets can be identified
morally or administratively.

Every Service Connection authenticates the Key/Credential proof but derives
fresh ephemeral traffic keys. Its terminal lifetime is no later than Credential
validity and the applicable Work Safety Lease; learned supersession may stop
leg/recovery work earlier. Best-effort erasure provides Forward Secrecy against
later Service/Node long-term-key compromise for honestly completed connections.
Live endpoint compromise, memory/snapshot remnants, and post-compromise healing
inside that live connection are not covered.

For connections opened from a Service Name/Link, the Name
generation/revision→Target Destination Binding also bounds live work. Learned
Recovery Pending, Release, or rebind to another Target stops new recovery and
closes finitely without retargeting the stream. Explicit Target/Target-Link
connections remain pinned and deliberately receive no Name catastrophe recovery.
