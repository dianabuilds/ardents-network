package instance

import "time"

// LimitAcceptance shortens this original recipient's local accepting window.
// It changes no signed Descriptor or registration expiry. Repeated calls never
// extend the first bound, and private custody remains retained until Close joins
// every original user. A supplied time cannot create recipient authority.
func (recipient *Recipient) LimitAcceptance(notAfter time.Time) error {
	if recipient == nil || recipient.binding == nil || recipient.binding.root == nil || recipient.registration == nil || notAfter.IsZero() {
		return ErrUnavailable
	}
	root := recipient.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if recipient.closed || recipient.acceptUntil.IsZero() {
		return ErrUnavailable
	}
	if notAfter.Before(recipient.acceptUntil) {
		recipient.acceptUntil = notAfter
	}
	return nil
}
