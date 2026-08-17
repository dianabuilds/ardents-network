//go:build linux && live

package network_test

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type blockedDNSControlTarget struct {
	Name    string `json:"name"`
	IfIndex int    `json:"ifindex"`
	Token   string `json:"token"`
}

var blockedExpectedDNSControls map[string]blockedDNSControlTarget

func runBlockedControlPlan(t *testing.T, root string, manifest blockedPathManifest) {
	t.Helper()
	targets := blockedDNSControlTargets(t, manifest)
	writeBlockedJSON(t, filepath.Join(root, "control-targets.json"), targets)
	blockedExpectedDNSControls = make(map[string]blockedDNSControlTarget, len(targets))
	for _, target := range targets {
		blockedExpectedDNSControls[target.Name] = target
		sendBlockedInterfaceControls(t, target)
		writeBlockedSignal(t, filepath.Join(root, "control-done"))
		waitBlockedFile(t, filepath.Join(root, "control-stopped"), 3*time.Second)
		for _, signal := range []string{"control-done", "control-stopped"} {
			if err := os.Remove(filepath.Join(root, signal)); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeBlockedSignal(t, filepath.Join(root, "controls-complete"))
	waitBlockedFile(t, filepath.Join(root, "controls-stopped"), 3*time.Second)
}

func loadBlockedControlTargets(t *testing.T, root string) {
	t.Helper()
	var targets []blockedDNSControlTarget
	readBlockedJSON(t, filepath.Join(root, "control-targets.json"), &targets)
	blockedExpectedDNSControls = make(map[string]blockedDNSControlTarget, len(targets))
	for _, target := range targets {
		if target.Name == "" || target.IfIndex <= 0 || len(target.Token) != 32 ||
			blockedExpectedDNSControls[target.Name].Name != "" {
			t.Fatal("external DNS control targets are invalid")
		}
		blockedExpectedDNSControls[target.Name] = target
	}
}

func completeBlockedDNSObservation(value blockedDNSResult, manifest blockedPathManifest) bool {
	if len(blockedExpectedDNSControls) != len(manifest.Required)+len(manifest.ControlOnly)+
		len(manifest.DynamicLoopback)+len(manifest.ControlLoopback) ||
		len(value.BoundaryControls) != len(blockedExpectedDNSControls) {
		return false
	}
	for _, target := range blockedExpectedDNSControls {
		control, exists := value.BoundaryControls[target.Name]
		if !exists || control.IfIndex != target.IfIndex || control.Token != target.Token || control.IPv4UDP < 2 ||
			control.IPv6UDP < 2 || control.IPv4TCP < 2 {
			return false
		}
	}
	return value.Controls >= int64(6*len(blockedExpectedDNSControls)) &&
		value.IPv4UDPControls >= int64(2*len(blockedExpectedDNSControls)) &&
		value.IPv6UDPControls >= int64(2*len(blockedExpectedDNSControls)) &&
		value.IPv4TCPControls >= int64(2*len(blockedExpectedDNSControls))
}

func blockedDNSControlTargets(t *testing.T, manifest blockedPathManifest) []blockedDNSControlTarget {
	t.Helper()
	targets, ok := blockedDNSControlTargetsWithoutTest(manifest)
	if !ok {
		t.Fatal("DNS control manifest cannot be bound to exact interfaces")
	}
	return targets
}

func blockedDNSControlTargetsWithoutTest(manifest blockedPathManifest) ([]blockedDNSControlTarget, bool) {
	boundaries := append(append([]blockedPathBoundary(nil), manifest.Required...), manifest.ControlOnly...)
	loopbacks := append(append([]string(nil), manifest.DynamicLoopback...), manifest.ControlLoopback...)
	result := make([]blockedDNSControlTarget, 0, len(boundaries)+len(loopbacks))
	seen := make(map[string]bool)
	for _, boundary := range boundaries {
		index := blockedInterfaceForAddresses(boundary.Source, boundary.Address)
		token, tokenErr := blockedDNSControlToken()
		if boundary.Name == "" || index <= 0 || seen[boundary.Name] || tokenErr != nil {
			return nil, false
		}
		seen[boundary.Name] = true
		result = append(result, blockedDNSControlTarget{Name: boundary.Name, IfIndex: index, Token: token})
	}
	loopback, err := net.InterfaceByName("lo")
	if err != nil {
		return nil, false
	}
	for _, name := range loopbacks {
		token, tokenErr := blockedDNSControlToken()
		if name == "" || seen[name] || tokenErr != nil {
			return nil, false
		}
		seen[name] = true
		result = append(result, blockedDNSControlTarget{Name: name, IfIndex: loopback.Index, Token: token})
	}
	return result, len(result) > 0
}

func blockedInterfaceForAddresses(values ...string) int {
	interfaces, err := net.Interfaces()
	if err != nil {
		return 0
	}
	for _, value := range values {
		wanted := net.ParseIP(value)
		if wanted == nil {
			continue
		}
		for _, current := range interfaces {
			addresses, addressErr := current.Addrs()
			if addressErr != nil {
				return 0
			}
			for _, address := range addresses {
				ip, _, parseErr := net.ParseCIDR(address.String())
				if parseErr == nil && ip.Equal(wanted) {
					return current.Index
				}
			}
		}
	}
	return 0
}

func sendBlockedInterfaceControls(t *testing.T, control blockedDNSControlTarget) {
	t.Helper()
	observerInterface, err := net.InterfaceByIndex(control.IfIndex)
	if err != nil {
		t.Fatal(err)
	}
	mac := observerInterface.HardwareAddr
	if len(mac) != 6 {
		mac = net.HardwareAddr{0x02, 0, 0, 0, 0, 1}
	}
	payload := []byte("ardents-s5-dns-positive-control-v2\x00" + control.Token + "\x00" + control.Name)
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(blockedNetworkShort(0x0003)))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	target := &syscall.SockaddrLinklayer{Protocol: blockedNetworkShort(0x0003), Ifindex: control.IfIndex}
	for range 2 {
		for _, frame := range [][]byte{blockedIPv4UDPControl(payload, mac), blockedIPv6UDPControl(payload, mac),
			blockedIPv4TCPControl(payload, mac)} {
			if err := syscall.Sendto(fd, frame, 0, target); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func blockedIPv4UDPControl(payload []byte, mac net.HardwareAddr) []byte {
	frame := blockedEthernetFrame(0x0800, 20+8+len(payload), mac)
	ip := frame[14:]
	ip[0], ip[8], ip[9] = 0x45, 64, 17
	binary.BigEndian.PutUint16(ip[2:4], uint16(len(ip)))
	copy(ip[12:16], []byte{192, 0, 2, 1})
	copy(ip[16:20], []byte{192, 0, 2, 2})
	binary.BigEndian.PutUint16(ip[10:12], blockedChecksum(ip[:20]))
	udp := ip[20:]
	binary.BigEndian.PutUint16(udp[0:2], 53053)
	binary.BigEndian.PutUint16(udp[2:4], 53)
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
	copy(udp[8:], payload)
	pseudo := append(append([]byte{}, ip[12:20]...), 0, 17, byte(len(udp)>>8), byte(len(udp)))
	blockedPutChecksum(udp[6:8], pseudo, udp)
	return frame
}

func blockedIPv6UDPControl(payload []byte, mac net.HardwareAddr) []byte {
	frame := blockedEthernetFrame(0x86dd, 40+8+len(payload), mac)
	ip := frame[14:]
	ip[0], ip[6], ip[7] = 0x60, 17, 64
	binary.BigEndian.PutUint16(ip[4:6], uint16(8+len(payload)))
	copy(ip[8:24], net.ParseIP("2001:db8::1").To16())
	copy(ip[24:40], net.ParseIP("2001:db8::2").To16())
	udp := ip[40:]
	binary.BigEndian.PutUint16(udp[0:2], 53053)
	binary.BigEndian.PutUint16(udp[2:4], 53)
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
	copy(udp[8:], payload)
	pseudo := make([]byte, 40)
	copy(pseudo[0:32], ip[8:40])
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(udp)))
	pseudo[39] = 17
	blockedPutChecksum(udp[6:8], pseudo, udp)
	return frame
}

func blockedIPv4TCPControl(payload []byte, mac net.HardwareAddr) []byte {
	frame := blockedEthernetFrame(0x0800, 40+len(payload), mac)
	ip := frame[14:]
	ip[0], ip[8], ip[9] = 0x45, 64, 6
	binary.BigEndian.PutUint16(ip[2:4], uint16(len(ip)))
	copy(ip[12:16], []byte{192, 0, 2, 1})
	copy(ip[16:20], []byte{192, 0, 2, 2})
	binary.BigEndian.PutUint16(ip[10:12], blockedChecksum(ip[:20]))
	tcp := ip[20:]
	binary.BigEndian.PutUint16(tcp[0:2], 53053)
	binary.BigEndian.PutUint16(tcp[2:4], 53)
	tcp[12], tcp[13] = 0x50, 0x02
	copy(tcp[20:], payload)
	pseudo := append(append([]byte{}, ip[12:20]...), 0, 6, byte(len(tcp)>>8), byte(len(tcp)))
	blockedPutChecksum(tcp[16:18], pseudo, tcp)
	return frame
}

func blockedEthernetFrame(protocol uint16, payload int, mac net.HardwareAddr) []byte {
	frame := make([]byte, 14+payload)
	copy(frame[0:6], mac)
	copy(frame[6:12], mac)
	binary.BigEndian.PutUint16(frame[12:14], protocol)
	return frame
}

func blockedDNSControlToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func blockedChecksum(parts ...[]byte) uint16 {
	var sum uint32
	for _, part := range parts {
		for len(part) >= 2 {
			sum += uint32(binary.BigEndian.Uint16(part[:2]))
			part = part[2:]
		}
		if len(part) == 1 {
			sum += uint32(part[0]) << 8
		}
	}
	for sum>>16 != 0 {
		sum = sum&0xffff + sum>>16
	}
	return ^uint16(sum)
}

func blockedPutChecksum(target []byte, parts ...[]byte) {
	value := blockedChecksum(parts...)
	if value == 0 {
		value = 0xffff
	}
	binary.BigEndian.PutUint16(target, value)
}

func blockedNetworkShort(value uint16) uint16 { return value<<8 | value>>8 }
