//go:build linux

package route

import (
	"errors"
	"net"
	"syscall"
	"time"
)

const carrierUserTimeoutMillis = 2500
const carrierCloseLingerSeconds = 10

func configureCarrierLiveness(connection net.Conn) error {
	tcp, ok := connection.(*net.TCPConn)
	if !ok {
		return errors.New("Carrier connection is not TCP")
	}
	if err := configureCarrierLinger(tcp); err != nil {
		return err
	}
	if err := tcp.SetKeepAliveConfig(net.KeepAliveConfig{
		Enable: true, Idle: time.Second, Interval: time.Second, Count: 2,
	}); err != nil {
		return err
	}
	raw, err := tcp.SyscallConn()
	if err != nil {
		return err
	}
	var optionErr error
	controlErr := raw.Control(func(fd uintptr) {
		optionErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, 18, carrierUserTimeoutMillis)
	})
	return errors.Join(controlErr, optionErr)
}

type carrierLingerSetter interface {
	SetLinger(int) error
}

func configureCarrierLinger(connection carrierLingerSetter) error {
	return connection.SetLinger(carrierCloseLingerSeconds)
}
