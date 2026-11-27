package main

import (
	"bytes"
	"crypto/sha256"
	"net"
	"testing"
)

func TestExchangeStreamsLargeDirectionalWorkload(t *testing.T) {
	client, publisher := net.Pipe()
	defer client.Close()
	defer publisher.Close()
	results := make(chan error, 2)
	go func() {
		value, err := Exchange(client, "client", [32]byte{17}, [32]byte{91}, 8<<20, 0, nil, nil)
		if value.Terminal != "success" {
			err = errorsJoin(err, "client did not finish")
		}
		results <- err
	}()
	go func() {
		value, err := Exchange(publisher, "publisher", [32]byte{91}, [32]byte{17}, 0, 8<<20, nil, nil)
		if value.Terminal != "success" || value.ReceivedBytes != 8<<20 {
			err = errorsJoin(err, "publisher did not receive exact bytes")
		}
		results <- err
	}()
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}

func TestPacingWriterUsesBoundedNonRecordAlignedChunks(t *testing.T) {
	var output bytes.Buffer
	write, err := PacingWriter(&output, "1ms")
	if err != nil {
		t.Fatal(err)
	}
	written, err := write(make([]byte, 32<<10))
	if err != nil || written != 16_381 || output.Len() != 16_381 {
		t.Fatalf("paced write=%d retained=%d err=%v", written, output.Len(), err)
	}
	if _, err := PacingWriter(&output, "3001ms"); err == nil {
		t.Fatal("unbounded stream pacing accepted")
	}
}

func errorsJoin(err error, message string) error {
	if err != nil {
		return err
	}
	return &workloadTestError{message}
}

type workloadTestError struct{ message string }

func (err *workloadTestError) Error() string { return err.message }

func TestExchangeRejectsWrongByteAtExpectedLength(t *testing.T) {
	// The first zero-seed byte comes from SHA-256 of the zero seed and
	// counter, independently of Exchange's generator.
	var initialBlock [40]byte
	firstByte := sha256.Sum256(initialBlock[:])[0]
	for _, test := range []struct {
		name string
		data byte
		want string
	}{
		{name: "correct", data: firstByte, want: "success"},
		{name: "wrong byte", data: firstByte ^ 1, want: "error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := Exchange(bytes.NewBuffer([]byte{test.data}), "receiver", [32]byte{}, [32]byte{}, 0, 1, nil, nil)
			if result.ReceivedBytes != 1 || result.Terminal != test.want || (err != nil) != (test.want == "error") {
				t.Fatalf("terminal %q, received %d, error %v", result.Terminal, result.ReceivedBytes, err)
			}
		})
	}
}
