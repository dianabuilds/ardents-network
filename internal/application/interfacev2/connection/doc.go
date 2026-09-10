// Package connection owns the closed text-Service version-2 local Connection
// contract: AAI3 requests, bounded ordered streams and one joined terminal
// outcome. It accepts explicit Target Links and refuses the reserved Name tag.
//
// Request admission and stream cleanup belong to this version together. AAI2
// retains its independently versioned compatibility owner; neither decoder
// silently accepts the other grammar. Shared client lifecycle changes require
// the corresponding cancellation/close regression scenarios in both owners.
package connection
