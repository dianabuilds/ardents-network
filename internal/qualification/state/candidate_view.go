package state

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"time"
)

type verifiedRecord struct {
	raw        []byte
	networkID  [32]byte
	nodeID     [32]byte
	generation uint64
	notBefore  time.Time
	notAfter   time.Time
	family     string
	capability byte
	endpoint   string
	capacity   uint16
	publicKey  ed25519.PublicKey
	keyID      [32]byte
	signature  []byte
}

type evaluatedInput struct {
	index  int
	record verifiedRecord
	code   uint16
}

type rejection struct {
	index uint32
	code  uint16
	raw   []byte
}

func evaluateInputs(input Case, epoch verifiedEpoch, inputs [][]byte) ([]verifiedRecord, []rejection) {
	items := make([]evaluatedInput, len(inputs))
	for index, raw := range inputs {
		record, err := decodeRecord(raw)
		items[index] = evaluatedInput{index: index, record: record}
		switch {
		case err != nil:
			items[index].code = 1
		case record.networkID != input.NetworkID:
			items[index].code = 2
		case !recordSignatureValid(record):
			items[index].code = 3
		case epoch.validFrom.Before(record.notBefore) || !epoch.validFrom.Before(record.notAfter):
			items[index].code = 4
		case record.capability != 1:
			items[index].code = 5
		case record.capacity == 0 || record.capacity > 1024:
			items[index].code = 6
		case authorityKeyCollision(input, record.keyID):
			items[index].code = 7
		}
	}
	markCollisions(items)
	accepted := make([]verifiedRecord, 0, len(items))
	rejected := make([]rejection, 0, len(items))
	for _, item := range items {
		if item.code == 0 {
			accepted = append(accepted, item.record)
		} else {
			rejected = append(rejected, rejection{uint32(item.index), item.code, inputs[item.index]})
		}
	}
	sort.Slice(accepted, func(i, j int) bool {
		return bytes.Compare(accepted[i].nodeID[:], accepted[j].nodeID[:]) < 0
	})
	return accepted, rejected
}

func decodeRecord(raw []byte) (verifiedRecord, error) {
	if len(raw) == 0 || len(raw) > 32<<10 {
		return verifiedRecord{}, errors.New("record framing length is invalid")
	}
	decoder := byteDecoder{raw: raw}
	magic, err := decoder.take(4)
	if err != nil || string(magic) != "ARNR" {
		return verifiedRecord{}, errors.New("record magic is invalid")
	}
	version, err := decoder.one()
	if err != nil || version != 1 {
		return verifiedRecord{}, errors.New("record schema version is invalid")
	}
	var record verifiedRecord
	record.raw = append([]byte(nil), raw...)
	network, err := decoder.take(32)
	if err != nil {
		return verifiedRecord{}, err
	}
	copy(record.networkID[:], network)
	node, err := decoder.take(32)
	if err != nil {
		return verifiedRecord{}, err
	}
	copy(record.nodeID[:], node)
	if record.generation, err = decoder.u64(); err != nil || record.generation == 0 {
		return verifiedRecord{}, errors.New("record generation is invalid")
	}
	from, err := decoder.i64()
	if err != nil {
		return verifiedRecord{}, err
	}
	until, err := decoder.i64()
	if err != nil || until <= from {
		return verifiedRecord{}, errors.New("record validity interval is invalid")
	}
	record.notBefore, record.notAfter = time.Unix(from, 0).UTC(), time.Unix(until, 0).UTC()
	if record.family, err = decoder.text(32); err != nil {
		return verifiedRecord{}, err
	}
	if record.capability, err = decoder.one(); err != nil {
		return verifiedRecord{}, err
	}
	if record.endpoint, err = decoder.text(96); err != nil {
		return verifiedRecord{}, err
	}
	if record.capacity, err = decoder.u16(); err != nil {
		return verifiedRecord{}, err
	}
	public, err := decoder.take(ed25519.PublicKeySize)
	if err != nil {
		return verifiedRecord{}, err
	}
	record.publicKey = append(ed25519.PublicKey(nil), public...)
	record.keyID = sha256.Sum256(public)
	record.signature, err = decoder.take(ed25519.SignatureSize)
	if err != nil || decoder.offset != len(raw) {
		return verifiedRecord{}, errors.New("record signature framing or trailing bytes are invalid")
	}
	return record, nil
}

func recordSignatureValid(record verifiedRecord) bool {
	unsigned := len(record.raw) - ed25519.SignatureSize
	return ed25519.Verify(record.publicKey, record.raw[:unsigned], record.signature)
}

func authorityKeyCollision(input Case, key [32]byte) bool {
	_, exists := input.Authorities[key]
	return exists
}

func markCollisions(items []evaluatedInput) {
	nodes, generations := make(map[[32]byte]int), make(map[string]int)
	keys, endpoints := make(map[[32]byte]int), make(map[string]int)
	for _, item := range items {
		if item.code != 0 {
			continue
		}
		nodes[item.record.nodeID]++
		generations[recordGeneration(item.record)]++
		keys[item.record.keyID]++
		endpoints[item.record.endpoint]++
	}
	for index := range items {
		item := &items[index]
		if item.code != 0 {
			continue
		}
		switch {
		case nodes[item.record.nodeID] > 1:
			item.code = 8
		case generations[recordGeneration(item.record)] > 1:
			item.code = 9
		case keys[item.record.keyID] > 1:
			item.code = 10
		case endpoints[item.record.endpoint] > 1:
			item.code = 11
		}
	}
}

func recordGeneration(record verifiedRecord) string {
	return string(record.nodeID[:]) + fmt.Sprintf("/%d", record.generation)
}

func verifyView(epoch verifiedEpoch, accepted []verifiedRecord, rejected []rejection) error {
	view := make([][]byte, len(accepted))
	families := make(map[string][2]uint32)
	var totalCapacity uint32
	for index, record := range accepted {
		view[index] = record.raw
		totalCapacity += uint32(record.capacity)
		summary := families[record.family]
		summary[0]++
		summary[1] += uint32(record.capacity)
		families[record.family] = summary
	}
	rejectionLeaves := make([][32]byte, len(rejected))
	for index, item := range rejected {
		rejectionLeaves[index] = rejectionLeaf(item.index, item.code, item.raw)
	}
	var maxCount, maxCapacity uint32
	for _, summary := range families {
		maxCount = max(maxCount, summary[0])
		maxCapacity = max(maxCapacity, summary[1])
	}
	if uint32(len(view)) != epoch.viewLength || recordRoot(view, 0x11) != epoch.viewRoot ||
		uint32(len(rejected)) != epoch.rejectedLength || hashedRoot(rejectionLeaves, 0x12) != epoch.rejectedRoot {
		return errors.New("candidate view or rejection root disagrees")
	}
	if epoch.eligibleCount != uint32(len(view)) || epoch.eligibleCapacity != totalCapacity ||
		epoch.familyCount != uint16(len(families)) || epoch.maxFamilyCount != uint16(maxCount) ||
		epoch.maxFamilyCapacity != maxCapacity {
		return errors.New("candidate summaries disagree")
	}
	return verifyDomains(epoch, families)
}
