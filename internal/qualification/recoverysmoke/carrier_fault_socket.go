package recoverysmoke

import (
	"encoding/binary"
	"net"
	"strconv"
)

const carrierSocketIDBytes = 48

func carrierSocketEndpoint(raw []byte, local bool) string {
	portOffset, addressOffset := 0, 4
	if !local {
		portOffset, addressOffset = 2, 20
	}
	if len(raw) != carrierSocketIDBytes {
		return ""
	}
	address := net.IP(raw[addressOffset : addressOffset+4]).String()
	port := binary.BigEndian.Uint16(raw[portOffset : portOffset+2])
	return net.JoinHostPort(address, strconv.Itoa(int(port)))
}

func carrierMatchesRemote(raw []byte, remoteIP net.IP, remotePort uint16) bool {
	return len(raw) == carrierSocketIDBytes && binary.BigEndian.Uint16(raw[2:4]) == remotePort &&
		net.IP(raw[20:24]).Equal(remoteIP.To4())
}
