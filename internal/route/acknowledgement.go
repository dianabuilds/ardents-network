package route

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
)

const acknowledgementBodySize = 149

func startAcknowledgement(ctx context.Context, socket, keyFile string) (func(), <-chan error, error) {
	file, err := os.Open(keyFile)
	if err != nil {
		return nil, nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, ed25519.PrivateKeySize*2+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(raw) != ed25519.PrivateKeySize*2 {
		return nil, nil, errors.New("introduction acknowledgement key is invalid")
	}
	private := make([]byte, ed25519.PrivateKeySize)
	if _, err := hex.Decode(private, raw); err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, nil, err
	}
	_ = os.Chmod(socket, 0o600)
	completed := make(chan error, 1)
	go serveAcknowledgement(listener, private, completed)
	stop := func() { _ = listener.Close(); _ = os.Remove(socket); eraseBytes(private) }
	go func() { <-ctx.Done(); _ = listener.Close() }()
	return stop, completed, nil
}

func serveAcknowledgement(listener net.Listener, private ed25519.PrivateKey, completed chan<- error) {
	connection, err := listener.Accept()
	if err != nil {
		completed <- err
		return
	}
	defer connection.Close()
	body := make([]byte, acknowledgementBodySize)
	if _, err = io.ReadFull(connection, body); err != nil || string(body[:4]) != "ASIA" || body[4] != 1 {
		completed <- errors.Join(err, errors.New("introduction acknowledgement request is malformed"))
		return
	}
	message := append([]byte("ardents-h3-introduction-ack-v1\x00"), body...)
	_, err = connection.Write(ed25519.Sign(private, message))
	completed <- err
}

func eraseBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
