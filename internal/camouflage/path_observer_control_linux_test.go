//go:build linux

package camouflage

import (
	"net"
	"testing"
)

func TestPathObserverClassifiesOnlyLinkLocalIPv6ControlOutsideDataPaths(t *testing.T) {
	observer := pathObserver{active: true, collecting: true, counts: map[string]int64{},
		unexpected: map[string]int64{}, bindings: map[string]pathTarget{}, bindingCounts: map[string]int64{}}
	packet := make([]byte, 14+40+8)
	packet[12], packet[13], packet[14] = 0x86, 0xdd, 0x60
	packet[14+6] = 58
	copy(packet[14+8:14+24], net.ParseIP("fe80::1").To16())
	copy(packet[14+24:14+40], net.ParseIP("ff02::16").To16())
	observer.observe(packet)
	if observer.external != 0 || observer.packets != 0 {
		t.Fatalf("IPv6 link control changed path evidence: %+v", observer)
	}
	packet[14+6] = 6
	observer.observe(packet)
	if observer.external != 1 {
		t.Fatalf("unknown TCP path was not rejected: %+v", observer)
	}
}
