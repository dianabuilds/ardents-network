// Package connection owns the closed text-Service version-2 local Connection
// contract: AAI3 requests, bounded ordered streams and one joined terminal
// outcome. It accepts explicit Target Links and refuses the reserved Name tag.
//
// Request admission and stream cleanup belong to this version together. The
// retired AAI2 magic is recognized only for refusal before Interface.Open; it
// does not select another Connection implementation. Changes to this owner
// require admission, cancellation, half-close and terminal-result regression
// evidence.
package connection
