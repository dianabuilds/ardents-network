//go:build ignore

package main

import (
	"errors"
	"net"
	"strings"
	"time"
)

const faultRelayProtocol = "r094-relay-control-v1"

func sendFaultRelayControl(endpoint, action, token string, attemptDeadline time.Time) (string, error) {
	address, err := net.ResolveUDPAddr("udp", endpoint)
	if err != nil {
		return "", err
	}
	connection, err := net.DialUDP("udp", nil, address)
	if err != nil {
		return "", err
	}
	defer connection.Close()
	deadline := time.Now().Add(2 * time.Second)
	if attemptDeadline.Before(deadline) {
		deadline = attemptDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return "", err
	}
	request := faultRelayProtocol + "|" + action + "|" + token
	if _, err := connection.Write([]byte(request)); err != nil {
		return "", err
	}
	buffer := make([]byte, 512)
	count, err := connection.Read(buffer)
	if err != nil {
		return "", err
	}
	response := string(buffer[:count])
	expectedPrefix := faultRelayProtocol + "|ok|" + action + "|"
	if !strings.HasPrefix(response, expectedPrefix) {
		return "", errors.New("fault relay returned an invalid control response")
	}
	return response[len(expectedPrefix):], nil
}
