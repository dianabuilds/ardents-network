package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"sync"
	"time"
)

func exchange(connection io.ReadWriter, role string, sendSeed, expectSeed [32]byte, sendCount, receiveCount int,
	write func([]byte) (int, error), progress func(uint32)) (observation, error) {
	sent, expected := workload(sendCount, sendSeed), workload(receiveCount, expectSeed)
	result := observation{Schema: "ardents-h3-stream-application-v1", Role: role, SendSeed: sendSeed, ExpectSeed: expectSeed,
		SentBytes: uint32(sendCount), SentDigest: sha256.Sum256(sent)}
	type transfer struct {
		value []byte
		err   error
	}
	transfers := make(chan transfer, 2)
	var writes sync.WaitGroup
	writes.Add(1)
	go func() {
		defer writes.Done()
		written := 0
		for written < len(sent) {
			if write == nil {
				write = connection.Write
			}
			count, writeErr := write(sent[written:])
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
		value := make([]byte, receiveCount)
		read := 0
		var err error
		for read < len(value) {
			want := min(16_381, len(value)-read)
			var count int
			count, err = io.ReadFull(connection, value[read:read+want])
			read += count
			if count > 0 && progress != nil {
				progress(uint32(read))
			}
			if err != nil {
				break
			}
		}
		transfers <- transfer{value: value[:read], err: err}
	}()
	first, second := <-transfers, <-transfers
	writes.Wait()
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

func workloadWriter(destination io.Writer, encodedDelay string) (func([]byte) (int, error), error) {
	if encodedDelay == "" {
		return destination.Write, nil
	}
	delay, err := time.ParseDuration(encodedDelay)
	if err != nil || delay <= 0 || delay > 100*time.Millisecond {
		return nil, errors.New("stream chunk delay is outside its bound")
	}
	return func(value []byte) (int, error) {
		if len(value) > 16_381 {
			value = value[:16_381]
		}
		written, writeErr := destination.Write(value)
		if written > 0 {
			time.Sleep(delay)
		}
		return written, writeErr
	}, nil
}
