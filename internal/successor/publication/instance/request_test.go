package instance

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
)

// External Authority issuance is fixture input, not a maintained signing API.
func fixtureResponse(request RequestView, generation uint64, caps uint32) []byte {
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, 32))
	public := authority.Public().(ed25519.PublicKey)
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public...))
	raw := append([]byte{0, 3}, public...)
	raw = append(raw, target[:]...)
	raw = append(raw, request.InstancePublic[:]...)
	for _, value := range []uint64{generation, uint64(request.NotBefore), uint64(request.NotAfter)} {
		raw = binary.BigEndian.AppendUint64(raw, value)
	}
	raw = append(raw, request.NetworkID[:]...)
	raw = binary.BigEndian.AppendUint32(raw, caps)
	raw = append(raw, ed25519.Sign(authority, raw)...)
	response := append([]byte("ardents-service-instance-response-v3\x00"), request.Commitment[:]...)
	return append(response, raw...)
}

func TestHostGeneratedRequestAndIndependentGrammar(t *testing.T) {
	value, err := generateState([32]byte{3}, 1900000000, 1900003600)
	if err != nil {
		t.Fatal(err)
	}
	defer value.erase()
	view := value.request
	independent := append([]byte("ardents-service-instance-request-v3\x00"), view.NetworkID[:]...)
	independent = append(independent, view.InstancePublic[:]...)
	independent = binary.BigEndian.AppendUint64(independent, 1900000000)
	independent = binary.BigEndian.AppendUint64(independent, 1900003600)
	commitment := sha256.Sum256(independent)
	independent = append(independent, commitment[:]...)
	if !bytes.Equal(independent, encodeRequest(view)) || commitment != view.Commitment {
		t.Fatal("canonical public request changed")
	}
	parsed, err := ParseRequest(independent)
	if err != nil || parsed != view {
		t.Fatal(parsed, err)
	}
	second, err := generateState(view.NetworkID, view.NotBefore, view.NotAfter)
	if err != nil {
		t.Fatal(err)
	}
	defer second.erase()
	if second.request.InstancePublic == view.InstancePublic {
		t.Fatal("host factory reused key")
	}
	for _, offset := range []int{0, len(requestDomain), len(independent) - 1} {
		bad := append([]byte(nil), independent...)
		bad[offset] ^= 1
		if _, err := ParseRequest(bad); err == nil {
			t.Fatal("mutated request accepted", offset)
		}
	}
	if _, ok := any(&Root{}).(crypto.Signer); ok {
		t.Fatal("Instance exposes generic signer")
	}
	rootType := reflect.TypeOf(Root{})
	for i := 0; i < rootType.NumField(); i++ {
		if rootType.Field(i).IsExported() {
			t.Fatal("mutable Root field exported")
		}
	}
}

func TestSignedResponseExactRequestBinding(t *testing.T) {
	value, err := generateState([32]byte{3}, 1900000000, 1900003600)
	if err != nil {
		t.Fatal(err)
	}
	defer value.erase()
	valid := fixtureResponse(value.request, 7, 3)
	credential, err := verifyResponse(valid, value.request)
	if err != nil || credential.Delegation().Generation != 7 || credential.Delegation().Instance != value.request.InstancePublic {
		t.Fatal(credential, err)
	}
	for _, mode := range []string{"commitment", "network", "instance", "time", "capability", "signature", "generation", "length", "overflow"} {
		t.Run(mode, func(t *testing.T) {
			request := value.request
			generation, caps := uint64(7), uint32(3)
			want := ErrUnavailable
			switch mode {
			case "commitment":
				request.Commitment[0] ^= 1
			case "network":
				request.NetworkID[0] ^= 1
			case "instance":
				request.InstancePublic[0] ^= 1
			case "time":
				request.NotAfter++
			case "capability":
				caps = 7
			case "generation":
				generation = 0
				want = ErrInvalid
			case "signature", "length", "overflow":
				want = ErrInvalid
			}
			raw := fixtureResponse(request, generation, caps)
			switch mode {
			case "signature":
				raw[len(raw)-1] ^= 1
			case "length":
				raw = append(raw, 0)
			case "overflow":
				binary.BigEndian.PutUint64(raw[len(responseDomain)+32+106:], 1<<63)
			}
			if _, err := verifyResponse(raw, value.request); !errors.Is(err, want) {
				t.Fatalf("response %s: %v", mode, err)
			}
		})
	}
}

func TestStateRejectsPrivateSeedAndNoncanonicalJSON(t *testing.T) {
	value, err := generateState([32]byte{3}, 1900000000, 1900003600)
	if err != nil {
		t.Fatal(err)
	}
	defer value.erase()
	raw, err := marshalState(value)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	copyOut, err := unmarshalState(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer copyOut.erase()
	copyOut.private[0] ^= 1
	if err = copyOut.validate(); err == nil {
		t.Fatal("different seed with original public suffix accepted")
	}
	for _, bad := range [][]byte{append([]byte(" "), raw...), bytes.Replace(raw, []byte(stateSchema), []byte("ardents-service-instance-root-v2"), 1), bytes.Replace(raw, []byte("{\"schema\":"), []byte("{\"unknown\":true,\"schema\":"), 1)} {
		if got, err := unmarshalState(bad); err == nil {
			got.erase()
			t.Fatal("noncanonical or retired state accepted")
		}
	}
}
