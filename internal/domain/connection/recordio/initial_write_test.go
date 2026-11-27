package recordio

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

func TestWriteInitialRequestCoalescesOneFlightWithoutRetry(t *testing.T) {
	challenge := wire.Challenge{
		Network: [32]byte{1}, Target: [32]byte{2}, InstanceGeneration: 1,
		Context: [32]byte{3}, Nonce: [32]byte{4},
	}
	continuity, err := wire.NewContinuity([32]byte{5}, wire.RoleInitiator, 1, 0, 0, 0, challenge.Context, [32]byte{6})
	if err != nil {
		t.Fatal(err)
	}
	var expected bytes.Buffer
	for _, record := range []wire.Record{{Challenge: &challenge}, {Continuity: &continuity}} {
		if err := wire.Write(&expected, record); err != nil {
			t.Fatal(err)
		}
	}
	refusal := errors.New("original physical write failed")
	for _, test := range []struct {
		name  string
		n     int
		cause error
	}{
		{"whole-flight", expected.Len(), nil},
		{"short-write", expected.Len() - 1, nil},
		{"error-after-bytes", expected.Len(), refusal},
	} {
		t.Run(test.name, func(t *testing.T) {
			writes := 0
			err := WriteInitialRequest(initialWriter(func(body []byte) (int, error) {
				writes++
				if !bytes.Equal(body, expected.Bytes()) || len(body) != 443 {
					t.Fatal("initial physical write differs from the fixed request", len(body))
				}
				return test.n, test.cause
			}), challenge, continuity)
			if writes != 1 {
				t.Fatal("initial request did not use exactly one physical write", writes)
			}
			if test.name == "whole-flight" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, io.ErrShortWrite) || test.cause != nil && !errors.Is(err, test.cause) {
				t.Fatal("initial write lost the original refusal", err)
			}
		})
	}
}

type initialWriter func([]byte) (int, error)

func (write initialWriter) Write(body []byte) (int, error) { return write(body) }
