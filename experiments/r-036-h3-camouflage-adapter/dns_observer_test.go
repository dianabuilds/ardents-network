//go:build ignore

package main

import (
	"encoding/binary"
	"testing"
)

func TestIsDNSPacketIPv4UDP(t *testing.T) {
	packet := ipv4Packet(17, 40123, 53)
	if !isDNSPacket(packet) {
		t.Fatal("UDP destination port 53 was not observed")
	}
}

func TestIsDNSPacketIPv4TCPResponse(t *testing.T) {
	packet := ipv4Packet(6, 53, 40123)
	if !isDNSPacket(packet) {
		t.Fatal("TCP source port 53 was not observed")
	}
}

func TestIsDNSPacketIgnoresOtherTraffic(t *testing.T) {
	if isDNSPacket(ipv4Packet(6, 443, 40123)) {
		t.Fatal("non-DNS traffic was counted")
	}
}

func TestObserverControlIsSeparatedFromCandidateDNS(t *testing.T) {
	control := append(ipv4Packet(17, 40123, 53), observerControlPayload...)
	binary.BigEndian.PutUint16(control[14+20+4:14+20+6], uint16(8+len(observerControlPayload)))
	if !isObserverControl(control, true) {
		t.Fatal("positive-control datagram was not recognized")
	}
	candidate := append(ipv4Packet(17, 40123, 53), []byte("candidate-query")...)
	binary.BigEndian.PutUint16(candidate[14+20+4:14+20+6], uint16(8+len("candidate-query")))
	if isObserverControl(candidate, true) {
		t.Fatal("candidate DNS traffic was mistaken for positive control")
	}
}

func TestIsDNSPacketIPv6UDP(t *testing.T) {
	packet := make([]byte, 14+40+8)
	binary.BigEndian.PutUint16(packet[12:14], 0x86dd)
	packet[14+6] = 17
	binary.BigEndian.PutUint16(packet[14+40:14+42], 40123)
	binary.BigEndian.PutUint16(packet[14+42:14+44], 53)
	if !isDNSPacket(packet) {
		t.Fatal("IPv6 UDP destination port 53 was not observed")
	}
}

func TestIsDNSPacketIPv6ExtensionHeader(t *testing.T) {
	packet := make([]byte, 14+40+8+8)
	binary.BigEndian.PutUint16(packet[12:14], 0x86dd)
	packet[14+6] = 0
	packet[14+40] = 17
	binary.BigEndian.PutUint16(packet[14+48:14+50], 40123)
	binary.BigEndian.PutUint16(packet[14+50:14+52], 53)
	if !isDNSPacket(packet) {
		t.Fatal("IPv6 extension-header DNS packet was not observed")
	}
}

func TestFragmentWithHiddenPortsIsAmbiguous(t *testing.T) {
	packet := ipv4Packet(17, 40123, 53)
	binary.BigEndian.PutUint16(packet[14+6:14+8], 1)
	if !isAmbiguousPacket(packet) {
		t.Fatal("non-initial fragment did not fail closed")
	}
}

func TestTCPControlTupleIsSeparated(t *testing.T) {
	packet := ipv4Packet(6, 53053, 53)
	if !isObserverControl(packet, true) {
		t.Fatal("TCP positive-control tuple was not recognized")
	}
	if isObserverControl(packet, false) {
		t.Fatal("TCP control tuple remained exempt after the control phase")
	}
}

func ipv4Packet(protocol byte, source, destination uint16) []byte {
	packet := make([]byte, 14+20+8)
	binary.BigEndian.PutUint16(packet[12:14], 0x0800)
	packet[14] = 0x45
	packet[14+9] = protocol
	binary.BigEndian.PutUint16(packet[14+20:14+22], source)
	binary.BigEndian.PutUint16(packet[14+22:14+24], destination)
	return packet
}
