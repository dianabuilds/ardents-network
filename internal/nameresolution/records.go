package nameresolution

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namelease"
)

func newRecordSet(network [32]byte, chains [][][]byte) (recordSet, error) {
	if network == [32]byte{} || len(chains) == 0 {
		return recordSet{}, errors.New("private resolution Record set is empty")
	}
	result := recordSet{network: network, chains: make(map[string]recordChain, len(chains))}
	for _, chain := range chains {
		head, copied, err := validateSignedChain(network, chain)
		name := head.Name
		_, duplicate := result.chains[name]
		if err != nil || duplicate {
			return recordSet{}, errors.New("private resolution Record chain is invalid or duplicated")
		}
		sample, encodeErr := encodeResponse(resolutionResponse{network: network, nonce: [32]byte{1}, deadline: 1,
			name: name, generation: head.Generation, revision: head.Revision, result: resultResolved, chain: copied})
		if encodeErr != nil {
			return recordSet{}, errors.New("private resolution Record chain does not fit the fixed response")
		}
		if _, padErr := padMessage(sample); padErr != nil {
			return recordSet{}, errors.New("private resolution Record chain does not fit the fixed response")
		}
		result.chains[name] = recordChain{head: head, signed: copied}
	}
	return result, nil
}

func validateSignedChain(network [32]byte, chain [][]byte) (namelease.Record, [][]byte, error) {
	if len(chain) == 0 || len(chain) > 127 {
		return namelease.Record{}, nil, errors.New("signed Record chain has invalid depth")
	}
	records := make([]namelease.Record, len(chain))
	copied := make([][]byte, len(chain))
	for index, signed := range chain {
		record, err := nameauthority.VerifyRecord(network, signed)
		if err != nil {
			return namelease.Record{}, nil, err
		}
		records[index] = record
		copied[index] = append([]byte(nil), signed...)
	}
	for index := 1; index < len(records); index++ {
		child, parent := records[index-1], records[index]
		if child.ParentName != parent.Name || child.ParentGeneration != parent.Generation {
			return namelease.Record{}, nil, errors.New("signed Record chain is discontinuous")
		}
	}
	if records[len(records)-1].ParentName != "" {
		return namelease.Record{}, nil, errors.New("signed Record chain does not reach a root")
	}
	return records[0], copied, nil
}

func (records recordSet) lookup(name string) (recordChain, bool) {
	chain, ok := records.chains[name]
	if !ok {
		return recordChain{}, false
	}
	copied := make([][]byte, len(chain.signed))
	for index := range chain.signed {
		copied[index] = append([]byte(nil), chain.signed[index]...)
	}
	chain.signed = copied
	return chain, true
}
