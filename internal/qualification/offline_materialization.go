package qualification

import (
	"bytes"
	"errors"
)

type offlineMaterialization struct {
	digest   [32]byte
	index    uint32
	record   []byte
	siblings [][32]byte
}

func verifyOfflineMaterials(epoch offlineEpoch, accepted []verifiedRecord, rawMaterials [][]byte) error {
	if len(rawMaterials) > 64 {
		return errors.New("independent materialization set exceeds 64 entries")
	}
	if len(accepted) > 0 && len(rawMaterials) == 0 {
		return errors.New("independent materialization evidence is missing")
	}
	seen := make(map[uint32]bool, len(rawMaterials))
	for _, raw := range rawMaterials {
		material, err := decodeOfflineMaterial(raw)
		if err != nil {
			return err
		}
		if material.digest != epoch.digest || material.index >= uint32(len(accepted)) || seen[material.index] {
			return errors.New("independent materialization identity is invalid")
		}
		seen[material.index] = true
		record := accepted[material.index].raw
		if !bytes.Equal(record, material.record) ||
			!proofMatches(record, material.index, uint32(len(accepted)), material.siblings, epoch.viewRoot) {
			return errors.New("independent materialization proof is invalid")
		}
	}
	return nil
}

func decodeOfflineMaterial(raw []byte) (offlineMaterialization, error) {
	if len(raw) < 42 {
		return offlineMaterialization{}, errors.New("independent materialization is truncated")
	}
	d := byteDecoder{raw: raw}
	var material offlineMaterialization
	digest, err := d.take(32)
	if err != nil {
		return offlineMaterialization{}, err
	}
	copy(material.digest[:], digest)
	if material.index, err = d.u32(); err != nil {
		return offlineMaterialization{}, err
	}
	recordLength, err := d.u32()
	if err != nil || recordLength == 0 || recordLength > 32<<10 {
		return offlineMaterialization{}, errors.New("independent materialization record length is invalid")
	}
	material.record, err = d.take(int(recordLength))
	if err != nil {
		return offlineMaterialization{}, err
	}
	count, err := d.u16()
	if err != nil || count > 64 {
		return offlineMaterialization{}, errors.New("independent materialization proof length is invalid")
	}
	material.siblings = make([][32]byte, count)
	for index := range material.siblings {
		sibling, readErr := d.take(32)
		if readErr != nil {
			return offlineMaterialization{}, readErr
		}
		copy(material.siblings[index][:], sibling)
	}
	if d.offset != len(raw) {
		return offlineMaterialization{}, errors.New("independent materialization has trailing bytes")
	}
	return material, nil
}
