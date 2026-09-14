//go:build linux

package resource

import (
	"net"
	"testing"
	"time"
)

func TestHostingKernelCountersIncludeBothLoopbackDirections(t *testing.T) {
	// This tests the native counter adapter in its declared network namespace.
	// Loopback is not a provider invoice or whole-system cost qualification.
	before, err := measureHosting([]string{"lo"})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	sender, err := net.Dial("udp4", listener.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	if err := sender.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := sender.Write(make([]byte, 512)); err != nil {
		t.Fatal(err)
	}
	if count, _, err := listener.ReadFrom(make([]byte, 1024)); err != nil || count != 512 {
		t.Fatalf("loopback control: %d / %v", count, err)
	}
	after, err := measureHosting([]string{"lo"})
	if err != nil {
		t.Fatal(err)
	}
	if before.Boot != after.Boot || before.Interfaces[0].Index != after.Interfaces[0].Index ||
		after.Interfaces[0].Tx-before.Interfaces[0].Tx < 512 || after.Interfaces[0].Rx-before.Interfaces[0].Rx < 512 {
		t.Fatalf("native counters omitted a direction: %+v -> %+v", before, after)
	}
}
