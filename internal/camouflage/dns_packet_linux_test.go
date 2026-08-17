//go:build linux

package camouflage

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

const dnsControlPrefix = "ardents-s5-dns-positive-control-v2\x00"

func isDNSPacket(packet []byte) bool {
	offset, protocol, ambiguous, ok := packetTransport(packet)
	return ok && !ambiguous && transportUsesDNS(packet, offset, protocol)
}

func dnsControlClass(packet []byte, allowTCP bool) (byte, string, string) {
	offset, protocol, ambiguous, ok := packetTransport(packet)
	if !ok || ambiguous || !transportUsesDNS(packet, offset, protocol) || len(packet) < offset+4 {
		return 0, "", ""
	}
	source := binary.BigEndian.Uint16(packet[offset : offset+2])
	destination := binary.BigEndian.Uint16(packet[offset+2 : offset+4])
	var payload []byte
	if protocol == 6 {
		if !allowTCP || source != 53 && destination != 53 || len(packet) < offset+20 {
			return 0, "", ""
		}
		header := int(packet[offset+12]>>4) * 4
		if header < 20 || len(packet) < offset+header {
			return 0, "", ""
		}
		payload = packet[offset+header:]
		if len(payload) == 0 && (source == 53 && destination == 53053 || source == 53053 && destination == 53) {
			return 4, "", ""
		}
	} else if protocol == 17 && len(packet) >= offset+8 {
		length := int(binary.BigEndian.Uint16(packet[offset+4 : offset+6]))
		if length < 8 || len(packet) < offset+length {
			return 0, "", ""
		}
		payload = packet[offset+8 : offset+length]
	} else {
		return 0, "", ""
	}
	name, token, ok := decodeDNSControlPayload(payload)
	if !ok {
		return 0, "", ""
	}
	ipOffset := 14
	if len(packet) >= 18 && binary.BigEndian.Uint16(packet[12:14]) == 0x8100 {
		ipOffset = 18
	}
	if len(packet) <= ipOffset {
		return 0, "", ""
	}
	class := byte(0)
	switch {
	case packet[ipOffset]>>4 == 4 && protocol == 17:
		class = 1
	case packet[ipOffset]>>4 == 6 && protocol == 17:
		class = 2
	case packet[ipOffset]>>4 == 4 && protocol == 6:
		class = 3
	}
	return class, name, token
}

func encodeDNSControlPayload(name, token string) []byte {
	return []byte(dnsControlPrefix + token + "\x00" + name)
}

func decodeDNSControlPayload(payload []byte) (string, string, bool) {
	if !bytes.HasPrefix(payload, []byte(dnsControlPrefix)) {
		return "", "", false
	}
	parts := strings.Split(string(payload[len(dnsControlPrefix):]), "\x00")
	if len(parts) != 2 || len(parts[0]) != 32 || parts[1] == "" || len(parts[1]) > 128 {
		return "", "", false
	}
	for _, value := range parts[0] {
		if !strings.ContainsRune("0123456789abcdef", value) {
			return "", "", false
		}
	}
	return parts[1], parts[0], true
}

func TestDNSControlRejectsNamespaceSubstitution(t *testing.T) {
	value := dnsObservation{BoundaryControls: make(map[string]dnsControlObservation)}
	accepted := dnsControlTarget{Name: "E-to-B-front", IfIndex: 7, Token: strings.Repeat("a", 32)}
	if !recordDNSControl(&value, accepted, 1) {
		t.Fatal("manifest-bound control was rejected")
	}
	foreignToken := accepted
	foreignToken.Token = strings.Repeat("b", 32)
	foreignInterface := accepted
	foreignInterface.IfIndex = 8
	if recordDNSControl(&value, foreignToken, 2) || recordDNSControl(&value, foreignInterface, 3) {
		t.Fatal("cross-namespace control substitution was accepted")
	}
}

func packetTransport(packet []byte) (int, byte, bool, bool) {
	if len(packet) < 14 {
		return 0, 0, false, false
	}
	protocol, offset := binary.BigEndian.Uint16(packet[12:14]), 14
	if protocol == 0x8100 && len(packet) >= 18 {
		protocol, offset = binary.BigEndian.Uint16(packet[16:18]), 18
	}
	switch protocol {
	case 0x0800:
		if len(packet) < offset+20 {
			return 0, 0, false, false
		}
		header := int(packet[offset]&0x0f) * 4
		if header < 20 || len(packet) < offset+header {
			return 0, 0, false, false
		}
		fragment := binary.BigEndian.Uint16(packet[offset+6 : offset+8])
		if fragment&0x1fff != 0 {
			return 0, 0, true, true
		}
		return offset + header, packet[offset+9], false, true
	case 0x86dd:
		if len(packet) < offset+40 {
			return 0, 0, false, false
		}
		return ipv6Transport(packet, offset+40, packet[offset+6])
	default:
		return 0, 0, false, false
	}
}

func ipv6Transport(packet []byte, offset int, next byte) (int, byte, bool, bool) {
	for range 8 {
		switch next {
		case 0, 43, 60:
			if len(packet) < offset+2 {
				return 0, 0, false, false
			}
			length := (int(packet[offset+1]) + 1) * 8
			next, offset = packet[offset], offset+length
		case 44:
			if len(packet) < offset+8 {
				return 0, 0, false, false
			}
			if binary.BigEndian.Uint16(packet[offset+2:offset+4])&0xfff8 != 0 {
				return 0, 0, true, true
			}
			next, offset = packet[offset], offset+8
		case 51:
			if len(packet) < offset+2 {
				return 0, 0, false, false
			}
			length := (int(packet[offset+1]) + 2) * 4
			next, offset = packet[offset], offset+length
		case 50:
			return 0, 0, true, true
		default:
			return offset, next, false, len(packet) >= offset
		}
		if len(packet) < offset {
			return 0, 0, false, false
		}
	}
	return 0, 0, true, true
}

func transportUsesDNS(packet []byte, offset int, protocol byte) bool {
	if (protocol != 6 && protocol != 17) || offset < 0 || len(packet) < offset+4 {
		return false
	}
	source := binary.BigEndian.Uint16(packet[offset : offset+2])
	destination := binary.BigEndian.Uint16(packet[offset+2 : offset+4])
	return source == 53 || destination == 53
}
