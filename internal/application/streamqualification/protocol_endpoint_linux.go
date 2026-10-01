//go:build linux

package streamqualification

import (
	"bytes"
	"errors"
	"io"
)

// InitializeWorker binds the verified installed process to Endpoint's fresh
// invocation nonce. The caller owns the attachment deadline and cancellation.
func InitializeWorker(attachment io.ReadWriter, init Init) error {
	if attachment == nil || init.Nonce == [32]byte{} || init.Seed == [32]byte{} {
		return errors.New("qualification initialization unavailable")
	}
	if _, err := init.Profile.Definition(init.Role); err != nil {
		return err
	}
	body := make([]byte, initBytes)
	copy(body, initMagic)
	body[len(initMagic)], body[len(initMagic)+1] = byte(init.Role), byte(init.Profile)
	copy(body[len(initMagic)+2:], init.Nonce[:])
	copy(body[len(initMagic)+34:], init.Seed[:])
	if err := writeWorkerBytes(attachment, body); err != nil {
		return err
	}
	ready := make([]byte, readyBytes)
	if _, err := io.ReadFull(attachment, ready); err != nil {
		return err
	}
	if string(ready[:len(readyMagic)]) != readyMagic || !bytes.Equal(ready[len(readyMagic):], init.Nonce[:]) {
		return errors.New("qualification readiness differs from invocation")
	}
	return nil
}
