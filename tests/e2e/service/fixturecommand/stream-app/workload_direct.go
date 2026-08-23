package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// DirectConfig describes one side of a bounded paired carrier baseline.
type DirectConfig struct {
	Role            string
	Address         string
	Seed            [32]byte
	Bytes           int
	Output          io.Writer
	Ready           func(string)
	StartDelay      time.Duration
	MeasureDuration time.Duration
}

const maximumDirectBytes = 256 << 20

// Direct runs one TCP side of a deterministic baseline and completes only
// after both kernels have observed the bounded stream shutdown.
func Direct(ctx context.Context, config DirectConfig) error {
	if ctx == nil || config.Address == "" || config.Bytes < 1 || config.Bytes > maximumDirectBytes || config.Output == nil ||
		config.StartDelay < 0 || config.StartDelay > 5*time.Second ||
		config.MeasureDuration < 0 || config.MeasureDuration > 5*time.Minute {
		return errors.New("direct workload configuration is incomplete or outside its bound")
	}
	switch config.Role {
	case "direct-listen":
		return listenDirect(ctx, config)
	case "direct-connect":
		if config.StartDelay > 0 {
			timer := time.NewTimer(config.StartDelay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
		return connectDirect(ctx, config)
	default:
		return errors.New("direct workload role is invalid")
	}
}

func listenDirect(ctx context.Context, config DirectConfig) error {
	listener, err := net.Listen("tcp", config.Address)
	if err != nil {
		return err
	}
	defer listener.Close()
	if config.Ready != nil {
		config.Ready(listener.Addr().String())
	}
	connection, err := acceptDirect(ctx, listener)
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := setDirectLifetime(ctx, connection); err != nil {
		return err
	}
	if config.MeasureDuration > 0 {
		return receiveTimedDirect(ctx, connection, config)
	}
	result, exchangeErr := Exchange(connection, "direct-server", [32]byte{}, config.Seed, 0, config.Bytes, nil, nil)
	if exchangeErr == nil {
		_, exchangeErr = connection.Write([]byte{1})
	}
	if exchangeErr == nil {
		exchangeErr = awaitDirectHalfClose(connection)
	}
	if exchangeErr != nil {
		result.Terminal = "error"
	}
	return errors.Join(exchangeErr, json.NewEncoder(config.Output).Encode(result))
}

func connectDirect(ctx context.Context, config DirectConfig) error {
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", config.Address)
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := setDirectLifetime(ctx, connection); err != nil {
		return err
	}
	if config.MeasureDuration > 0 {
		return sendTimedDirect(ctx, connection, config)
	}
	result, exchangeErr := Exchange(connection, "direct-client", config.Seed, [32]byte{}, config.Bytes, 0, nil, nil)
	if exchangeErr == nil {
		acknowledgement := []byte{0}
		_, exchangeErr = io.ReadFull(connection, acknowledgement)
		if exchangeErr == nil && acknowledgement[0] != 1 {
			exchangeErr = errors.New("direct workload acknowledgement is invalid")
		}
	}
	if exchangeErr == nil {
		writer, ok := connection.(interface{ CloseWrite() error })
		if !ok {
			exchangeErr = errors.New("direct workload connection cannot half-close")
		} else if exchangeErr = writer.CloseWrite(); exchangeErr == nil {
			exchangeErr = awaitDirectHalfClose(connection)
		}
	}
	if exchangeErr != nil {
		result.Terminal = "error"
	}
	return errors.Join(exchangeErr, json.NewEncoder(config.Output).Encode(result))
}

func awaitDirectHalfClose(connection io.Reader) error {
	var unexpected [1]byte
	count, err := connection.Read(unexpected[:])
	if count == 0 && errors.Is(err, io.EOF) {
		return nil
	}
	return errors.Join(err, errors.New("direct workload completion handshake is invalid"))
}

func setDirectLifetime(ctx context.Context, connection net.Conn) error {
	if deadline, ok := ctx.Deadline(); ok {
		if err := connection.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set direct workload deadline: %w", err)
		}
	}
	if tcp, ok := connection.(*net.TCPConn); ok {
		if err := tcp.SetLinger(10); err != nil {
			return fmt.Errorf("set direct workload close bound: %w", err)
		}
	}
	return nil
}

func acceptDirect(ctx context.Context, listener net.Listener) (net.Conn, error) {
	result := make(chan struct {
		connection net.Conn
		err        error
	}, 1)
	go func() {
		connection, err := listener.Accept()
		result <- struct {
			connection net.Conn
			err        error
		}{connection, err}
	}()
	select {
	case <-ctx.Done():
		_ = listener.Close()
		return nil, ctx.Err()
	case value := <-result:
		return value.connection, value.err
	}
}
