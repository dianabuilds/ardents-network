//go:build linux

package textdocument

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"unicode/utf8"
)

// InitializeWorker implements the Endpoint side of the fixed local exchange.
// The caller must establish the installed worker and accepted socket before
// sending a snapshot. Readiness is only nonce/digest acknowledgement, never an
// isolation assertion or authority to issue a Principal or Grant.
func InitializeWorker(ctx context.Context, attachment io.ReadWriteCloser, mode WorkerMode, nonce [32]byte, snapshot []byte) (resultErr error) {
	if ctx == nil || attachment == nil || nonce == [32]byte{} ||
		(mode != ReaderWorker && mode != PublisherWorker) || len(snapshot) > MaximumBytes ||
		!utf8.Valid(snapshot) || mode == ReaderWorker && len(snapshot) != 0 {
		return errors.New("text worker initialization is invalid")
	}
	if ctx.Err() != nil {
		return errors.New("text worker initialization was cancelled")
	}
	cleanupDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(cleanupDone); _ = attachment.Close() })
	defer func() {
		if !stop() {
			<-cleanupDone
			if resultErr == nil {
				resultErr = errors.New("text worker initialization was cancelled")
			}
		}
	}()
	// Keep the digest and transmitted body in one private immutable observation.
	body := bytes.Clone(snapshot)
	defer clear(body)
	digest := sha256.Sum256(body)
	var header [77]byte
	copy(header[:8], "ARDTWP01")
	header[8] = byte(mode)
	copy(header[9:41], nonce[:])
	binary.BigEndian.PutUint32(header[41:45], uint32(len(body)))
	copy(header[45:], digest[:])
	if _, err := io.Copy(attachment, io.MultiReader(bytes.NewReader(header[:]), bytes.NewReader(body))); err != nil {
		return errors.New("text worker initialization was interrupted")
	}
	if err := readWorkerReadiness(attachment, nonce, digest); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return errors.New("text worker readiness was cancelled")
	}
	return nil
}

func readWorkerReadiness(attachment io.Reader, nonce, digest [32]byte) error {
	// Read one extra byte to reject coalesced unsolicited frames. Short stream
	// reads are valid; each fragment still passes the caller's credential reader.
	var ready [73]byte
	received := 0
	for received < 72 {
		n, err := attachment.Read(ready[received:])
		if n < 0 || n > len(ready)-received {
			return errors.New("text worker readiness is invalid")
		}
		received += n
		if err != nil || n == 0 || received > 72 {
			return errors.New("text worker readiness is interrupted or oversized")
		}
	}
	if string(ready[:8]) != "ARDTWR01" || !bytes.Equal(ready[8:40], nonce[:]) || !bytes.Equal(ready[40:72], digest[:]) {
		return errors.New("text worker readiness is bound to another job")
	}
	return nil
}
