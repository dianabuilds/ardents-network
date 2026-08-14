//go:build linux

package recoverysmoke

import (
	"encoding/binary"
	"errors"
	"net"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

const (
	carrierDiagRequestBytes = 56
	carrierDiagMessageBytes = 72
	carrierTCPEstablished   = 1
	carrierDiagMaxMessages  = 256
	carrierDiagMaxBytes     = 256 << 10
	carrierDiagMaxDatagrams = 64
)

func platformCarrierSockets(remote string) ([]carrierObservation, error) {
	host, portText, _ := net.SplitHostPort(remote)
	port, _ := strconv.ParseUint(portText, 10, 16)
	messages, err := carrierDiagExchange(unix.SOCK_DIAG_BY_FAMILY, unix.NLM_F_REQUEST|unix.NLM_F_DUMP, nil, 5*time.Second)
	if err != nil {
		return nil, err
	}
	var result []carrierObservation
	for _, message := range messages {
		if len(message) < carrierDiagMessageBytes || message[0] != unix.AF_INET || message[1] != carrierTCPEstablished {
			continue
		}
		socketID := append([]byte(nil), message[4:52]...)
		if carrierMatchesRemote(socketID, net.ParseIP(host), uint16(port)) {
			observation := carrierObservationFromID(socketID, binary.NativeEndian.Uint32(message[68:72]))
			name, index, interfaceErr := platformCarrierInterfaceForAddress(observation.LocalAddress)
			if interfaceErr != nil {
				return nil, interfaceErr
			}
			observation.InterfaceName, observation.InterfaceIndex = name, index
			result = append(result, observation)
		}
	}
	return result, nil
}

func platformCarrierSocketPresent(socketID []byte, timeout time.Duration) (bool, error) {
	messages, err := carrierDiagExchange(unix.SOCK_DIAG_BY_FAMILY, unix.NLM_F_REQUEST, socketID, timeout)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exactCarrierSocketResponse(messages, socketID)
}

func exactCarrierSocketResponse(messages [][]byte, socketID []byte) (bool, error) {
	if len(messages) != 1 || len(messages[0]) < 52 || string(messages[0][4:52]) != string(socketID) {
		return false, errors.New("exact inet_diag socket response is missing or mismatched")
	}
	return true, nil
}

func platformCarrierInterfaceForAddress(endpoint string) (string, int, error) {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", 0, err
	}
	for _, candidate := range mustInterfaces() {
		addresses, addressErr := candidate.Addrs()
		if addressErr != nil {
			return "", 0, addressErr
		}
		for _, address := range addresses {
			ip, _, parseErr := net.ParseCIDR(address.String())
			if parseErr == nil && ip.Equal(net.ParseIP(host)) {
				return candidate.Name, candidate.Index, nil
			}
		}
	}
	return "", 0, errors.New("Carrier interface identity is missing")
}

func mustInterfaces() []net.Interface {
	interfaces, _ := net.Interfaces()
	return interfaces
}

func platformDeleteCarrierInterface(name string) error {
	interfaceValue, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	file, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	defer unix.Close(file)
	if err := unix.Bind(file, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return err
	}
	if err := unix.SetsockoptTimeval(file, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 5}); err != nil {
		return err
	}
	request := make([]byte, unix.NLMSG_HDRLEN+unix.SizeofIfInfomsg)
	binary.NativeEndian.PutUint32(request[0:4], uint32(len(request)))
	binary.NativeEndian.PutUint16(request[4:6], unix.RTM_DELLINK)
	binary.NativeEndian.PutUint16(request[6:8], unix.NLM_F_REQUEST|unix.NLM_F_ACK)
	binary.NativeEndian.PutUint32(request[8:12], 1)
	request[unix.NLMSG_HDRLEN] = unix.AF_UNSPEC
	binary.NativeEndian.PutUint32(request[unix.NLMSG_HDRLEN+4:unix.NLMSG_HDRLEN+8], uint32(interfaceValue.Index))
	if err := unix.Sendto(file, request, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return err
	}
	response := make([]byte, 4096)
	count, _, err := unix.Recvfrom(file, response, 0)
	if err != nil {
		return err
	}
	_, done, err := parseCarrierDiagDatagram(response[:count])
	if err != nil {
		return err
	}
	if !done {
		return errors.New("interface deletion acknowledgement is incomplete")
	}
	return nil
}

