// Package node owns one original Forwarding, Issuer, Descriptor, Introduction or JOIN invocation: startup acquisitions,
// synchronous denial, physical retirement and retained completion. Network,
// Admission, Hosting, Reachability, Route and processresource retain their separate policies.
//
// The original invocation declaration grammar (plan_declaration.go) is pure
// configuration validation kept with this runtime owner (ADR-0125 decision 6): a
// closed byte codec mirroring network/source/servingdeclaration.ServingDeclaration, shared by the
// runtime, the retained repair commands and the node prepare-plan operator
// command. It returns declaration facts only and opens no resource. It performs
// no State acceptance, root creation, hosting initialization, signing or
// processresource policy. Node validates invocation root separation, and the
// command binds the declared State and selected role constructor. After startup,
// Node supervises the original invocation and releases borrowed authority only
// after physical join. The shared declaration and supervision mechanisms leave
// each role's operational policy with its owner.
package node
