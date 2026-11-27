// Package store owns one receiving Descriptor history, from exclusive
// custody through durable floor updates and joined retirement. Reachability
// verifies signed proofs and proposes the same conflict transition used by
// lookup history. Route and Node supply fresh authority and the original
// caller. The Store retains its own refusal and physical completion.
//
// The lease and directory-sync adapters support Linux, macOS and Windows.
// A stopped inspection selects the Linux native custody checks. Its
// network_signing_native tests exercise the original protected files.
package store
