package stage6evidence

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

type controlCorpus struct {
	order      namespace.ClaimOrder
	records    []namespace.Record
	operations []controlOperation
}

func newControlCorpus(network [32]byte, now time.Time, policy namespace.Policy) (controlCorpus, error) {
	var corpus controlCorpus
	longLease := now.Add(100 * 24 * time.Hour)
	fixtures := []struct {
		name, key string
	}{
		{"renew-name", "renew"}, {"record-name", "record"}, {"release-name", "release"},
		{"transfer-name", "transfer"}, {"root", "delegate-parent"}, {"policy-name", "policy"},
		{"policy-disable-name", "policy-disable"}, {"recovery-name", "recovery"},
		{"recovery-cancel-name", "recovery-cancel"}, {"recovery-complete-name", "recovery-complete"},
		{"recovery-resume-name", "recovery-resume"},
	}
	keys := make(map[string]ed25519.PrivateKey, len(fixtures))
	for _, fixture := range fixtures {
		key := evidenceKey("control-" + fixture.key)
		keys[fixture.name] = key
		corpus.records = append(corpus.records, controlRecord(fixture.name, key, longLease))
	}
	claimKey := evidenceKey("control-claim")
	claim := evidenceNamedClaimFor(network, 12, 0, [32]byte{1}, claimKey, "claim-root")
	order, claimProof := evidenceClaimSetFor(network, 12, []namespace.Claim{claim})
	corpus.order = order
	claimRaw, err := namespace.CanonicalProof(nil, &claimProof)
	if err != nil {
		return corpus, err
	}
	corpus.operations = append(corpus.operations, controlOperation{Kind: "claim", Name: "claim-root",
		Generation: 1, Authority: publicBytes(claimKey), LeaseNotAfter: now.Add(time.Hour).UnixMilli(),
		OrderingProof: claimRaw})
	for _, kind := range []string{"renew", "record", "release", "transfer"} {
		name := kind + "-name"
		current := recordByName(corpus.records, name)
		op := namespace.Op{Kind: kind, Name: name, Authority: current.Authority,
			ExpectedGeneration: 1, ExpectedRevision: 1}
		operation := controlOperation{Kind: kind, Name: name, Generation: 1, ExpectedRevision: 1}
		switch kind {
		case "renew":
			op.LeaseDuration, operation.LeaseNotAfter = policy.DefaultLeaseDuration,
				now.Add(policy.DefaultLeaseDuration).UnixMilli()
		case "record":
			op.Kind, op.Target = "publish", [32]byte{12}
			operation.Target, operation.RecordNotAfter = op.Target, now.Add(time.Hour).UnixMilli()
		case "transfer":
			successor := evidenceKey("control-transfer-successor")
			op.SuccessorAuthority = hex.EncodeToString(successor.Public().(ed25519.PublicKey))
			operation.SuccessorAuthority = publicBytes(successor)
		}
		operation.AuthorityProof, err = namespace.SignTransition(network, current, op, keys[name])
		if err != nil {
			return corpus, err
		}
		corpus.operations = append(corpus.operations, operation)
	}
	parent := recordByName(corpus.records, "root")
	childKey := evidenceKey("control-child")
	childOp := namespace.Op{Kind: "claim", Name: "leaf.root", Generation: 1,
		Authority: hex.EncodeToString(childKey.Public().(ed25519.PublicKey)), Parents: []namespace.Record{parent},
		LeaseDuration: policy.DefaultLeaseDuration}
	childProof, err := namespace.SignTransition(network, parent, childOp, keys["root"])
	if err != nil {
		return corpus, err
	}
	corpus.operations = append(corpus.operations, controlOperation{Kind: "delegate", Name: "leaf.root",
		ParentName: "root", ParentGeneration: 1, ParentRevision: 1, ChildGeneration: 1,
		Authority: publicBytes(childKey), LeaseNotAfter: now.Add(policy.DefaultLeaseDuration).UnixMilli(),
		AuthorityProof: childProof})
	recoveryOperations, err := recoveryControlOperations(network, now, corpus.records, keys)
	if err != nil {
		return corpus, err
	}
	corpus.operations = append(corpus.operations, recoveryOperations...)
	for index := range corpus.operations {
		corpus.operations[index].OperationDigest = controlShapeDigest(corpus.operations[index])
	}
	return corpus, nil
}

func controlRecord(name string, key ed25519.PrivateKey, lease time.Time) namespace.Record {
	return namespace.Record{Name: name, Generation: 1, Revision: 1, Lease: "active", Consistency: "current",
		Recovery: "stable", Authority: hex.EncodeToString(key.Public().(ed25519.PublicKey)),
		LeaseExpiresAt: lease.Unix(), GraceExpiresAt: lease.Add(time.Hour).Unix(), Continuity: 1}
}

func controlRecoveryPolicy(network [32]byte, record namespace.Record,
	current ed25519.PrivateKey,
) (namespace.RecoveryPolicy, []ed25519.PrivateKey) {
	policy := namespace.RecoveryPolicy{Network: network, Name: record.Name, Generation: 1, Revision: 1,
		CurrentAuthority: publicBytes(current), Threshold: 2, Delay: 72 * time.Hour}
	signers := []ed25519.PrivateKey{evidenceKey("control-recovery-1"), evidenceKey("control-recovery-2")}
	sort.Slice(signers, func(i, j int) bool {
		return bytes.Compare(signers[i].Public().(ed25519.PublicKey), signers[j].Public().(ed25519.PublicKey)) < 0
	})
	for _, signer := range signers {
		policy.Participants = append(policy.Participants, publicBytes(signer))
	}
	return policy, signers
}

func publicBytes(key ed25519.PrivateKey) [32]byte {
	var out [32]byte
	copy(out[:], key.Public().(ed25519.PublicKey))
	return out
}

func recordByName(records []namespace.Record, name string) namespace.Record {
	return records[recordIndex(records, name)]
}
func recordIndex(records []namespace.Record, name string) int {
	for index := range records {
		if records[index].Name == name {
			return index
		}
	}
	return -1
}
