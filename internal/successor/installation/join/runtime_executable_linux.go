package join

import (
	"errors"
	"os"
	"sync"
	"syscall"
)

// runtimeExecutableView owns a read-only mapping of this process's actual
// executable only. It is not a copied image or an accepting verdict: every
// comparison rereads all mapped bytes, and the original descriptor still
// supplies exact EOF and native custody checks. Linux refuses writable opens
// of an executing image; mutable foreign files must never take this path.
// The mapping is private to runtimeFiles and is unmapped only after its
// original physical users join. No slice crosses that lifetime boundary.
type runtimeExecutableView struct {
	original *initialFile
	body     []byte
	once     sync.Once
	failure  error
}

// A different executing inode leaves the ordinary full descriptor reader in
// place. This matters for independent file-only observers: they prove neither
// that this program is executing nor any Startup/process/manager authority.
// The caller retains a partial result before handling an error and closing it.
func openRuntimeExecutableView(original *initialFile) (result *runtimeExecutableView, returned error) {
	if original == nil || original.file == nil || original.info == nil || len(original.body) == 0 || len(original.body) > 64<<20 {
		return nil, ErrInput
	}
	actual, err := original.file.Stat()
	if err != nil || !os.SameFile(original.info, actual) || actual.Size() != int64(len(original.body)) {
		return nil, errors.Join(ErrBinding, err)
	}
	self, err := os.Open("/proc/self/exe")
	if err != nil {
		return nil, err
	}
	defer func() {
		returned = errors.Join(returned, self.Close())
		if returned != nil && result != nil {
			returned = errors.Join(returned, result.close())
		}
	}()
	current, err := self.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(actual, current) {
		return nil, nil
	}
	// Use the freshly opened kernel self reference, not a numeric fd that an
	// external descriptor-loss control could close and reassign between calls.
	body, err := syscall.Mmap(int(self.Fd()), 0, len(original.body), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, err
	}
	return &runtimeExecutableView{original: original, body: body}, nil
}

func (view *runtimeExecutableView) close() error {
	if view == nil {
		return nil
	}
	view.once.Do(func() {
		view.failure = syscall.Munmap(view.body)
		if view.failure == nil {
			view.body = nil
		}
	})
	return view.failure
}
