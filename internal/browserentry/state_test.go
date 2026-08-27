package browserentry

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestPublisherRetainsOnlyItsCurrentLoopbackProxyPort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser-entry.json")
	publisher, err := OpenPublisher(path)
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Close()
	if err := publisher.Publish(43123); err != nil {
		t.Fatal(err)
	}
	state, err := readState(path)
	if err != nil {
		t.Fatal(err)
	}
	probeCapability, probeDecodeErr := base64.RawStdEncoding.DecodeString(state.ProbeCapability)
	proxyCredential, credentialDecodeErr := base64.RawStdEncoding.DecodeString(state.ProxyCredential)
	if state.Port != 43123 || probeDecodeErr != nil || credentialDecodeErr != nil || string(probeCapability) != string(publisher.probeCapability[:]) || string(proxyCredential) != string(publisher.proxyCredential[:]) {
		t.Fatalf("browser Entry state = %+v, want current loopback port and separate current credentials", state)
	}
	if err := publisher.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := readState(path); err == nil {
		t.Fatal("cleared Browser Entry state remained readable")
	}
}

func TestReadStateRejectsPriorSingleCapabilitySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser-entry.json")
	if err := os.WriteFile(path, []byte(`{"schema":"ardents-browser-entry-state-v1","port":43123,"capability":"AA"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readState(path); err == nil {
		t.Fatal("prior Browser Entry state schema was accepted")
	}
}
