package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/streamworkload"
)

const directSchema = "ardents-h3-s43-direct-application-v1"

func runDirect(arguments []string, output io.Writer) error {
	if len(arguments) != 6 || arguments[0] != "direct" {
		return errors.New("usage: ardents-stream-app direct <listen|connect> <send|receive> <address> <seed-file> <bytes>")
	}
	operation, direction, address := arguments[1], arguments[2], arguments[3]
	seed, err := readSeed(arguments[4])
	count, countErr := strconv.ParseUint(arguments[5], 10, 32)
	if err != nil || countErr != nil || count == 0 || count > 256<<20 ||
		(operation != "listen" && operation != "connect") ||
		(direction != "send" && direction != "receive") {
		return errors.Join(err, countErr, errors.New("direct stream configuration is invalid"))
	}
	encoder := json.NewEncoder(output)
	connection, closeListener, err := openDirect(operation, address, encoder)
	if err != nil {
		return err
	}
	defer closeListener()
	defer connection.Close()
	role := "direct-" + operation
	if err := waitWorkloadRelease(role); err != nil {
		return err
	}
	if err := connection.SetDeadline(time.Now().Add(15 * time.Minute)); err != nil {
		return err
	}
	send, receive := 0, int(count)
	if direction == "send" {
		send, receive = int(count), 0
	}
	progress := func(received uint32) {
		_ = encoder.Encode(map[string]any{"schema": "ardents-h3-s43-direct-progress-v1",
			"role": role, "received_bytes": received})
	}
	value, transferErr := streamworkload.Exchange(connection, role, seed, seed, send, receive, nil, progress)
	result := struct {
		Schema, Operation, Direction string
		Observation                  streamworkload.Observation
	}{Schema: directSchema, Operation: operation, Direction: direction, Observation: value}
	return errors.Join(transferErr, encoder.Encode(result))
}

func openDirect(operation, address string, encoder *json.Encoder) (net.Conn, func() error, error) {
	if operation == "connect" {
		connection, err := net.DialTimeout("tcp", address, 30*time.Second)
		return connection, func() error { return nil }, err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, func() error { return nil }, err
	}
	if err := encoder.Encode(map[string]string{"schema": directSchema, "kind": "ready"}); err != nil {
		_ = listener.Close()
		return nil, func() error { return nil }, err
	}
	if tcp, ok := listener.(*net.TCPListener); ok {
		_ = tcp.SetDeadline(time.Now().Add(30 * time.Second))
	}
	connection, err := listener.Accept()
	if err != nil {
		_ = listener.Close()
		return nil, func() error { return nil }, fmt.Errorf("accept direct baseline: %w", err)
	}
	return connection, listener.Close, nil
}
