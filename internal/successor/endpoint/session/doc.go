// Package session owns one serial holder conversation and its original Route,
// issuance and Publication lifetimes. Commands decode and render only. Methods
// check genuine Execution permission independently of command dispatch. Callers
// serialize operations and call Close after the current operation returns.
package session
