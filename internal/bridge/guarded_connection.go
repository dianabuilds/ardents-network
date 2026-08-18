package bridge

import (
	"errors"
	"net"
	"time"
)

var errCarrierIneligible = errors.New("bridge carrier is no longer eligible")

// guardedConnection prevents a Route caller from extending an authenticated
// Bridge bound after the Adapter has opened its carriage.
type guardedConnection struct {
	net.Conn
	deadline  time.Time
	clock     func() time.Time
	confident func() bool
}

func (connection *guardedConnection) Read(value []byte) (int, error) {
	if err := connection.check(); err != nil {
		return 0, err
	}
	return connection.Conn.Read(value)
}

func (connection *guardedConnection) Write(value []byte) (int, error) {
	if err := connection.check(); err != nil {
		return 0, err
	}
	return connection.Conn.Write(value)
}

func (connection *guardedConnection) SetDeadline(value time.Time) error {
	return connection.Conn.SetDeadline(connection.bound(value))
}

func (connection *guardedConnection) SetReadDeadline(value time.Time) error {
	return connection.Conn.SetReadDeadline(connection.bound(value))
}

func (connection *guardedConnection) SetWriteDeadline(value time.Time) error {
	return connection.Conn.SetWriteDeadline(connection.bound(value))
}

func (connection *guardedConnection) bound(value time.Time) time.Time {
	if value.IsZero() || connection.deadline.Before(value) {
		return connection.deadline
	}
	return value
}

func (connection *guardedConnection) check() error {
	if connection.confident() && connection.clock().Before(connection.deadline) {
		return nil
	}
	_ = connection.Conn.Close()
	return errCarrierIneligible
}
