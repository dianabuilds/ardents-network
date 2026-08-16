package streamworkload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"time"
)

const timedDirectReceiptBytes = 8 + sha256.Size

func sendTimedDirect(ctx context.Context, connection net.Conn, config DirectConfig) error {
	started := time.Now()
	deadline := started.Add(config.MeasureDuration)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	stop := context.AfterFunc(ctx, func() { abortTimedDirect(connection) })
	defer stop()
	if err := connection.SetWriteDeadline(deadline); err != nil {
		return err
	}
	source := generator{seed: config.Seed}
	buffer := make([]byte, 16_381)
	written := 0
	for time.Now().Before(deadline) && written < config.Bytes {
		chunk := min(len(buffer), config.Bytes-written)
		source.fill(buffer[:chunk])
		pending := buffer[:chunk]
		for len(pending) > 0 {
			count, err := connection.Write(pending)
			if count > 0 {
				written += count
				pending = pending[count:]
			}
			if err != nil {
				if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() || time.Now().Before(deadline) {
					return err
				}
				pending = nil
				break
			}
		}
	}
	measured := time.Since(started)
	if ctx.Err() != nil || measured < config.MeasureDuration {
		abortTimedDirect(connection)
		return errors.Join(ctx.Err(), errors.New("timed direct workload ended before its measurement window"))
	}
	if written == config.Bytes && measured < config.MeasureDuration {
		return errors.New("timed direct workload exhausted its offered bytes")
	}
	if err := connection.SetWriteDeadline(time.Time{}); err != nil {
		return err
	}
	tcp, ok := connection.(*net.TCPConn)
	if !ok {
		return errors.New("timed direct workload connection is not TCP")
	}
	if err := tcp.CloseWrite(); err != nil {
		return err
	}
	receipt := make([]byte, timedDirectReceiptBytes)
	if _, err := io.ReadFull(connection, receipt); err != nil {
		return err
	}
	delivered := binary.BigEndian.Uint64(receipt[:8])
	deliveredDigest := directPrefixDigest(config.Seed, int(delivered))
	if delivered == 0 || delivered > uint64(config.Bytes) || !bytes.Equal(receipt[8:], deliveredDigest[:]) {
		return errors.New("timed direct workload receipt does not match delivered bytes")
	}
	if err := awaitDirectHalfClose(connection); err != nil {
		return err
	}
	result := Observation{Schema: "ardents-h3-stream-application-v1", Role: "direct-client", Terminal: "success",
		SentBytes: uint32(delivered), SentDigest: deliveredDigest, SendSeed: config.Seed,
		DurationMillis: uint32(measured / time.Millisecond)}
	return json.NewEncoder(config.Output).Encode(result)
}

func directPrefixDigest(seed [32]byte, count int) [32]byte {
	hash, source := sha256.New(), generator{seed: seed}
	buffer := make([]byte, 16_381)
	for generated := 0; generated < count; {
		chunk := min(len(buffer), count-generated)
		source.fill(buffer[:chunk])
		_, _ = hash.Write(buffer[:chunk])
		generated += chunk
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func receiveTimedDirect(ctx context.Context, connection net.Conn, config DirectConfig) error {
	stop := context.AfterFunc(ctx, func() { abortTimedDirect(connection) })
	defer stop()
	started := time.Now()
	hash, expected := sha256.New(), generator{seed: config.Seed}
	value, want := make([]byte, 16_381), make([]byte, 16_381)
	received := 0
	for {
		count, err := connection.Read(value)
		if count > 0 {
			if received+count > config.Bytes {
				return errors.New("timed direct workload exceeded its offered-byte bound")
			}
			expected.fill(want[:count])
			if !bytes.Equal(value[:count], want[:count]) {
				return errors.New("timed direct workload bytes differ")
			}
			_, _ = hash.Write(value[:count])
			received += count
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.Join(ctx.Err(), err)
		}
	}
	measured := time.Since(started)
	tolerance := min(config.MeasureDuration/20, 100*time.Millisecond)
	if measured < config.MeasureDuration-tolerance {
		return errors.New("timed direct workload ended before the receiver measurement window")
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	receipt := make([]byte, timedDirectReceiptBytes)
	binary.BigEndian.PutUint64(receipt[:8], uint64(received))
	copy(receipt[8:], digest[:])
	if _, err := connection.Write(receipt); err != nil {
		return err
	}
	tcp, ok := connection.(*net.TCPConn)
	if !ok {
		return errors.New("timed direct workload connection is not TCP")
	}
	if err := tcp.CloseWrite(); err != nil {
		return err
	}
	result := Observation{Schema: "ardents-h3-stream-application-v1", Role: "direct-server", Terminal: "success",
		ReceivedBytes: uint32(received), ReceivedDigest: digest, ExpectSeed: config.Seed,
		DurationMillis: uint32(measured / time.Millisecond)}
	return json.NewEncoder(config.Output).Encode(result)
}

func abortTimedDirect(connection net.Conn) {
	if tcp, ok := connection.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
	_ = connection.Close()
}
