// Package recordio owns bounded physical reads and initial request coalescing
// for Service Connection. It uses the wire grammar and reports bytes and
// transport failures to the authenticator or original Stream actor. Callers
// retain the transport, authority checks, deadlines and join obligations.
package recordio
