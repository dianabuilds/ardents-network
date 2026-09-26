package node

import "context"

// dutyHandle is the bounded capability handle for one running listener.
type dutyHandle struct {
	Done    <-chan error
	Protect func(bool)
	Usage   func() (uint64, uint64, uint64)
	Stop    func()
	Drain   func(context.Context) error
}
