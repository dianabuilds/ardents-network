//go:build linux

package forwarding

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
	"net"
	"sync"
	"time"
)

type outerTestInnerConn struct {
	outer   carrier.Carrier
	lane    uint32
	mu      sync.Mutex
	inbound []byte
}

func (connection *outerTestInnerConn) Read(value []byte) (int, error) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	for len(connection.inbound) == 0 {
		frame, err := ardp.ReadFrame(connection.outer)
		if err != nil {
			return 0, err
		}
		if frame.Kind == 6 && frame.Lane == connection.lane {
			connection.inbound = append(connection.inbound, frame.Body...)
			continue
		}
		if frame.Kind == 9 && frame.Lane == connection.lane {
			return 0, errors.New("inner lane closed")
		}
	}
	count := copy(value, connection.inbound)
	connection.inbound = connection.inbound[count:]
	return count, nil
}
func (connection *outerTestInnerConn) Write(value []byte) (int, error) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	for rest := value; len(rest) != 0; {
		count := len(rest)
		if count > 16<<10 {
			count = 16 << 10
		}
		if err := ardp.WriteFrame(connection.outer, ardp.Frame{Kind: 6, Lane: connection.lane, Body: rest[:count]}); err != nil {
			return 0, err
		}
		rest = rest[count:]
	}
	return len(value), nil
}
func (connection *outerTestInnerConn) Close() error                     { return nil }
func (connection *outerTestInnerConn) LocalAddr() net.Addr              { return outerTestAddr{} }
func (connection *outerTestInnerConn) RemoteAddr() net.Addr             { return outerTestAddr{} }
func (connection *outerTestInnerConn) SetDeadline(time.Time) error      { return nil }
func (connection *outerTestInnerConn) SetReadDeadline(time.Time) error  { return nil }
func (connection *outerTestInnerConn) SetWriteDeadline(time.Time) error { return nil }

type outerTestAddr struct{}

func (outerTestAddr) Network() string { return "test" }
func (outerTestAddr) String() string  { return "outer-test" }
