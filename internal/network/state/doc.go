// Package state orchestrates authenticated Network State acceptance and publication.
//
// Readers receive immutable snapshots only after durable publication. Source,
// clock, or resource-governor uncertainty prevents fresh publication. Its
// current bounded private encoding is not a public wire format.
package state
