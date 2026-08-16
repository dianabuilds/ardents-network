package camouflage

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
)

func reserveLoopbackAddress() (string, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	address := listener.Addr().String()
	return address, listener.Close()
}

func validNextLeg(raw string) bool {
	address, err := netip.ParseAddrPort(raw)
	return err == nil && address.Addr().Is4() && !address.Addr().IsUnspecified() &&
		!address.Addr().IsMulticast() && address.Port() != 0
}

func intString(value uint16) string { return strconv.Itoa(int(value)) }

func publicURL(config Config) (string, error) {
	if config.serverName == "" || config.path == "" {
		return "", errors.New("adapter-config-invalid")
	}
	return "https://" + config.serverName + config.path, nil
}
