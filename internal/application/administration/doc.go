// Package administration owns version 1 of the separately authorized local
// Publish/Withdraw Interface. It exposes neither connection bytes nor State,
// Entry, Target, Route, Credential, Custody, Service key, or Network types.
// publish_withdraw.go defines the operations and outcomes. publication_request.go
// matches requests with their successful outcomes, and local_request_exchange.go
// handles the bounded Unix socket exchange.
package administration
