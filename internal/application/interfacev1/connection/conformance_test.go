package connection

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestVersionedConformanceVectors(t *testing.T) {
	t.Parallel()
	var vector struct {
		Schema           string `json:"schema"`
		InterfaceVersion string `json:"interface_version"`
		Request          struct {
			ServiceLink string `json:"service_link"`
			WireHex     string `json:"wire_hex"`
		} `json:"request"`
		Terminal struct {
			Class   OutcomeClass `json:"class"`
			Reason  string       `json:"reason"`
			WireHex string       `json:"wire_hex"`
		} `json:"terminal"`
	}
	raw, err := os.ReadFile("testdata/conformance-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	if vector.Schema != "ardents-application-interface-conformance-v1" || vector.InterfaceVersion != InterfaceVersion {
		t.Fatalf("Connection vector version = %q / %q", vector.Schema, vector.InterfaceVersion)
	}
	request := make([]byte, len(localMagic)+2+len(vector.Request.ServiceLink))
	copy(request, localMagic)
	binary.BigEndian.PutUint16(request[len(localMagic):], uint16(len(vector.Request.ServiceLink)))
	copy(request[len(localMagic)+2:], vector.Request.ServiceLink)
	assertConformanceHex(t, "request", request, vector.Request.WireHex)
	var terminal bytes.Buffer
	if err := writeTerminal(&terminal, Outcome{Class: vector.Terminal.Class, Reason: vector.Terminal.Reason}); err != nil {
		t.Fatal(err)
	}
	assertConformanceHex(t, "terminal", terminal.Bytes(), vector.Terminal.WireHex)
}

func assertConformanceHex(t *testing.T, kind string, actual []byte, expected string) {
	t.Helper()
	want, err := hex.DecodeString(expected)
	if err != nil || !bytes.Equal(actual, want) {
		t.Errorf("%s wire = %x, decode error %v; want %s", kind, actual, err, expected)
	}
}
