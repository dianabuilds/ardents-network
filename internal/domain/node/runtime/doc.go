// Package runtime owns one original Node role, from admission through
// observation, synchronous denial and joined completion. The five role
// constructors retain their own purpose and original physical borrowers.
// Common lifetime code joins them before returning the original producer and
// private TLS material. State, Admission, Hosting and Route retain their rules.
//
// start.go binds the declared role to the original State. The role's own
// source opens its dependencies. lifetime_unix.go owns admission and
// retirement, and invocation.go retains the joined result for the command.
// Credential files keep native pins and private key material inside this package.
//
// Linux is the supported command runtime. Linux and macOS select the Unix
// role composition and file custody, with required process protection checked
// before any accepting role effect. Other hosts select typed unavailability.
// The node_process_native tests require the original Linux process profile.
package runtime
