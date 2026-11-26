// Package connection owns the local Connection contract for the protected
// Endpoint. AAI3 selects the document profile and AAI4 selects the bidirectional
// byte-stream profile. Both use explicit Target Links, bounded ordered streams,
// directional EOF and one joined terminal outcome. Name remains reserved.
//
// The retired AAI2 magic is recognized for refusal before Interface.Open.
// Changes to this owner require admission, cancellation, half-close and
// terminal-result regression evidence.
//
// The pure typed refusal constructor Refuse (server_refusal.go) is portable;
// the refusal setupCleanupError classifier and the server admission stay with
// the protected Linux Endpoint (#885).
package connection
