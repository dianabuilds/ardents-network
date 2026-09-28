//go:build linux

package endpoint

// tokens is the single owner of the context's token authority: the
// granted permission with its holder key, approved request, per-class
// reservations and stocked closed tokens, plus the single in-flight
// issuance operation slot bound to that permission. The zero value is
// ready for use under textContext.mu.
type tokens struct {
	permission *permission
	issuance   *issuanceOperation
}
