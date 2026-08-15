//go:build !linux

package route

import (
	"errors"
	"net"
	"time"
)

const carrierCloseLingerSeconds = 10

func configureCarrierLiveness(connection net.Conn) error {
	tcp, ok := connection.(*net.TCPConn)
	if !ok {
		return errors.New("carrier connection is not TCP")
	}
	if err := tcp.SetLinger(carrierCloseLingerSeconds); err != nil {
		return err
	}
	return tcp.SetKeepAliveConfig(net.KeepAliveConfig{
		Enable: true, Idle: time.Second, Interval: time.Second, Count: 2,
	})
}
