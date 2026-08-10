package tooling

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNativeCaptureFilterUsesResolvedAddresses(t *testing.T) {
	got := nativeCaptureFilter([]string{"172.20.0.2", "fd00::2"})
	want := []string{"host", "172.20.0.2", "or", "host", "fd00::2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("capture filter = %v, want %v", got, want)
	}
}

func TestNativeCaptureAllRequiresOneIsolatedLink(t *testing.T) {
	config := nativeToolConfig{
		SchemaVersion: nativeToolRoleSchema, RunID: "run", Role: "capture-user", Mode: "capture",
		Links: []nativeCaptureLink{{Name: "direct-link", Peer: "service", CaptureAll: true}},
	}
	path := filepath.Join(t.TempDir(), "tool.json")
	write := func() {
		data, err := json.Marshal(config)
		if err != nil || os.WriteFile(path, data, 0o600) != nil {
			t.Fatal("write native capture fixture")
		}
	}
	write()
	if _, err := readNativeToolConfig(path); err != nil {
		t.Fatalf("single isolated capture rejected: %v", err)
	}
	config.Links = append(config.Links, nativeCaptureLink{Name: "second-link", Peer: "other"})
	write()
	if _, err := readNativeToolConfig(path); err == nil {
		t.Fatal("unfiltered multi-link capture was accepted")
	}
}

func TestNativeCaptureCountsOriginalWireBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "link.pcap")
	data := make([]byte, 24+16+3+16+2)
	copy(data[:4], []byte{0xd4, 0xc3, 0xb2, 0xa1})
	binary.LittleEndian.PutUint32(data[24+8:], 3)
	binary.LittleEndian.PutUint32(data[24+12:], 1500)
	second := 24 + 16 + 3
	binary.LittleEndian.PutUint32(data[second+8:], 2)
	binary.LittleEndian.PutUint32(data[second+12:], 60)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := pcapWireBytes(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1560 {
		t.Fatalf("wire bytes = %d, want 1560", got)
	}
}

func TestNativeCaptureRejectsTruncatedPacketPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "truncated.pcap")
	data := make([]byte, 24+16+2)
	copy(data[:4], []byte{0xd4, 0xc3, 0xb2, 0xa1})
	binary.LittleEndian.PutUint32(data[24+8:], 3)
	binary.LittleEndian.PutUint32(data[24+12:], 60)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := pcapWireBytes(path); err == nil {
		t.Fatal("truncated packet payload was accepted")
	}
}