func carrierDiagExchange(messageType uint16, flags uint16, socketID []byte, timeout time.Duration) ([][]byte, error) {
	file, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_INET_DIAG)
	if err != nil {
		return nil, err
	}
	defer unix.Close(file)
	if err := unix.Bind(file, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, err
	}
	deadline := unix.NsecToTimeval(timeout.Nanoseconds())
	if err := unix.SetsockoptTimeval(file, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &deadline); err != nil {
		return nil, err
	}
	if err := unix.Sendto(file, makeCarrierDiagRequest(messageType, flags, socketID), 0,
		&unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, err
	}
	return receiveCarrierDiag(file, flags&unix.NLM_F_DUMP != 0)
}

func makeCarrierDiagRequest(messageType uint16, flags uint16, socketID []byte) []byte {
	request := make([]byte, 16+carrierDiagRequestBytes)
	binary.NativeEndian.PutUint32(request[0:4], uint32(len(request)))
	binary.NativeEndian.PutUint16(request[4:6], messageType)
	binary.NativeEndian.PutUint16(request[6:8], flags)
	binary.NativeEndian.PutUint32(request[8:12], 1)
	request[16], request[17] = unix.AF_INET, unix.IPPROTO_TCP
	states := uint32(1 << carrierTCPEstablished)
	if len(socketID) == carrierSocketIDBytes {
		states = ^uint32(0)
	}
	binary.NativeEndian.PutUint32(request[20:24], states)
	if len(socketID) == carrierSocketIDBytes {
		copy(request[24:], socketID)
	} else {
		for index := 64; index < 72; index++ {
			request[index] = 0xff
		}
	}
	return request
}

func receiveCarrierDiag(file int, multipart bool) ([][]byte, error) {
	var result [][]byte
	buffer := make([]byte, 64<<10)
	totalBytes := 0
	for datagram := 0; datagram < carrierDiagMaxDatagrams; datagram++ {
		count, _, err := unix.Recvfrom(file, buffer, 0)
		if err != nil {
			return nil, err
		}
		payloads, done, err := parseCarrierDiagDatagram(buffer[:count])
		if err != nil {
			return nil, err
		}
		for _, payload := range payloads {
			totalBytes += len(payload)
			if len(result) >= carrierDiagMaxMessages || totalBytes > carrierDiagMaxBytes {
				return nil, errors.New("inet_diag response exceeded its bound")
			}
			result = append(result, append([]byte(nil), payload...))
		}
		if !multipart {
			return result, nil
		}
		if done {
			return result, nil
		}
	}
	return nil, errors.New("inet_diag response did not terminate within its datagram bound")
}

func parseCarrierDiagDatagram(buffer []byte) ([][]byte, bool, error) {
	var result [][]byte
	for offset := 0; offset+16 <= len(buffer); {
		length := int(binary.NativeEndian.Uint32(buffer[offset : offset+4]))
		if length < 16 || offset+length > len(buffer) {
			return nil, false, errors.New("malformed inet_diag response")
		}
		messageType := binary.NativeEndian.Uint16(buffer[offset+4 : offset+6])
		payload := buffer[offset+16 : offset+length]
		switch messageType {
		case unix.NLMSG_DONE:
			return result, true, nil
		case unix.NLMSG_ERROR:
			if len(payload) < 4 {
				return nil, false, errors.New("short inet_diag error")
			}
			code := int32(binary.NativeEndian.Uint32(payload[:4]))
			if code == 0 {
				return result, true, nil
			}
			return nil, false, unix.Errno(-code)
		default:
			result = append(result, payload)
		}
		offset += (length + 3) &^ 3
	}
	if len(buffer)%4 != 0 {
		return nil, false, errors.New("truncated inet_diag response")
	}
	return result, false, nil
}
