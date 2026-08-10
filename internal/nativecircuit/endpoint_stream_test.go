package nativecircuit

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestJoinedRouteTimedUploadIsVerifiedAtBothEndpoints(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	certificate, trust := newEndpointFixture(t, "active-instance")
	user, service := net.Pipe()
	nonce := randomHandle(t)
	spec := streamSpec{Direction: streamUpload, Seed: "timed-seed", Duration: 30 * time.Millisecond}
	serviceResult := make(chan endpointObservation, 1)
	serviceError := make(chan error, 1)
	go func() {
		observation, err := runEndpointServiceStream(ctx, service, certificate, nonce, spec)
		serviceResult <- observation
		serviceError <- err
	}()
	userObservation, err := runEndpointUserStream(ctx, user, trust, nonce, spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	serviceObservation := <-serviceResult
	if err := <-serviceError; err != nil {
		t.Fatal(err)
	}
	if !userObservation.ApplicationBytesVerified || !serviceObservation.ApplicationBytesVerified || userObservation.ApplicationBytes == 0 || userObservation.ApplicationBytes != serviceObservation.ApplicationBytes {
		t.Fatalf("user = %+v, service = %+v", userObservation, serviceObservation)
	}
}

func TestTimedStreamVerifiesSeededBytes(t *testing.T) {
	t.Parallel()
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	spec := streamSpec{Direction: streamUpload, Seed: "fixed-seed", Duration: 25 * time.Millisecond}
	received := make(chan streamResult, 1)
	errors := make(chan error, 1)
	go func() {
		result, err := receiveTimedStream(context.Background(), right, spec)
		received <- result
		errors <- err
	}()
	sent, err := sendTimedStream(context.Background(), left, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
	got := <-received
	if sent.Bytes == 0 || got.Bytes != sent.Bytes || got.Digest != sent.Digest {
		t.Fatalf("sent = %+v, received = %+v", sent, got)
	}
}

func TestTimedStreamRejectsWrongSeed(t *testing.T) {
	t.Parallel()
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	errors := make(chan error, 1)
	go func() {
		_, err := receiveTimedStream(context.Background(), right, streamSpec{
			Direction: streamUpload, Seed: "wrong-seed", Duration: 20 * time.Millisecond,
		})
		errors <- err
	}()
	if err := writeFrame(left, frame{Type: frameProtectedData, Payload: seededStreamChunk("right-seed", 0)}); err != nil {
		t.Fatal(err)
	}
	if err := <-errors; err == nil {
		t.Fatal("receiver accepted bytes from the wrong deterministic seed")
	}
}

func TestStreamSpecRejectsNonQualificationDuration(t *testing.T) {
	t.Parallel()
	if err := validateStreamSpec(streamSpec{Direction: streamUpload, Seed: "seed", Duration: time.Second}, true); err == nil {
		t.Fatal("qualification stream accepted a duration other than 60 seconds")
	}
	if err := validateStreamSpec(streamSpec{Direction: streamDownload, Seed: "seed", Duration: 60 * time.Second}, true); err != nil {
		t.Fatal(err)
	}
}
