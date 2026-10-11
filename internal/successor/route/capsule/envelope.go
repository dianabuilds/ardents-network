package capsule

import (
	"encoding/binary"
	"errors"
	"time"
)

const envelopeSize = 474

// Header is the complete Introduction-visible binding. It discloses no Target,
// Rendezvous, Connection identifier or callback address.
type Header struct {
	Slot, DeliveryNonce [32]byte
	Revision            uint64
	Expiry              time.Time
}

// Envelope retains an immutable copied, exact-size capsule. Possession grants
// neither a live recipient nor permission to accept its private request.
type Envelope struct {
	header        Header
	encapsulation [32]byte
	ciphertext    [360]byte
}

func validHeader(h Header) bool {
	return h.Slot != [32]byte{} && h.DeliveryNonce != [32]byte{} && h.Revision != 0 &&
		h.Expiry.Unix() > 0 && h.Expiry == h.Expiry.UTC().Truncate(time.Second)
}

// Parse admits only the exact v3 envelope, without padding or legacy fallback.
func Parse(raw []byte) (Envelope, error) {
	if len(raw) != envelopeSize || binary.BigEndian.Uint16(raw[112:114]) != 360 {
		return Envelope{}, errors.New("introduction capsule length invalid")
	}
	seconds := binary.BigEndian.Uint64(raw[40:48])
	if seconds > 1<<63-1 {
		return Envelope{}, errors.New("introduction capsule expiry overflow")
	}
	var e Envelope
	copy(e.header.Slot[:], raw[:32])
	e.header.Revision = binary.BigEndian.Uint64(raw[32:40])
	e.header.Expiry = time.Unix(int64(seconds), 0).UTC()
	copy(e.header.DeliveryNonce[:], raw[48:80])
	copy(e.encapsulation[:], raw[80:112])
	copy(e.ciphertext[:], raw[114:])
	if !validHeader(e.header) || e.encapsulation == [32]byte{} {
		return Envelope{}, errors.New("introduction capsule binding invalid")
	}
	return e, nil
}

func headerBytes(h Header) []byte {
	raw := append([]byte(nil), h.Slot[:]...)
	raw = binary.BigEndian.AppendUint64(raw, h.Revision)
	raw = binary.BigEndian.AppendUint64(raw, uint64(h.Expiry.Unix()))
	return append(raw, h.DeliveryNonce[:]...)
}

func (e Envelope) Header() Header          { return e.header }
func (e Envelope) Encapsulation() [32]byte { return e.encapsulation }
func (e Envelope) Ciphertext() [360]byte   { return e.ciphertext }

// Bytes returns a detached canonical envelope suitable for unchanged forwarding.
func (e Envelope) Bytes() []byte {
	raw := append(headerBytes(e.header), e.encapsulation[:]...)
	raw = binary.BigEndian.AppendUint16(raw, 360)
	return append(raw, e.ciphertext[:]...)
}

// Info is the exact accepted generation/profile domain, including its NUL.
func Info(profile [32]byte) []byte {
	return append([]byte("ardents-introduction-capsule-v3\x00"), profile[:]...)
}

// AssociatedData binds Info and the exact 80-byte header, excluding encapsulation.
func (e Envelope) AssociatedData(profile [32]byte) []byte {
	return append(Info(profile), headerBytes(e.header)...)
}
