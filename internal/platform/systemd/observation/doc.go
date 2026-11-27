// Package observation decodes detached, typed systemd properties and compares
// them without losing their signatures, precision or missing fields. It is
// portable and has no connection, caller lifetime or native authority.
// Platform systemd produces these facts. Endpoint unit policy, Execution and
// State recovery check the facts against their own admitted requirements.
package observation
