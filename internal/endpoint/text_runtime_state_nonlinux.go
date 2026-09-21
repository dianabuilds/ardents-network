//go:build !linux

package endpoint

// Unsupported platforms cannot retain text authority or installed worker roots.
type endpointTextState struct{}

func (*endpointTextState) textPublicationOwned() bool  { return false }
func (*endpointTextState) closeTextContexts() error    { return nil }
func (*endpointTextState) closeTextSourceRoots() error { return nil }
