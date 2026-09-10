//go:build !linux

package endpoint

// Unsupported platforms cannot retain text authority or installed worker roots.
// Legacy Endpoint ownership checks and shutdown still call this empty seam.
type endpointTextState struct{}

func (*endpointTextState) textPublicationOwned() bool                               { return false }
func (*endpointTextState) configureTextSources(closedEndpointState, string, string) {}
func (*endpointTextState) closeTextContexts() error                                 { return nil }
func (*endpointTextState) closeTextSourceRoots() error                              { return nil }
