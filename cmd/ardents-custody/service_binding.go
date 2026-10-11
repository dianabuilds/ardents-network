package main

import (
	"errors"
	legacycustody "github.com/dianabuilds/ardents-network/internal/custody"
)

func legacyCommandBinding(environment, network, root, kind, identity string) (legacycustody.AuthorityBinding, error) {
	var binding legacycustody.AuthorityBinding
	for _, value := range []struct {
		text string
		dest []byte
	}{
		{environment, binding.Environment[:]},
		{network, binding.Network[:]},
		{root, binding.Root[:]},
		{identity, binding.IDCommitment[:]},
	} {
		if err := decodeCommandCommitment(value.text, value.dest); err != nil {
			return legacycustody.AuthorityBinding{}, errors.New("custody operation requires lowercase SHA-256 commitments")
		}
	}
	binding.Kind = legacycustody.AuthorityKind(kind)
	if binding.Kind != legacycustody.AuthorityService && binding.Kind != legacycustody.AuthorityName && binding.Kind != legacycustody.AuthorityAdmission {
		return legacycustody.AuthorityBinding{}, errors.New("custody operation kind must be service, name, or admission")
	}
	return binding, nil
}
