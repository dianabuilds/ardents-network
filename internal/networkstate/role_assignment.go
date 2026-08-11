package networkstate

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

func verifyDomainSummaries(epoch epochEnvelope, families map[string][2]uint32) error {
	computed := make(map[string][2]uint32, len(epoch.domains))
	for _, domain := range epoch.domains {
		computed[domain.id] = [2]uint32{}
	}
	for family, summary := range families {
		domain, err := assignedDomain(epoch, family)
		if err != nil {
			return err
		}
		current := computed[domain]
		current[0] += summary[0]
		current[1] += summary[1]
		computed[domain] = current
	}
	for _, expected := range epoch.domains {
		actual := computed[expected.id]
		if uint16(actual[0]) != expected.count || actual[1] != expected.capacity {
			return errors.New("role domain summaries are inconsistent")
		}
	}
	return nil
}

func assignedDomain(epoch epochEnvelope, family string) (string, error) {
	var selected string
	var selectedDigest [32]byte
	for index, domain := range epoch.domains {
		digest := assignmentDigest(epoch, family, domain.id)
		if index > 0 && digest == selectedDigest {
			return "", errors.New("role assignment digest tie")
		}
		if selected == "" || bytes.Compare(digest[:], selectedDigest[:]) < 0 {
			selected, selectedDigest = domain.id, digest
		}
	}
	return selected, nil
}

func assignmentDigest(epoch epochEnvelope, family, domain string) [32]byte {
	encoded := make([]byte, 0, 27+32+8+32+len(family)+len(domain))
	encoded = append(encoded, []byte("ardents-h3-role-domain-v1\x00")...)
	encoded = append(encoded, epoch.networkID[:]...)
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], epoch.number)
	encoded = append(encoded, number[:]...)
	encoded = append(encoded, epoch.assignmentSeed[:]...)
	encoded = append(encoded, family...)
	encoded = append(encoded, domain...)
	return sha256.Sum256(encoded)
}
