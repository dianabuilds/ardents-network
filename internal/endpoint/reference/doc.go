// Package reference owns the bounded static Reference Site and the
// connection-scoped transparent HTTP/1.1 presentation for an already-
// authenticated Service Target. Its alpha proxy can forward only an explicitly
// registered `.ard` HTTP name to one of those local presentations; it neither
// resolves names, tunnels HTTPS, proxies ordinary URLs, parses Target Links,
// selects a Target, connects to a Service, nor changes browser or system
// configuration.
package reference
