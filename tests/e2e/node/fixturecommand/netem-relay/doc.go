// Command netem-relay is an native Rendezvous qualification-only transparent TCP relay.
// It applies one fixed Linux netem rule to its own container interface before
// forwarding a bounded test connection to the exact supplied upstream.
package main
