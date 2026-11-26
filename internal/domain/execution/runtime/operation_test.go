package runtime

import (
	"context"
	"testing"
)

func TestDetachedOperationCannotSupplyPermission(t *testing.T) {
	for _, operation := range []*Operation{nil, {}} {
		if operation.Check() == nil {
			t.Fatal("detached operation supplied permission")
		}
	}
}

// This cancellation/retirement fixture supplies no qualified Job or Grant.
func TestCopiedOperationCannotCompleteOriginalBorrow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := &Operation{ctx: ctx, caller: ctx, cancel: cancel, stop: func() bool { return true }, done: make(chan struct{})}
	original.self = original
	copy := &Operation{self: original.self, ctx: original.ctx, caller: original.caller, cancel: original.cancel, stop: original.stop, done: original.done}
	copy.Close()
	if ctx.Err() != nil {
		t.Fatal("copied operation canceled the original borrower")
	}
	select {
	case <-original.done:
		t.Fatal("copied operation reported the original use joined")
	default:
	}
	original.Close()
	select {
	case <-original.done:
	default:
		t.Fatal("original operation did not complete its own use")
	}
}
