// Command netem-relay is a qualification-only bounded TCP or UDP relay.
// It applies one fixed Linux netem rule to its own container interface before
// forwarding bounded traffic to the exact supplied upstream.
package main
