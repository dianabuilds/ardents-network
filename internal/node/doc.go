// Package node composes one authenticated Node process: it validates local
// reservations, selects a State duty, supervises the selected role, reacts to
// process pressure, and joins terminal results. Closed forwarding, issuer,
// resolution, introduction and JOIN duties own their listeners, admitted
// children and durable roots in role packages. Node opens shared Hosting
// handles and transfers each handle's close to its selected duty.
//
// process_config.go and closed_reservations.go declare local configuration;
// admission.go validates State-selected duties; lifecycle.go owns process
// transitions; duty_server.go dispatches roles. The authority child borrows
// current State and verifies selected-profile tokens; hosting bounds shared
// reservations and class-2 reserve-before-spend policy. The outer child owns
// one accepted Carrier's inner lanes, writer and joined cleanup. Node retains
// the private probe and class-1/3 control envelope. duty_handle.go names the
// process stop/drain boundary; lifecycle_event.go and event_writer.go publish
// observations. Retired native duties remain unavailable.
package node
