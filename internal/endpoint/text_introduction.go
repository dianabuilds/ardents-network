//go:build linux

package endpoint

// textIntroduction is the single owner of the context's Introduction
// subsystem: the admitted Introduction prefix lifecycle, the delivery
// dispatch rendezvous, the bounded exchange set, and the cryptographic
// admission window. The zero value is ready for use under textContext.mu.
type textIntroduction struct {
	dispatch  textIntroductionDispatch
	exchanges textIntroductionExchangeSet
	admission textIntroductionAdmission
	prefix    introductionPrefixLifecycle
}
