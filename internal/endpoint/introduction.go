//go:build linux

package endpoint

// introduction is the single owner of the context's Introduction
// subsystem: the admitted Introduction prefix lifecycle, the delivery
// dispatch rendezvous, the bounded exchange set, and the cryptographic
// admission window. The zero value is ready for use under textContext.mu.
type introduction struct {
	dispatch  introductionDispatch
	exchanges introductionExchangeSet
	admission introductionAdmission
	prefix    introductionPrefixLifecycle
}
