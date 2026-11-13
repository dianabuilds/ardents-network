package systemd

import "context"

// Each fixed-purpose owner retains its own original connection and one-attempt
// state. This shared physical mechanism chooses no unit or Installation policy.
func (r *managerReference) reloadOriginal(attempted *bool) error {
	return r.performOriginal(func(ctx context.Context, r *managerReference) error {
		if err := r.admitOriginalReload(attempted); err != nil {
			return err
		}
		_, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.Reload")
		return err
	})
}

// Physical I/O is serialized by the caller. This short local gate orders an
// attempted reload against Close and retains uncertainty before sending bytes.
func (r *managerReference) admitOriginalReload(attempted *bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() {
		return ErrUnavailable
	}
	if r.terminal != nil {
		return r.terminal
	}
	if err := r.original.Err(); err != nil {
		return err
	}
	if *attempted {
		return ErrObservation
	}
	*attempted = true
	return nil
}
