// Package transfer owns the ordered byte and Terminal state of one logical
// Service Connection across its authenticated Attachments. It tracks the
// original accepted, delivered and exposed byte ranges and the control receipt
// chain for each generation. The Connection serializes all calls and owns
// authority, actual record I/O and physical joins.
package transfer
