package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/streamworkload"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) > 0 && (arguments[0] == "direct-listen" || arguments[0] == "direct-connect") {
		return runDirectCommand(arguments, output)
	}
	if len(arguments) != 7 || arguments[0] != "run" && arguments[0] != "run-short" {
		return errors.New("usage: ardents-stream-app run[-short] <role> <socket> <send-seed-file> <expect-seed-file> <send-bytes> <receive-bytes> | direct-<listen|connect> <address> <seed-file> <bytes>")
	}
	sendSeed, err := readSeed(arguments[3])
	if err != nil {
		return err
	}
	expectSeed, err := readSeed(arguments[4])
	if err != nil {
		return err
	}
	sendCount, receiveCount, err := streamCounts(arguments[5], arguments[6])
	if err != nil {
		return err
	}
	resultPath, err := endpoint.ResultPath(arguments[2])
	if err != nil {
		return err
	}
	resultConnection, err := net.DialTimeout("unix", resultPath, 5*time.Second)
	if err != nil {
		return err
	}
	connection, err := net.DialTimeout("unix", arguments[2], 5*time.Second)
	if err != nil {
		_ = resultConnection.Close()
		return err
	}
	lifetime, err := streamLifetime()
	if err != nil {
		_ = connection.Close()
		_ = resultConnection.Close()
		return err
	}
	if err := connection.SetDeadline(time.Now().Add(lifetime)); err != nil {
		_ = connection.Close()
		_ = resultConnection.Close()
		return fmt.Errorf("bound Application stream lifetime: %w", err)
	}
	if err := resultConnection.SetDeadline(time.Now().Add(lifetime)); err != nil {
		_ = connection.Close()
		_ = resultConnection.Close()
		return fmt.Errorf("bound Application result lifetime: %w", err)
	}
	stream, err := endpoint.OpenApplication(connection, resultConnection)
	if err != nil {
		_ = connection.Close()
		_ = resultConnection.Close()
		return err
	}
	defer stream.Close()
	encoder := json.NewEncoder(output)
	if err := encoder.Encode(map[string]string{"schema": "ardents-stream-ready-v1", "role": arguments[1]}); err != nil {
		return err
	}
	write, err := streamworkload.PacingWriter(stream, os.Getenv("ARDENTS_STREAM_CHUNK_DELAY"))
	if err != nil {
		return err
	}
	progress := func(received uint32) {
		if os.Getenv("ARDENTS_STREAM_PROGRESS") == "1" {
			_ = encoder.Encode(map[string]any{"schema": "ardents-stream-progress-v1", "role": arguments[1],
				"received_bytes": received, "at_unix_nano": time.Now().UnixNano()})
		}
	}
	classifiedResult := waitForResult(stream)
	var result streamworkload.Observation
	var streamErr error
	if arguments[0] == "run-short" {
		if sendCount+receiveCount != 512+(64<<10) {
			return errors.New("short corpus byte counts are not canonical")
		}
		result, streamErr = streamworkload.ExchangeShort(stream, arguments[1], sendSeed, expectSeed, write, progress)
	} else {
		result, streamErr = streamworkload.Exchange(stream, arguments[1], sendSeed, expectSeed,
			sendCount, receiveCount, write, progress)
	}
	classifiedOutcome := <-classifiedResult
	classified, resultErr := classifiedOutcome.result, classifiedOutcome.err
	result.ResultClass, result.AuthenticatedTarget = classified.Class, classified.AuthenticatedTarget
	result.CompletedAtUnixNano = time.Now().UnixNano()
	if classified.Class != "clean service connection close" {
		resultErr = errors.Join(resultErr, errors.New("connection result is not semantic success"))
	}
	err = errors.Join(streamErr, resultErr)
	if encodeErr := encoder.Encode(result); encodeErr != nil {
		return errors.Join(err, encodeErr)
	}
	return err
}
