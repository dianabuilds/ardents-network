package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
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
	if len(arguments) != 6 || arguments[0] != "run" {
		return errors.New("usage: ardents-stream-app run <role> <socket> <send-seed> <expect-seed> <bytes>")
	}
	sendSeed, err := strconv.Atoi(arguments[3])
	if err != nil {
		return err
	}
	expectSeed, err := strconv.Atoi(arguments[4])
	if err != nil {
		return err
	}
	count, err := strconv.Atoi(arguments[5])
	if err != nil || count < 1 || count > 64<<10 {
		return errors.New("stream byte count is outside its bound")
	}
	connection, err := net.DialTimeout("unix", arguments[2], 5*time.Second)
	if err != nil {
		return err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	result, streamErr := exchange(connection, arguments[1], byte(sendSeed), byte(expectSeed), count)
	classified, resultErr := applicationipc.Read(connection)
	result.ResultClass, result.AuthenticatedTarget = classified.Class, classified.AuthenticatedTarget
	if classified.Class != "clean service connection close" {
		resultErr = errors.Join(resultErr, errors.New("connection result is not semantic success"))
	}
	err = errors.Join(streamErr, resultErr)
	if encodeErr := json.NewEncoder(output).Encode(result); encodeErr != nil {
		return errors.Join(err, encodeErr)
	}
	return err
}

func exchange(connection io.ReadWriter, role string, sendSeed, expectSeed byte, count int) (observation, error) {
	sent, expected := workload(count, sendSeed), workload(count, expectSeed)
	result := observation{Schema: "ardents-h3-stream-application-v1", Role: role,
		SentBytes: uint32(count), SentDigest: sha256.Sum256(sent)}
	type transfer struct {
		value []byte
		err   error
	}
	transfers := make(chan transfer, 2)
	var writers sync.WaitGroup
	writers.Add(1)
	go func() {
		defer writers.Done()
		written := 0
		for written < len(sent) {
			count, writeErr := connection.Write(sent[written:])
			written += count
			if writeErr != nil {
				transfers <- transfer{err: writeErr}
				return
			}
			if count == 0 {
				transfers <- transfer{err: io.ErrShortWrite}
				return
			}
		}
		transfers <- transfer{}
	}()
	go func() {
		value := make([]byte, count)
		_, err := io.ReadFull(connection, value)
		transfers <- transfer{value: value, err: err}
	}()
	first, second := <-transfers, <-transfers
	writers.Wait()
	received := first.value
	if received == nil {
		received = second.value
	}
	err := errors.Join(first.err, second.err)
	result.ReceivedBytes, result.ReceivedDigest = uint32(len(received)), sha256.Sum256(received)
	if err != nil || !bytes.Equal(received, expected) {
		result.Terminal = "error"
		return result, errors.Join(err, errors.New("opaque stream length, order, or bytes differ"))
	}
	result.Terminal = "success"
	return result, nil
}

func workload(count int, seed byte) []byte {
	value := make([]byte, count)
	for index := range value {
		value[index] = byte((index*131 + int(seed)) % 251)
	}
	return value
}
