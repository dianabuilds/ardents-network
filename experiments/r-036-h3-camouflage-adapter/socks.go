//go:build ignore

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

func openSOCKS(listener, target string, args map[string]string, deadline time.Time) (net.Conn, error) {
	timeout := time.Until(deadline)
	if timeout <= 0 {
		return nil, errors.New("startup deadline expired before SOCKS")
	}
	if timeout > 2*time.Second {
		timeout = 2 * time.Second
	}
	conn, err := net.DialTimeout("tcp", listener, timeout)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (net.Conn, error) {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(deadline)
	if _, err = conn.Write([]byte{5, 1, 2}); err != nil {
		return fail(err)
	}
	response := make([]byte, 2)
	if _, err = io.ReadFull(conn, response); err != nil {
		return fail(err)
	}
	if response[0] != 5 || response[1] != 2 {
		return fail(fmt.Errorf("SOCKS auth selection %x", response))
	}
	encoded := encodeArgs(args)
	if len(encoded) == 0 || len(encoded) > 255 {
		return fail(errors.New("SOCKS argument envelope outside 1..255 bytes"))
	}
	auth := append([]byte{1, byte(len(encoded))}, []byte(encoded)...)
	auth = append(auth, 1, 0)
	if _, err = conn.Write(auth); err != nil {
		return fail(err)
	}
	if _, err = io.ReadFull(conn, response); err != nil || response[1] != 0 {
		return fail(fmt.Errorf("SOCKS authentication failed: %x %v", response, err))
	}
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return fail(err)
	}
	ip := net.ParseIP(host).To4()
	port, err := strconv.Atoi(portText)
	if ip == nil || err != nil || port < 1 || port > 65535 {
		return fail(errors.New("target is not canonical IPv4:port"))
	}
	request := []byte{5, 1, 0, 1, ip[0], ip[1], ip[2], ip[3], 0, 0}
	binary.BigEndian.PutUint16(request[8:], uint16(port))
	if _, err = conn.Write(request); err != nil {
		return fail(err)
	}
	header := make([]byte, 4)
	if _, err = io.ReadFull(conn, header); err != nil {
		return fail(err)
	}
	if header[0] != 5 || header[1] != 0 {
		return fail(fmt.Errorf("SOCKS connect failed: %x", header))
	}
	addressBytes := 0
	switch header[3] {
	case 1:
		addressBytes = 4
	case 4:
		addressBytes = 16
	case 3:
		length := []byte{0}
		if _, err = io.ReadFull(conn, length); err != nil {
			return fail(err)
		}
		addressBytes = int(length[0])
	default:
		return fail(errors.New("invalid SOCKS reply address type"))
	}
	if _, err = io.CopyN(io.Discard, conn, int64(addressBytes+2)); err != nil {
		return fail(err)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func encodeArgs(args map[string]string) string {
	order := []string{"cert", "iat-mode", "url", "servername"}
	parts := make([]string, 0, len(args))
	for _, key := range order {
		if value, ok := args[key]; ok {
			parts = append(parts, escapeArg(key)+"="+escapeArg(value))
		}
	}
	return strings.Join(parts, ";")
}

func escapeArg(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "=", "\\=", ";", "\\;")
	return replacer.Replace(value)
}
