// Package terminal encodes and verifies the fixed closed-Route issuance,
// Descriptor, JOIN, and registration operation bodies. It owns their exact
// 4-KiB and 16-KiB sizes, nonce binding, padding, and terminal status grammar.
// Route owns the surrounding lane and transport; this package neither selects
// a Carrier nor grants authority to use a decoded operation.
package terminal
