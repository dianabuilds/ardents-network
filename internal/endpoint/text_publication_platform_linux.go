//go:build linux

package endpoint

// textContext adds installed Publisher startup and drain state to local authority.
// Like the surrounding context state, both fields are protected by owner.mu.
type textContext struct {
	textContextState
	publicationStarting bool
	publicationDrain    chan struct{}
}

// Only the explicit admission stop permits a normal producer drain. A joined
// cancellation, delivery failure or cleanup error still aborts the publication.
func onlyTextPublicationDraining(err error) bool {
	if err == errTextPublicationDraining {
		return true
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) == 0 {
		return false
	}
	for _, cause := range joined.Unwrap() {
		if !onlyTextPublicationDraining(cause) {
			return false
		}
	}
	return true
}
