//go:build !linux

package route

import (
	"errors"
	"net"
	"time"
)

func configureCarrierLiveness(connection net.Conn) error {
	tcp, ok := connection.(*net.TCPConn)
	if !ok {
		return errors.New("carrier connection is not TCP")
	}
	return tcp.SetKeepAliveConfig(net.KeepAliveConfig{
		Enable: true, Idle: time.Second, Interval: time.Second, Count: 2,
	})
}
