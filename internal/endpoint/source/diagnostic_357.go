//go:build linux

package source

// Temporary process-local ordinal; no persisted or wire identity.
func (acquisition *ResolutionAcquisition) Diagnostic357ID() uint64 {
	if acquisition == nil {
		return 0
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return 0
	}
	prefix := handle.prefix.Load()
	if prefix == nil {
		return 0
	}
	return prefix.Diagnostic357ID()
}
