//go:build linux

package runtime

import (
	"errors"
	"sync"
)

// workerBorrow adapts the retained attachment to a bridge while rechecking the
// exact original operation on every I/O. Possession grants no authority, and
// Close captures final authority before physical retirement and retains the
// first cleanup failure. The Stream qualification and Text reader bridges share
// this one mechanism but keep distinct per-role operation check hooks and their
// own unavailable prose. Sharing the mechanism merges no authority: a borrowed
// attachment never supplies the operation's permission.
type workerBorrow struct {
	invocation  *Invocation
	unavailable string
	check       func(*Invocation, *Operation) error
	once        sync.Once
	closeErr    error
}

func (borrow *workerBorrow) checkOperation() error {
	invocation := borrow.invocation
	invocation.useMu.Lock()
	operation := invocation.operation
	invocation.useMu.Unlock()
	if operation == nil || operation.invocation != invocation {
		return errors.New(borrow.unavailable)
	}
	return borrow.check(invocation, operation)
}

func (borrow *workerBorrow) Read(body []byte) (int, error) {
	if err := borrow.invocation.job.Check(); err != nil {
		return 0, err
	}
	n, err := borrow.invocation.activation.Attachment.Read(body)
	return n, errors.Join(err, borrow.checkOperation())
}

func (borrow *workerBorrow) Write(body []byte) (int, error) {
	if err := borrow.checkOperation(); err != nil {
		return 0, err
	}
	n, err := borrow.invocation.activation.Attachment.Write(body)
	return n, errors.Join(err, borrow.checkOperation())
}

func (borrow *workerBorrow) Close() error {
	borrow.once.Do(func() {
		authority := borrow.checkOperation()
		physical := borrow.invocation.activation.Attachment.Close()
		borrow.closeErr = errors.Join(authority, physical)
		borrow.invocation.useMu.Lock()
		operation := borrow.invocation.operation
		borrow.invocation.useMu.Unlock()
		if operation != nil {
			operation.RetainCleanup(physical)
		}
	})
	return borrow.closeErr
}

// streamOperationCheck rechecks the exact original Stream qualification
// operation. It is the qualification bridge's distinct check hook.
func streamOperationCheck(_ *Invocation, operation *Operation) error {
	return operation.Check()
}

// readerOperationCheck rechecks the original reader caller and the exact original
// ordinary Reader operation. It is the Text reader bridge's distinct check hook;
// it consults the reader caller, which the Stream qualification check never does.
func readerOperationCheck(invocation *Invocation, operation *Operation) error {
	return errors.Join(invocation.readerCaller.Err(), operation.CheckReader())
}
