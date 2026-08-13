package servicenegative

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
)

type observedConnection struct {
	net.Conn
	once          sync.Once
	entered       chan struct{}
	threshold     uint32
	read          atomic.Uint32
	thresholdOnce sync.Once
	reached       chan struct{}
	gate          *gatedWriter
}

func (connection *observedConnection) Read(output []byte) (int, error) {
	connection.once.Do(func() {
		if connection.entered != nil {
			close(connection.entered)
		}
	})
	count, err := connection.Conn.Read(output)
	if connection.threshold > 0 && connection.read.Add(uint32(count)) >= connection.threshold {
		connection.thresholdOnce.Do(func() { close(connection.reached) })
	}
	return count, err
}

func (connection *observedConnection) Write(input []byte) (int, error) {
	connection.once.Do(func() {
		if connection.entered != nil {
			close(connection.entered)
		}
	})
	if connection.gate != nil {
		connection.gate.blocked.Store(true)
	}
	return connection.Conn.Write(input)
}

type gatedWriter struct {
	net.Conn
	ctx     context.Context
	blocked atomic.Bool
}

func (connection *gatedWriter) Write(input []byte) (int, error) {
	if !connection.blocked.Load() {
		return connection.Conn.Write(input)
	}
	<-connection.ctx.Done()
	return 0, connection.ctx.Err()
}
