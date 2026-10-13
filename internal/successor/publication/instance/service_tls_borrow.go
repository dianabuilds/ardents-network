package instance

import (
	"errors"
	"net"
	"sync"
	"time"
)

// TLS cancellation closes only its borrowing transport. The original JOIN
// remains retained until ServiceAttachment.Close has joined every TLS user.
type serviceTLSBorrow struct {
	service  *ServiceAttachment
	once     sync.Once
	closeErr error
}

func (borrow *serviceTLSBorrow) Read(body []byte) (int, error) {
	if err := errors.Join(borrow.service.ctx.Err(), borrow.service.lease.operation.CheckPublisher()); err != nil {
		return 0, err
	}
	n, err := borrow.service.physical.Read(body)
	return n, errors.Join(err, borrow.service.ctx.Err(), borrow.service.lease.operation.CheckPublisher())
}

func (borrow *serviceTLSBorrow) Write(body []byte) (int, error) {
	if err := errors.Join(borrow.service.ctx.Err(), borrow.service.lease.operation.CheckPublisher()); err != nil {
		return 0, err
	}
	n, err := borrow.service.physical.Write(body)
	return n, errors.Join(err, borrow.service.ctx.Err(), borrow.service.lease.operation.CheckPublisher())
}

func (borrow *serviceTLSBorrow) Close() error {
	borrow.once.Do(func() {
		borrow.service.cancel()
		borrow.closeErr = borrow.service.physical.InterruptIO()
	})
	return borrow.closeErr
}

func (borrow *serviceTLSBorrow) LocalAddr() net.Addr  { return borrow.service.physical.LocalAddr() }
func (borrow *serviceTLSBorrow) RemoteAddr() net.Addr { return borrow.service.physical.RemoteAddr() }
func (borrow *serviceTLSBorrow) SetDeadline(end time.Time) error {
	return borrow.service.physical.SetDeadline(end)
}
func (borrow *serviceTLSBorrow) SetReadDeadline(end time.Time) error {
	return borrow.service.physical.SetReadDeadline(end)
}
func (borrow *serviceTLSBorrow) SetWriteDeadline(end time.Time) error {
	return borrow.service.physical.SetWriteDeadline(end)
}
