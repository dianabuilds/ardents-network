package blockedverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestFinalHostileExerciseRequiresIndependentCryptographicBindings(t *testing.T) {
	seed := finalFaultDigest('0')
	receipt := []byte("external hostile receipt")
	digest := sha256.Sum256(receipt)
	value := finalFaultExercise{Schema: "ardents-h3-final-fault-exercise-v4",
		CellID: "hostile/G9-ledger-leakage/duplicate-ordinal/4", Group: "G9-ledger-leakage",
		Variant: "duplicate-ordinal", Episode: "4", InjectionClass: "ledger-mutation",
		Subject: "G9-ledger-leakage/duplicate-ordinal", BeforeSHA256: finalFaultDigest('1'),
		AfterSHA256: finalFaultDigest('1'), Relation: "same", Receipt: receipt,
		ReceiptSHA256: hex.EncodeToString(digest[:]),
		ActorSHA256:   finalFaultDigest('4'), SeedSHA256: finalFaultSeedDigest(seed), OffsetMillis: 7, External: true}
	if !validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("complete hostile exercise was rejected")
	}
	value.Relation, value.AfterSHA256 = "different", finalFaultDigest('2')
	if validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("G9 mutation after a rejected operation was accepted")
	}
	value.Relation, value.AfterSHA256, value.ActorSHA256 = "same", finalFaultDigest('1'), finalFaultDigest('z')
	if validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("non-hex actor identity was accepted")
	}
	value.ActorSHA256 = finalFaultDigest('4')
	value.Receipt[0]++
	if validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("hostile receipt whose bytes differ from its commitment was accepted")
	}
}

func TestFinalUnknownInviteRequiresRawMutationAndStableState(t *testing.T) {
	seed := finalFaultDigest('0')
	receipt, _ := json.Marshal(finalUnknownInviteReceipt{Schema: "ardents-h3-g9-unknown-invite-v1",
		BaselineTerminal: "accepted", Terminal: "invalid", BeforeInput: []byte{1, 2}, AfterInput: []byte{1, 2, 0}})
	digest := sha256.Sum256(receipt)
	value := finalFaultExercise{Schema: "ardents-h3-final-fault-exercise-v4",
		CellID: "hostile/G9-ledger-leakage/unknown-invite-field/4", Group: "G9-ledger-leakage",
		Variant: "unknown-invite-field", Episode: "4", InjectionClass: "ledger-mutation",
		Subject: "G9-ledger-leakage/unknown-invite-field", BeforeSHA256: finalFaultDigest('1'),
		AfterSHA256: finalFaultDigest('1'), Relation: "same", Receipt: receipt,
		ReceiptSHA256: hex.EncodeToString(digest[:]), ActorSHA256: finalFaultDigest('4'),
		SeedSHA256: finalFaultSeedDigest(seed), OffsetMillis: 7, External: true}
	if !validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("complete unknown-Invite rejection was rejected")
	}
	value.AfterSHA256, value.Relation = finalFaultDigest('2'), "different"
	if validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("unknown-Invite durable state mutation was accepted")
	}
}

func finalFaultDigest(value byte) string {
	result := make([]byte, 64)
	for index := range result {
		result[index] = value
	}
	return string(result)
}
