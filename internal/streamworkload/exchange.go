package streamworkload

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"sync"
)

// Observation is the complete public result of one opaque workload exchange.
type Observation struct {
	Schema              string   `json:"schema"`
	Role                string   `json:"role"`
	Terminal            string   `json:"terminal"`
	SentBytes           uint32   `json:"sent_bytes"`
	ReceivedBytes       uint32   `json:"received_bytes"`
	SentDigest          [32]byte `json:"sent_digest"`
	ReceivedDigest      [32]byte `json:"received_digest"`
	ReceivedTail        [32]byte `json:"received_tail"`
	DurationMillis      uint32   `json:"duration_millis,omitempty"`
	ResultClass         string   `json:"result_class"`
	AuthenticatedTarget [32]byte `json:"authenticated_target"`
	SendSeed            [32]byte `json:"send_seed"`
	ExpectSeed          [32]byte `json:"expect_seed"`
	Corpus              string   `json:"corpus,omitempty"`
	RequestNonce        [32]byte `json:"request_nonce,omitempty"`
	CompletedAtUnixNano int64    `json:"completed_at_unix_nano,omitempty"`
}

type byteFiller interface{ fill([]byte) }

// Exchange concurrently sends and validates deterministic bytes without a
// payload-sized allocation. A nil writer uses the supplied connection.
func Exchange(connection io.ReadWriter, role string, sendSeed, expectSeed [32]byte,
	sendCount, receiveCount int, write func([]byte) (int, error), progress func(uint32)) (Observation, error) {
	return exchange(connection, role, sendSeed, expectSeed, sendCount, receiveCount, write, progress,
		&generator{seed: sendSeed}, &generator{seed: expectSeed})
}

func exchange(connection io.ReadWriter, role string, sendSeed, expectSeed [32]byte,
	sendCount, receiveCount int, write func([]byte) (int, error), progress func(uint32),
	sendSource, expectSource byteFiller,
) (Observation, error) {
	result := Observation{Schema: "ardents-h3-stream-application-v1", Role: role,
		SendSeed: sendSeed, ExpectSeed: expectSeed, SentBytes: uint32(sendCount)}
	type transfer struct {
		digest  [32]byte
		tail    [32]byte
		count   uint32
		sending bool
		err     error
	}
	transfers := make(chan transfer, 2)
	var writes sync.WaitGroup
	writes.Add(1)
	go func() {
		defer writes.Done()
		hash, buffer := sha256.New(), make([]byte, 16_381)
		written := 0
		for written < sendCount {
			chunk := min(len(buffer), sendCount-written)
			sendSource.fill(buffer[:chunk])
			pending := buffer[:chunk]
			for len(pending) > 0 {
				if write == nil {
					write = connection.Write
				}
				count, writeErr := write(pending)
				if count > 0 {
					_, _ = hash.Write(pending[:count])
					pending = pending[count:]
				}
				written += count
				if writeErr != nil || count == 0 {
					transfers <- transfer{err: errors.Join(writeErr, io.ErrShortWrite)}
					return
				}
			}
		}
		var digest [32]byte
		copy(digest[:], hash.Sum(nil))
		transfers <- transfer{digest: digest, count: uint32(written), sending: true}
	}()
	go func() {
		hash := sha256.New()
		value, expected := make([]byte, 16_381), make([]byte, 16_381)
		var tail [32]byte
		read := 0
		var err error
		for read < receiveCount {
			want := min(len(value), receiveCount-read)
			count, readErr := io.ReadFull(connection, value[:want])
			expectSource.fill(expected[:count])
			if !bytes.Equal(value[:count], expected[:count]) {
				readErr = errors.Join(readErr, errors.New("opaque stream length, order, or bytes differ"))
			}
			read += count
			if count > 0 {
				_, _ = hash.Write(value[:count])
				appendTail(&tail, value[:count], read)
				if progress != nil {
					progress(uint32(read))
				}
			}
			err = readErr
			if err != nil {
				break
			}
		}
		var digest [32]byte
		copy(digest[:], hash.Sum(nil))
		transfers <- transfer{digest: digest, tail: tail, count: uint32(read), err: err}
	}()
	first, second := <-transfers, <-transfers
	writes.Wait()
	sent, received := first, second
	if !first.sending {
		sent, received = second, first
	}
	err := errors.Join(first.err, second.err)
	result.SentBytes, result.SentDigest = sent.count, sent.digest
	result.ReceivedBytes, result.ReceivedDigest, result.ReceivedTail = received.count, received.digest, received.tail
	if err != nil || sent.count != uint32(sendCount) || received.count != uint32(receiveCount) {
		result.Terminal = "error"
		return result, errors.Join(err, errors.New("opaque stream length, order, or bytes differ"))
	}
	result.Terminal = "success"
	return result, nil
}

func appendTail(tail *[32]byte, value []byte, total int) {
	if total < len(tail) {
		copy(tail[:], value)
		return
	}
	if len(value) >= len(tail) {
		copy(tail[:], value[len(value)-len(tail):])
		return
	}
	copy(tail[:], tail[len(value):])
	copy(tail[len(tail)-len(value):], value)
}
