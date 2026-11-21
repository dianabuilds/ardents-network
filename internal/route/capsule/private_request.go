package capsule

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
)

// Digest commits to the complete canonical recipient-only plaintext. It
// neither seals a capsule nor accepts any candidate fact as live authority.
func (v Request) Digest() ([32]byte, error) {
	raw, err := encodeRequest(v)
	if err != nil {
		return [32]byte{}, err
	}
	defer clear(raw)
	return sha256.Sum256(raw), nil
}

// Request contains recipient-only candidate input. Its grammar establishes no
// Network duty, Work Safety lease, Instance authentication or Connection permit.
type Request struct {
	Network, Target, PublicationDigest                           [32]byte
	Revision                                                     uint64
	RendezvousNode                                               [32]byte
	RendezvousDutyGeneration                                     uint64
	JoinSecret, HandshakeContext, ProfileDigest, ConnectionNonce [32]byte
	AttachmentGeneration                                         uint64
	Deadline                                                     time.Time
	InitiatorBinding                                             [32]byte
	WorkSafetyNotAfter, WorkSafetyMaximum, NoNewRecoveryAfter    int64
}

func validRequest(v Request) bool {
	for _, field := range [][32]byte{v.Network, v.Target, v.PublicationDigest, v.RendezvousNode, v.JoinSecret, v.HandshakeContext, v.ProfileDigest, v.ConnectionNonce, v.InitiatorBinding} {
		if field == [32]byte{} {
			return false
		}
	}
	return v.Revision != 0 && v.RendezvousDutyGeneration != 0 && v.AttachmentGeneration != 0 &&
		v.Deadline.Unix() > 0 && v.Deadline == v.Deadline.UTC().Truncate(time.Second) &&
		v.Deadline.Unix() <= v.WorkSafetyNotAfter && v.WorkSafetyMaximum >= v.WorkSafetyNotAfter &&
		v.NoNewRecoveryAfter > 0 && v.NoNewRecoveryAfter <= v.WorkSafetyNotAfter
}

func encodeRequest(v Request) ([]byte, error) {
	if !validRequest(v) {
		return nil, errors.New("introduction private request invalid")
	}
	raw := make([]byte, 0, 344)
	for _, f := range [][32]byte{v.Network, v.Target, v.PublicationDigest} {
		raw = append(raw, f[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, v.Revision)
	raw = append(raw, v.RendezvousNode[:]...)
	raw = binary.BigEndian.AppendUint64(raw, v.RendezvousDutyGeneration)
	for _, f := range [][32]byte{v.JoinSecret, v.HandshakeContext, v.ProfileDigest, v.ConnectionNonce} {
		raw = append(raw, f[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, v.AttachmentGeneration)
	raw = binary.BigEndian.AppendUint64(raw, uint64(v.Deadline.Unix()))
	raw = append(raw, v.InitiatorBinding[:]...)
	for _, bound := range []int64{v.WorkSafetyNotAfter, v.WorkSafetyMaximum, v.NoNewRecoveryAfter} {
		raw = binary.BigEndian.AppendUint64(raw, uint64(bound))
	}
	return raw, nil
}

// ParseRequest checks the exact 344-byte plaintext and its canonical bounds.
// The caller must separately authenticate every candidate fact before effects.
func ParseRequest(raw []byte) (Request, error) {
	if len(raw) != 344 {
		return Request{}, errors.New("introduction private request length invalid")
	}
	var v Request
	copy(v.Network[:], raw[:32])
	copy(v.Target[:], raw[32:64])
	copy(v.PublicationDigest[:], raw[64:96])
	v.Revision = binary.BigEndian.Uint64(raw[96:104])
	copy(v.RendezvousNode[:], raw[104:136])
	v.RendezvousDutyGeneration = binary.BigEndian.Uint64(raw[136:144])
	copy(v.JoinSecret[:], raw[144:176])
	copy(v.HandshakeContext[:], raw[176:208])
	copy(v.ProfileDigest[:], raw[208:240])
	copy(v.ConnectionNonce[:], raw[240:272])
	v.AttachmentGeneration = binary.BigEndian.Uint64(raw[272:280])
	seconds := binary.BigEndian.Uint64(raw[280:288])
	if seconds > 1<<63-1 {
		return Request{}, errors.New("introduction private deadline overflow")
	}
	v.Deadline = time.Unix(int64(seconds), 0).UTC()
	copy(v.InitiatorBinding[:], raw[288:320])
	for i, bound := range []*int64{&v.WorkSafetyNotAfter, &v.WorkSafetyMaximum, &v.NoNewRecoveryAfter} {
		seconds = binary.BigEndian.Uint64(raw[320+i*8 : 328+i*8])
		if seconds > 1<<63-1 {
			return Request{}, errors.New("introduction private safety bound overflow")
		}
		*bound = int64(seconds)
	}
	if !validRequest(v) {
		return Request{}, errors.New("introduction private request invalid")
	}
	return v, nil
}
