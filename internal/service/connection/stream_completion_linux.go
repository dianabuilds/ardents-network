//go:build linux

package connection

// Done closes after all native data and terminal-control workers have stopped
// and the current Attachment has been released. RunBounded may return earlier
// after a successful Application outcome while its bounded control tail lives.
// The selected recovery-capable text-Service process is Linux-only.
func (stream *Stream) Done() <-chan struct{} {
	if stream == nil {
		return nil
	}
	return stream.done
}
