package camouflage

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

func openClientSOCKS(listener string, config Config, deadline time.Time) (net.Conn, error) {
	timeout := min(2*time.Second, time.Until(deadline))
	if timeout <= 0 {
		return nil, errors.New("startup deadline expired before SOCKS")
	}
	connection, err := net.DialTimeout("tcp", listener, timeout)
	if err != nil {
		return nil, err
	}
	fail := func(failure error) (net.Conn, error) {
		_ = connection.Close()
		return nil, failure
	}
	_ = connection.SetDeadline(deadline)
	if _, err = connection.Write([]byte{5, 1, 2}); err != nil {
		return fail(err)
	}
	response := make([]byte, 2)
	if _, err = io.ReadFull(connection, response); err != nil || response[0] != 5 || response[1] != 2 {
		return fail(fmt.Errorf("SOCKS auth selection %x: %w", response, err))
	}
	arguments := clientArguments(config)
	if len(arguments) == 0 || len(arguments) > 255 {
		return fail(errors.New("SOCKS argument envelope outside 1..255 bytes"))
	}
	auth := append([]byte{1, byte(len(arguments))}, []byte(arguments)...)
	auth = append(auth, 1, 0)
	if _, err = connection.Write(auth); err != nil {
		return fail(err)
	}
	if _, err = io.ReadFull(connection, response); err != nil || response[0] != 1 || response[1] != 0 {
		return fail(fmt.Errorf("SOCKS authentication failed %x: %w", response, err))
	}
	request := []byte{5, 1, 0, 1, config.address[0], config.address[1], config.address[2], config.address[3], 0, 0}
	binary.BigEndian.PutUint16(request[8:], config.port)
	if _, err = connection.Write(request); err != nil {
		return fail(err)
	}
	header := make([]byte, 4)
	if _, err = io.ReadFull(connection, header); err != nil || header[0] != 5 || header[1] != 0 {
		return fail(fmt.Errorf("SOCKS connect refused %x: %w", header, err))
	}
	addressBytes, err := socksAddressBytes(connection, header[3])
	if err != nil {
		return fail(err)
	}
	if _, err = io.CopyN(io.Discard, connection, int64(addressBytes+2)); err != nil {
		return fail(err)
	}
	_ = connection.SetDeadline(time.Time{})
	return connection, nil
}

func clientArguments(config Config) string {
	target := net.JoinHostPort(net.IP(config.address[:]).String(), strconv.Itoa(int(config.port)))
	values := []string{
		"cert=" + escapeSOCKS(base64.StdEncoding.EncodeToString(config.chainPin[:])),
		"url=" + escapeSOCKS("https://"+target+config.path),
		"servername=" + escapeSOCKS(config.serverName),
	}
	return strings.Join(values, ";")
}

func escapeSOCKS(value string) string {
	return strings.NewReplacer("\\", "\\\\", "=", "\\=", ";", "\\;").Replace(value)
}

func socksAddressBytes(connection net.Conn, kind byte) (int, error) {
	switch kind {
	case 1:
		return 4, nil
	case 4:
		return 16, nil
	case 3:
		length := []byte{0}
		if _, err := io.ReadFull(connection, length); err != nil {
			return 0, err
		}
		return int(length[0]), nil
	default:
		return 0, errors.New("invalid SOCKS reply address type")
	}
}
