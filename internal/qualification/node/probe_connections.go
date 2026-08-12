package node

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"
)

func slowNodeProbe(address string, deadline time.Time) error {
	connection, err := (&net.Dialer{Deadline: deadline}).Dial("tcp", address)
	if err != nil {
		return err
	}
	defer connection.Close()
	_ = connection.SetDeadline(deadline)
	if _, err := connection.Write([]byte{0x16}); err != nil {
		return err
	}
	var response [1]byte
	_, err = connection.Read(response[:])
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		return errors.New("slow pre-authentication peer exceeded its deadline")
	}
	if err == nil {
		return errors.New("slow pre-authentication peer received application data")
	}
	return nil
}

func partialNodeProbe(address string, config *tls.Config, deadline time.Time) error {
	connection, err := tls.DialWithDialer(&net.Dialer{Deadline: deadline}, "tcp", address, config)
	if err != nil {
		return errors.New("authorized partial-frame client could not authenticate")
	}
	defer connection.Close()
	_ = connection.SetDeadline(deadline)
	if _, err := connection.Write([]byte("ARNP")); err != nil {
		return err
	}
	var response [1]byte
	_, err = connection.Read(response[:])
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		return errors.New("authorized partial-frame work exceeded its deadline")
	}
	if err == nil {
		return errors.New("authorized partial frame returned a success byte")
	}
	return nil
}

func floodNodeProbe(ctx context.Context, address string, config *tls.Config) error {
	var work sync.WaitGroup
	success := make(chan net.Conn, 32)
	for range 32 {
		work.Add(1)
		go func() {
			defer work.Done()
			connection, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp", address, config.Clone())
			if err == nil {
				success <- connection
			}
		}()
	}
	work.Wait()
	close(success)
	authenticated := len(success)
	for connection := range success {
		_ = connection.Close()
	}
	if authenticated == 0 || authenticated > 16 {
		return errors.New("node connection flood violated the 16-open internal cap")
	}
	return nil
}
