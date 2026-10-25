//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"errors"
	"fmt"
	"syscall"
)

// A resume of an already exiting original thread can return ESRCH while its
// final wait notification is still pending. This retains the failed request;
// neither ESRCH nor a missing proc path establishes native completion.
type sourceRefusalResumes struct {
	pending map[int]sourceRefusalResume
}

type sourceRefusalResume struct {
	group   int
	exit    int
	request error
}

func (r *sourceRefusalResumes) retain(tid, group, exit int, err error) {
	if r.pending == nil {
		r.pending = map[int]sourceRefusalResume{}
	}
	original, exists := r.pending[tid]
	if !exists {
		original.group, original.exit = group, exit
	}
	original.request = errors.Join(original.request, err)
	r.pending[tid] = original
}

func (r *sourceRefusalResumes) joined(tid, group int, status syscall.WaitStatus) error {
	original := r.pending[tid]
	if original.request == nil {
		return nil
	}
	if group != original.group {
		return errors.Join(original.request, errors.New("original refusal resume group differs"))
	}
	if !status.Exited() && !status.Signaled() {
		return errors.Join(original.request, errors.New("original refusal resume still lacks terminal reap"))
	}
	delete(r.pending, tid)
	if !status.Exited() || status.ExitStatus() != original.exit {
		return errors.Join(original.request, fmt.Errorf("original refusal thread%d joined with status%#x, expected exit%d", tid, uint32(status), original.exit))
	}
	return nil
}

func (r *sourceRefusalResumes) unresolved() (err error) {
	for _, original := range r.pending {
		err = errors.Join(err, original.request)
	}
	return err
}
