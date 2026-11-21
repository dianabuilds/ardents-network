package ardp

import (
	"encoding/binary"
	"errors"
	"time"
)

// Open is the canonical 49-byte recipient description carried by OPEN.
// A Node OPEN appends its separate restriction byte. Decoding authenticates
// neither the recipient nor its current duty or deadline.
type Open struct {
	RecipientNodeID         [32]byte
	RecipientDutyGeneration uint64
	Purpose                 uint8
	Deadline                time.Time
}

// ChildRestriction is the immutable Node-child wire restriction. Its value
// describes an already owned reservation; decoding it grants no authority.
type ChildRestriction uint8

const (
	OrdinaryChild ChildRestriction = iota
	IssuerBootstrapChild
)

// NodeOpen carries the separate authenticated-Node envelope. Endpoint OPEN
// cannot supply this restriction; receiving owners check it before TLS work.
type NodeOpen struct {
	Recipient   Open
	Restriction ChildRestriction
}

// DecodeNodeOpen refuses the retired 49-byte Node envelope and unknown
// restrictions. Role, admission and restriction propagation remain owner rules.
func DecodeNodeOpen(body []byte) (NodeOpen, error) {
	if len(body) != 50 || ChildRestriction(body[49]) > IssuerBootstrapChild {
		return NodeOpen{}, errors.New("route Node OPEN envelope invalid")
	}
	recipient, err := DecodeOpen(body[:49])
	if err != nil {
		return NodeOpen{}, err
	}
	return NodeOpen{Recipient: recipient, Restriction: ChildRestriction(body[49])}, nil
}

// EncodeNodeOpen emits a selected reservation's restriction without inferring
// it from peer input. The caller owns selection and permission to emit it.
func EncodeNodeOpen(o NodeOpen) ([]byte, error) {
	if o.Restriction > IssuerBootstrapChild {
		return nil, errors.New("route Node OPEN restriction invalid")
	}
	return append(EncodeOpen(o.Recipient, false), byte(o.Restriction)), nil
}

// DecodeOpen decodes only the recipient description. Callers enforce the
// exact Endpoint/Node envelope, restriction, role and current-time bounds.
func DecodeOpen(body []byte) (Open, error) {
	if len(body) != 49 {
		return Open{}, errors.New("route OPEN length invalid")
	}
	o := Open{}
	copy(o.RecipientNodeID[:], body[:32])
	o.RecipientDutyGeneration = binary.BigEndian.Uint64(body[32:40])
	o.Purpose = body[40]
	o.Deadline = time.Unix(int64(binary.BigEndian.Uint64(body[41:])), 0).UTC()
	if o.RecipientNodeID == [32]byte{} || o.RecipientDutyGeneration == 0 || (o.Purpose != 1 && o.Purpose != 3 && o.Purpose != 7 && o.Purpose != 4 && o.Purpose != 5 && o.Purpose != 6) {
		return Open{}, errors.New("route OPEN facts invalid")
	}
	return o, nil
}

// EncodeOpen serializes already selected fields. Selection, currentness and
// permission to emit them belong to the operation, not this codec.
func EncodeOpen(o Open, node bool) []byte {
	body := append([]byte(nil), o.RecipientNodeID[:]...)
	body = binary.BigEndian.AppendUint64(body, o.RecipientDutyGeneration)
	body = append(body, o.Purpose)
	body = binary.BigEndian.AppendUint64(body, uint64(o.Deadline.Unix()))
	if node {
		body = append(body, 0)
	}
	return body
}
