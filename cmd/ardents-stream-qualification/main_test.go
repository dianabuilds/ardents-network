package main

import (
	"bytes"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func TestWorkerEntrypointsAcceptOnlyTheirFixedRole(t *testing.T) {
	nonce := [32]byte{1}
	input := bytes.NewBuffer(initialization(streamqualification.ReaderRole, streamqualification.ClientToPublisher, nonce, [32]byte{2}))
	var output bytes.Buffer
	if err := run([]string{"worker-reader"}, input, &output); err == nil {
		t.Fatal("readiness without the complete stream workload passed")
	}
	if actual := output.Bytes(); len(actual) != 40 || string(actual[:8]) != "ARDTQR01" || !bytes.Equal(actual[8:], nonce[:]) {
		t.Fatal("worker did not return the exact fixed readiness acknowledgement")
	}
	if err := run([]string{"worker-reader", "extra"}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("arbitrary worker argument was accepted")
	}
	wrongRole := bytes.NewBuffer(initialization(streamqualification.ReaderRole, streamqualification.ClientToPublisher, nonce, [32]byte{2}))
	if err := run([]string{"worker-publisher"}, wrongRole, &bytes.Buffer{}); err == nil {
		t.Fatal("publisher entrypoint accepted reader initialization")
	}
}

func initialization(role streamqualification.Role, profile streamqualification.Profile, nonce, seed [32]byte) []byte {
	body := append([]byte("ARDTQP01"), byte(role), byte(profile))
	body = append(body, nonce[:]...)
	return append(body, seed[:]...)
}
