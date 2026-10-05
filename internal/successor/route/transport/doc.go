// Package transport owns protected Route channel execution. It consumes fresh
// Network observations and transferred Admission grants, authenticates exact
// role peers, and retains physical reservations until its borrowers have joined.
// Exact context openings own Source/Responder acquisitions, receiving JOIN
// pairing and bounded framed streams independently of retained selection.
// It supplies no Service, publication, Instance or Application authority.
package transport
