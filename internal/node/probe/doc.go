// Package probe owns the private role-probe TLS listener, its bounded work,
// nonce replay memory and joined shutdown. Node supplies one authenticated duty
// and supervises the returned handle; this package does not select a duty.
package probe
