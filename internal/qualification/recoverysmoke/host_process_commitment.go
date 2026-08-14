package recoverysmoke

import (
	"crypto/sha256"
	"encoding/binary"
)

type processEvidenceRef struct {
	Adapter                 string
	Scope, Executable, Tree [32]byte
	Commitment              [32]byte
	Identity, Incarnation   string
}

func bindProcessRef(value processRef) processEvidenceRef {
	result := processEvidenceRef{Adapter: value.Adapter, Scope: value.Scope,
		Executable: value.Executable, Tree: value.Tree,
		Identity: value.Identity, Incarnation: value.Incarnation}
	result.Commitment = processRefCommitment(result)
	return result
}

func processRefCommitment(value processEvidenceRef) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-qualification-host-process-v1\x00"))
	_, _ = hash.Write(value.Scope[:])
	_, _ = hash.Write(value.Executable[:])
	_, _ = hash.Write(value.Tree[:])
	for _, field := range []string{value.Adapter, value.Identity, value.Incarnation} {
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(field)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(field))
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func processObservationCommitment(ref processEvidenceRef, projection []byte, pid uint32,
	running bool, observedAt int64) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-qualification-process-observation-v1\x00"))
	_, _ = hash.Write(ref.Commitment[:])
	projectionDigest := sha256.Sum256(projection)
	_, _ = hash.Write(projectionDigest[:])
	var encoded [8]byte
	binary.BigEndian.PutUint32(encoded[:4], pid)
	if running {
		encoded[4] = 1
	}
	_, _ = hash.Write(encoded[:5])
	binary.BigEndian.PutUint64(encoded[:], uint64(observedAt))
	_, _ = hash.Write(encoded[:])
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}
