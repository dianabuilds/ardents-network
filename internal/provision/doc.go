// Package provision initializes protected Node state, configuration, identity,
// and first-Operator enrollment material. It also owns the strict stopped
// local-v2 adapter that reads protected legacy state and holds the shared
// manager fence while constructing authority migration evidence. It does not
// own normal runtime lifecycle, migration truth, or recurring credential
// administration.
package provision
