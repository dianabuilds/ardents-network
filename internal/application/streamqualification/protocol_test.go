package streamqualification

import (
	"bytes"
	"testing"
)

func TestInitializationRoundTripRejectsSubstitution(t *testing.T) {
	init := Init{Role: ReaderRole, Profile: ClientToPublisher, Nonce: [32]byte{1}, Seed: [32]byte{2}}
	wire := bytes.NewBuffer(encodeInitialization(init))
	actual, err := ReadInit(wire, ReaderRole)
	if err != nil || actual != init {
		t.Fatalf("initialization: %#v / %v", actual, err)
	}
	if _, err := ReadInit(bytes.NewReader([]byte(initMagic)), ReaderRole); err == nil {
		t.Fatal("truncated initialization was accepted")
	}
	if _, err := ReadInit(bytes.NewReader(append([]byte(initMagic), append([]byte{byte(PublisherRole), byte(ClientToPublisher)}, make([]byte, 64)...)...)), ReaderRole); err == nil {
		t.Fatal("wrong worker role was accepted")
	}
}

func TestReadyReturnsExactNonce(t *testing.T) {
	var wire bytes.Buffer
	nonce := [32]byte{3}
	if err := WriteReady(&wire, nonce); err != nil {
		t.Fatal(err)
	}
	if actual := wire.Bytes(); len(actual) != readyBytes || string(actual[:len(readyMagic)]) != readyMagic || !bytes.Equal(actual[len(readyMagic):], nonce[:]) {
		t.Fatal("readiness did not contain the exact nonce")
	}
}

func encodeInitialization(init Init) []byte {
	body := make([]byte, initBytes)
	copy(body, initMagic)
	body[len(initMagic)] = byte(init.Role)
	body[len(initMagic)+1] = byte(init.Profile)
	copy(body[len(initMagic)+2:], init.Nonce[:])
	copy(body[len(initMagic)+2+len(init.Nonce):], init.Seed[:])
	return body
}

func TestProfilesRetainFullOpenSets(t *testing.T) {
	for _, role := range []Role{ReaderRole, PublisherRole} {
		for _, profile := range []Profile{ClientToPublisher, PublisherToClient} {
			schedule, err := profile.Definition(role)
			if err != nil || schedule.ActiveConnections == 0 || schedule.ActiveConnections > schedule.OpenConnections {
				t.Fatalf("profile %d role %d: %#v / %v", profile, role, schedule, err)
			}
		}
	}
}
