// Package provisioning dispatches offline Network artifact commands and
// produces the bounded initial State/Node key inventory. It renders public
// receipts after the original operation completes.
//
// Start with network_commands.go for the Network command. networkdraft owns
// public declarations and unsigned bytes, networkpreparation owns original
// public input and exclusive unsigned output, and networksigning owns original
// protected inputs and fixed-purpose signed output. Initial key birth stays
// with initial_network_keys_linux.go. The native command tests join these
// operations through real file custody and signed State acceptance.
//
// The separate infrastructure package owns the fixed-role provision command,
// and sourcegenesis supplies its Source cohort. runtimeplan writes runtime
// declarations under the existing Node codecs. Network State, runtime Node
// and Route each admit their own effects independently.
package provisioning
