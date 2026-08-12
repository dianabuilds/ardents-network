package state

import (
	"bytes"
	"errors"
)

type materialization struct {
	digest   [32]byte
	index    uint32
	record   []byte
	siblings [][32]byte
}

func verifyMaterials(epoch verifiedEpoch, accepted []verifiedRecord, rawMaterials [][]byte) error {
	if len(rawMaterials) > 64 {
		return errors.New("materialization set exceeds 64 entries")
	}
	if len(accepted) > 0 && len(rawMaterials) == 0 {
		return errors.New("materialization evidence is missing")
	}
	seen := make(map[uint32]bool, len(rawMaterials))
	for _, raw := range rawMaterials {
		material, err := decodeMaterial(raw)
		if err != nil {
			return err
		}
		if material.digest != epoch.digest || material.index >= uint32(len(accepted)) || seen[material.index] {
			return errors.New("materialization identity is invalid")
		}
		seen[material.index] = true
		record := accepted[material.index].raw
		if !bytes.Equal(record, material.record) ||
			!proofMatches(record, material.index, uint32(len(accepted)), material.siblings, epoch.viewRoot) {
			return errors.New("materialization proof is invalid")
		}
	}
	return nil
}

func decodeMaterial(raw []byte) (materialization, error) {
	if len(raw) < 42 {
		return materialization{}, errors.New("materialization is truncated")
	}
	decoder := byteDecoder{raw: raw}
	var material materialization
	digest, err := decoder.take(32)
	if err != nil {
		return materialization{}, err
	}
	copy(material.digest[:], digest)
	if material.index, err = decoder.u32(); err != nil {
		return materialization{}, err
	}
	recordLength, err := decoder.u32()
	if err != nil || recordLength == 0 || recordLength > 32<<10 {
		return materialization{}, errors.New("materialization record length is invalid")
	}
	material.record, err = decoder.take(int(recordLength))
	if err != nil {
		return materialization{}, err
	}
	count, err := decoder.u16()
	if err != nil || count > 64 {
		return materialization{}, errors.New("materialization proof length is invalid")
	}
	material.siblings = make([][32]byte, count)
	for index := range material.siblings {
		sibling, readErr := decoder.take(32)
		if readErr != nil {
			return materialization{}, readErr
		}
		copy(material.siblings[index][:], sibling)
	}
	if decoder.offset != len(raw) {
		return materialization{}, errors.New("materialization has trailing bytes")
	}
	return material, nil
}
