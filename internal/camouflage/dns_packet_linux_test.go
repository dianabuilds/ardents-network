//go:build linux

package camouflage

import (
	"bytes"
	"encoding/binary"
)

var dnsControlPayload = []byte("ardents-s5-dns-positive-control")

func isDNSPacket(packet []byte) bool {
	offset, protocol, ambiguous, ok := packetTransport(packet)
	return ok && !ambiguous && transportUsesDNS(packet, offset, protocol)
}

func isDNSControl(packet []byte, allowTCP bool) bool {
	offset, protocol, ambiguous, ok := packetTransport(packet)
	if !ok || ambiguous || !transportUsesDNS(packet, offset, protocol) || len(packet) < offset+4 {
		return false
	}
	source := binary.BigEndian.Uint16(packet[offset : offset+2])
	destination := binary.BigEndian.Uint16(packet[offset+2 : offset+4])
	if protocol == 6 {
		return allowTCP && ((source == 53 && destination == 53053) ||
			(source == 53053 && destination == 53))
	}
	if protocol != 17 || len(packet) < offset+8 {
		return false
	}
	length := int(binary.BigEndian.Uint16(packet[offset+4 : offset+6]))
	return length >= 8 && len(packet) >= offset+length &&
		bytes.Equal(packet[offset+8:offset+length], dnsControlPayload)
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
