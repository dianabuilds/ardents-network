// Package update applies one complete authenticated H3 technical-tracer update
// as an immutable transaction and reconstructs only terminal committed results.
//
// It owns its bounded state root, staging, rollback reservation, activation,
// journal, and cleanup. It does not parse release metadata, mutate release
// floors or Authority state, bootstrap an empty root, install a package, or
// expose intermediate storage operations. It is not a supported installer,
// activator, or Custody surface.
package update
