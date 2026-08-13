package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/applicationipc"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) != 7 || arguments[0] != "run" {
		return errors.New("usage: ardents-stream-app run <role> <socket> <send-seed-file> <expect-seed-file> <send-bytes> <receive-bytes>")
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
	connection, err := net.DialTimeout("unix", arguments[2], 5*time.Second)
	if err != nil {
		return err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	write, err := workloadWriter(connection, os.Getenv("ARDENTS_STREAM_CHUNK_DELAY"))
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	gateOffset, gate, err := progressGate(arguments[1])
	if err != nil {
		return err
	}
	if gateOffset > 0 && sendCount > 0 {
		write = gatedWorkloadWriter(write, gateOffset, gate)
	}
	progress := func(received uint32) {
		if os.Getenv("ARDENTS_STREAM_PROGRESS") == "1" {
			_ = encoder.Encode(map[string]any{"schema": "ardents-h3-stream-progress-v1", "role": arguments[1],
				"received_bytes": received})
		}
	}
	result, streamErr := exchange(connection, arguments[1], sendSeed, expectSeed, sendCount, receiveCount, write, progress)
	classified, resultErr := applicationipc.Read(connection)
	result.ResultClass, result.AuthenticatedTarget = classified.Class, classified.AuthenticatedTarget
	if classified.Class != "clean service connection close" {
		resultErr = errors.Join(resultErr, errors.New("connection result is not semantic success"))
	}
	err = errors.Join(streamErr, resultErr)
	if encodeErr := encoder.Encode(result); encodeErr != nil {
		return errors.Join(err, encodeErr)
	}
	return err
}

func streamCounts(sendText, receiveText string) (int, int, error) {
	send64, sendErr := strconv.ParseInt(sendText, 10, 32)
	receive64, receiveErr := strconv.ParseInt(receiveText, 10, 32)
	if sendErr != nil || receiveErr != nil || send64 < 0 || receive64 < 0 || send64 > 4<<20 || receive64 > 4<<20 ||
		(send64 == 0 && receive64 == 0) {
		return 0, 0, errors.New("stream byte counts are outside their bound")
	}
	return int(send64), int(receive64), nil
}
