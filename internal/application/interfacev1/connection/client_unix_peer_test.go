package connection

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

func acceptNonReadingPeer(listener *net.UnixListener, ready chan<- peerSetupResult) {
	defer close(ready)
	connection, err := acceptApplicationPeer(listener)
	if err != nil {
		ready <- peerSetupResult{err: err}
		return
	}
	if err := connection.SetReadBuffer(4 << 10); err != nil {
		ready <- failedPeerSetup(connection, err)
		return
	}
	var frame [4]byte
	if _, err := io.ReadFull(connection, frame[:]); err != nil {
		ready <- failedPeerSetup(connection, fmt.Errorf("read first Application frame: %w", err))
		return
	}
	length := binary.BigEndian.Uint32(frame[:])
	if length == 0 || length > maximumFrame {
		ready <- failedPeerSetup(connection, fmt.Errorf("first Application frame length = %d", length))
		return
	}
	ready <- peerSetupResult{connection: connection}
}

func acceptConnectedPeer(listener *net.UnixListener, ready chan<- peerSetupResult) {
	defer close(ready)
	connection, err := acceptApplicationPeer(listener)
	if err != nil {
		ready <- peerSetupResult{err: err}
		return
	}
	ready <- peerSetupResult{connection: connection}
}

func acceptCapturingPeer(listener *net.UnixListener, received chan<- captureResult, done chan<- struct{}) {
	defer close(done)
	defer close(received)
	result := captureResult{}
	connection, err := acceptApplicationPeer(listener)
	if err != nil {
		result.err = err
		received <- result
		return
	}
	defer func() {
		result.err = errors.Join(result.err, connection.Close())
		received <- result
	}()
	var data bytes.Buffer
	for {
		var frame [4]byte
		if _, err := io.ReadFull(connection, frame[:]); err != nil {
			result.err = err
			return
		}
		length := binary.BigEndian.Uint32(frame[:])
		if length == 0 {
			result.data = bytes.Clone(data.Bytes())
			return
		}
		if length > maximumFrame {
			result.err = errors.New("peer received an oversized frame")
			return
		}
		if _, err := io.CopyN(&data, connection, int64(length)); err != nil {
			result.err = err
			return
		}
	}
}

func acceptApplicationPeer(listener *net.UnixListener) (*net.UnixConn, error) {
	connection, err := listener.AcceptUnix()
	if err != nil {
		return nil, err
	}
	header := make([]byte, len(localMagic)+2)
	if _, err := io.ReadFull(connection, header); err != nil {
		return nil, errors.Join(err, connection.Close())
	}
	if string(header[:len(localMagic)]) != localMagic {
		return nil, errors.Join(errors.New("peer received an invalid request header"), connection.Close())
	}
	linkLength := int(binary.BigEndian.Uint16(header[len(localMagic):]))
	if linkLength == 0 {
		return nil, errors.Join(errors.New("peer received an empty Target Link"), connection.Close())
	}
	if _, err := io.CopyN(io.Discard, connection, int64(linkLength)); err != nil {
		return nil, errors.Join(err, connection.Close())
	}
	if _, err := connection.Write([]byte{1}); err != nil {
		return nil, errors.Join(err, connection.Close())
	}
	return connection, nil
}

func failedPeerSetup(connection *net.UnixConn, err error) peerSetupResult {
	return peerSetupResult{err: errors.Join(err, connection.Close())}
}
