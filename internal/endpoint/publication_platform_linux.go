//go:build linux

package endpoint

// textContext is the local authority holder. Every local subsystem, including
// the publication owner with its startup and drain barriers, lives inside
// textContextState and is protected by owner.mu unless an owner documents its
// own mutex.
type textContext struct {
	textContextState
}

// Only the explicit admission stop permits a normal producer drain. A joined
// cancellation, delivery failure or cleanup error still aborts the publication.
func onlyPublicationDraining(err error) bool {
	if err == errPublicationDraining {
		return true
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) == 0 {
		return false
	}
	for _, cause := range joined.Unwrap() {
		if !onlyPublicationDraining(cause) {
			return false
		}
	}
	return true
}
