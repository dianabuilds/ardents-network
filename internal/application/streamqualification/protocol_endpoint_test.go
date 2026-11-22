package streamqualification

import (
	"bytes"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestEndpointInitializationInteroperatesWithFixedWorker(t *testing.T) {
	for _, role := range []Role{ReaderRole, PublisherRole} {
		for _, profile := range []Profile{ClientToPublisher, PublisherToClient} {
			t.Run(strconv.Itoa(int(role))+"/"+strconv.Itoa(int(profile)), func(t *testing.T) {
				endpoint, worker := net.Pipe()
				defer endpoint.Close()
				defer worker.Close()
				deadline := time.Now().Add(3 * time.Second)
				if err := endpoint.SetDeadline(deadline); err != nil {
					t.Fatal(err)
				}
				if err := worker.SetDeadline(deadline); err != nil {
					t.Fatal(err)
				}
				init := Init{Role: role, Profile: profile, Nonce: [32]byte{1}, Seed: [32]byte{2}}
				type workerResult struct {
					init Init
					err  error
				}
				finished := make(chan workerResult, 1)
				go func() {
					actual, err := ReadInit(worker, role)
					if err == nil {
						err = WriteReady(worker, actual.Nonce)
					}
					finished <- workerResult{init: actual, err: err}
				}()
				if err := InitializeWorker(endpoint, init); err != nil {
					t.Fatal(err)
				}
				result := <-finished
				if result.err != nil || result.init != init {
					t.Fatalf("fixed worker exchange: %#v / %v", result.init, result.err)
				}
			})
		}
	}
}

func TestEndpointInitializationRefusesInvalidInvocationBeforeOutput(t *testing.T) {
	for _, test := range []struct {
		name       string
		attachment io.ReadWriter
		init       Init
	}{
		{"absent attachment", nil, Init{Role: ReaderRole, Profile: ClientToPublisher, Nonce: [32]byte{1}, Seed: [32]byte{2}}},
		{"zero nonce", &bytes.Buffer{}, Init{Role: ReaderRole, Profile: ClientToPublisher, Seed: [32]byte{2}}},
		{"zero seed", &bytes.Buffer{}, Init{Role: ReaderRole, Profile: ClientToPublisher, Nonce: [32]byte{1}}},
		{"invalid role", &bytes.Buffer{}, Init{Role: Role(0), Profile: ClientToPublisher, Nonce: [32]byte{1}, Seed: [32]byte{2}}},
		{"invalid profile", &bytes.Buffer{}, Init{Role: ReaderRole, Profile: Profile(0), Nonce: [32]byte{1}, Seed: [32]byte{2}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := InitializeWorker(test.attachment, test.init); err == nil {
				t.Fatal("invalid invocation was accepted")
			}
			if buffer, ok := test.attachment.(*bytes.Buffer); ok && buffer.Len() != 0 {
				t.Fatal("invalid invocation emitted initialization bytes")
			}
		})
	}
}

func TestEndpointInitializationRejectsForeignOrIncompleteReadiness(t *testing.T) {
	nonce := [32]byte{1}
	valid := append([]byte(readyMagic), nonce[:]...)
	for _, test := range []struct {
		name       string
		readiness  []byte
		closeAfter bool
	}{
		{"foreign magic", append([]byte("WRONG!!!"), nonce[:]...), false},
		{"foreign nonce", append([]byte(readyMagic), make([]byte, 32)...), false},
		{"truncated readiness", valid[:len(valid)-1], true},
	} {
		t.Run(test.name, func(t *testing.T) {
			endpoint, worker := net.Pipe()
			defer endpoint.Close()
			defer worker.Close()
			deadline := time.Now().Add(3 * time.Second)
			if err := endpoint.SetDeadline(deadline); err != nil {
				t.Fatal(err)
			}
			if err := worker.SetDeadline(deadline); err != nil {
				t.Fatal(err)
			}
			init := Init{Role: ReaderRole, Profile: ClientToPublisher, Nonce: nonce, Seed: [32]byte{2}}
			go func() {
				if _, err := ReadInit(worker, ReaderRole); err != nil {
					return
				}
				_, _ = worker.Write(test.readiness)
				if test.closeAfter {
					_ = worker.Close()
				}
			}()
			if err := InitializeWorker(endpoint, init); err == nil {
				t.Fatal("foreign or incomplete readiness was accepted")
			}
		})
	}
}
