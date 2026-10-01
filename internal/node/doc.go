// Package node composes one authenticated Node process: it validates local
// reservations, selects a State duty, supervises the selected role, reacts to
// process pressure, and joins terminal results. Closed forwarding, issuer,
// resolution, introduction and JOIN duties own their listeners, admitted
// children and durable roots in role packages. Node opens shared Hosting
// handles through the Hosting owner and transfers each handle's close to its
// selected duty.
//
// process_config.go declares shared process configuration; each role adapter
// declares its own local profile. admission.go validates State-selected duties;
// lifecycle.go owns process transitions; duty_server.go dispatches roles and
// holds their supervision surface. The authority child borrows
// current State and verifies selected-profile tokens; hosting bounds shared
// reservations and class-2 reserve-before-spend policy. The outer child owns
// one accepted Carrier's inner lanes, writer and joined cleanup. The probe
// child owns its private TLS listener, work and joined drain. Hosting owns
// reservation policy for all admitted classes; lifecycle_event.go publishes
// observations through platform writers. Retired native duties remain unavailable.
package node
