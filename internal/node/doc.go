// Package node owns one authenticated Node duty from admission through terminal
// cleanup. Closed forwarding startup transfers its spend ledger, duty limits,
// and bootstrap controller only as one fully initialized receiving-resource
// owner. The server owns accepted producers, pool interruption, that resource
// owner, and the host; its session set separately owns outgoing Carrier readers,
// joins them after all producers, and retains their terminal cleanup result.
// The retired native Rendezvous, Initiator, Responder, Introduction, and
// Transit-issuance engines are absent; their plan stanzas remain only at the
// command refusal boundary. Current closed receivers and their shared Carrier
// mechanics remain distinct duties.
//
// process_config.go and closed_reservations.go declare local configuration;
// admission.go validates State-selected duties; lifecycle.go owns process
// transitions; duty_server.go dispatches listeners. The closed_* listener
// files retain each duty's resources, while closed_outer_* owns shared Carrier
// lifetime. closed_role_token_verification.go authenticates the same selected
// profile token for forwarding and control admission; closed_control_admission.go
// owns the control host reservation, while forwarding owns its own reservation
// and spend. duty_handle.go names the common stop/drain boundary;
// lifecycle_event.go and event_writer.go define and emit observations.
package node
