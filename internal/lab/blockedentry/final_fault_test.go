package blockedentry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestFinalHostileExerciseBindsVariantAndExternalReceipt(t *testing.T) {
	value := fixtureFinalFaultExercise("hostile/G7-forbidden-path/dns/2")
	seed := repeatFinalHex("0", 64)
	if !validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("complete hostile exercise was rejected")
	}
	value.ReceiptSHA256 = ""
	if validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("hostile exercise without an external receipt was accepted")
	}
	value = fixtureFinalFaultExercise("hostile/G7-forbidden-path/dns/2")
	value.Variant = "direct-target"
	if validFinalFaultExercise("hostile/G7-forbidden-path/dns/2", seed, &value) {
		t.Fatal("hostile exercise for another variant was accepted")
	}
	value = fixtureFinalFaultExercise("hostile/G7-forbidden-path/dns/2")
	var receipt finalForbiddenPathReceipt
	if json.Unmarshal(value.Receipt, &receipt) != nil {
		t.Fatal("fixture receipt is invalid")
	}
	receipt.CandidateContract[0]++
	value.Receipt, _ = json.Marshal(receipt)
	digest := sha256.Sum256(value.Receipt)
	value.ReceiptSHA256 = hex.EncodeToString(digest[:])
	if validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("G7 receipt with forbidden path use was accepted")
	}
}

func TestFinalG9RejectedOperationRequiresStableState(t *testing.T) {
	value := fixtureFinalFaultExercise("hostile/G7-forbidden-path/dns/2")
	value.CellID = "hostile/G9-ledger-leakage/duplicate-ordinal/2"
	value.Group = "G9-ledger-leakage"
	value.Variant = "duplicate-ordinal"
	value.Subject = "G9-ledger-leakage/duplicate-ordinal"
	value.InjectionClass = "ledger-mutation"
	value.AfterSHA256 = value.BeforeSHA256
	value.Relation = "same"
	seed := repeatFinalHex("0", 64)
	if !validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("stable G9 rejected operation was rejected")
	}
	value.AfterSHA256 = "03" + repeatFinalHex("0", 62)
	value.Relation = "different"
	if validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("G9 mutation after a rejected operation was accepted")
	}
}

func TestFinalG9UnknownInviteBindsMutationAndStableState(t *testing.T) {
	value := fixtureFinalFaultExercise("hostile/G7-forbidden-path/dns/2")
	value.CellID = "hostile/G9-ledger-leakage/unknown-invite-field/2"
	value.Group = "G9-ledger-leakage"
	value.Variant = "unknown-invite-field"
	value.Subject = "G9-ledger-leakage/unknown-invite-field"
	value.InjectionClass = "ledger-mutation"
	value.AfterSHA256 = value.BeforeSHA256
	value.Relation = "same"
	value.Receipt, _ = json.Marshal(finalUnknownInviteReceipt{Schema: "ardents-h3-g9-unknown-invite-v1",
		BaselineTerminal: "accepted", Terminal: "invalid", BeforeInput: []byte{1, 2}, AfterInput: []byte{1, 2, 0}})
	digest := sha256.Sum256(value.Receipt)
	value.ReceiptSHA256 = hex.EncodeToString(digest[:])
	seed := repeatFinalHex("0", 64)
	if !validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("complete unknown-Invite rejection was rejected")
	}
	value.Receipt, _ = json.Marshal(finalUnknownInviteReceipt{Schema: "ardents-h3-g9-unknown-invite-v1",
		BaselineTerminal: "accepted", Terminal: "invalid", BeforeInput: []byte{1, 2}, AfterInput: []byte{1, 3, 0}})
	digest = sha256.Sum256(value.Receipt)
	value.ReceiptSHA256 = hex.EncodeToString(digest[:])
	if validFinalFaultExercise(value.CellID, seed, &value) {
		t.Fatal("non-trailing unknown-Invite mutation was accepted")
	}
}

func fixtureFinalFaultExercise(cell string) finalFaultExercise {
	input, _ := json.Marshal(struct {
		CandidateEnvelope string   `json:"candidate_envelope"`
		AmbientProxy      []string `json:"ambient_proxy"`
	}{hex.EncodeToString(fixtureG7Envelope()), []string{"http://127.0.0.1:1", "http://127.0.0.1:2", "socks5://127.0.0.1:3"}})
	inputDigest := sha256.Sum256(input)
	contract, _ := json.Marshal(finalG7ComponentContract{Schema: "ardents-h3-g7-component-v1", Variant: "dns",
		Component: "adapter-resolver", Input: input, ReachableTargets: []string{"203.0.113.7:8443"},
		ObservedTargets: []string{"203.0.113.7:8443"}, ChildEnvironment: []string{"TOR_PT_CLIENT_TRANSPORTS=webtunnel",
			"TOR_PT_EXIT_ON_STDIN_CLOSE=1", "TOR_PT_MANAGED_TRANSPORT_VER=1", "TOR_PT_STATE_LOCATION=/tmp/state"},
		StateEntries: []string{}})
	contractDigest := sha256.Sum256(contract)
	receipt, _ := json.Marshal(finalForbiddenPathReceipt{Schema: "ardents-h3-g7-receipt-v2", Variant: "dns",
		Source: "host-alias", Component: "adapter-resolver", InputSHA256: hex.EncodeToString(inputDigest[:]),
		Calls: 1, ContactStarts: 1, Terminal: "bridge-attempt-exhausted", DeadlineOffset: 1,
		CandidateContract: contract, CandidateContractSHA256: hex.EncodeToString(contractDigest[:])})
	digest := sha256.Sum256(receipt)
	return finalFaultExercise{Schema: "ardents-h3-final-fault-exercise-v4", CellID: cell,
		Group: "G7-forbidden-path", Variant: "dns", Episode: "2", InjectionClass: "forbidden-path",
		Subject: "G7-forbidden-path/dns", BeforeSHA256: "01" + repeatFinalHex("0", 62),
		AfterSHA256: "02" + repeatFinalHex("0", 62), Relation: "different",
		Receipt: receipt, ReceiptSHA256: hex.EncodeToString(digest[:]), ActorSHA256: "04" + repeatFinalHex("0", 62),
		SeedSHA256:   finalFaultSeedDigest(repeatFinalHex("0", 64)),
		OffsetMillis: 1, External: true}
}

func fixtureG7Envelope() []byte {
	raw := append([]byte("ardents-h3-wt1"), 1, byte(len("webtunnel-v0.0.6")))
	raw = append(raw, "webtunnel-v0.0.6"...)
	raw = append(raw, 203, 0, 113, 7, 0x20, 0xfb)
	raw = append(raw, 0, byte(len("/entry")))
	raw = append(raw, "/entry"...)
	raw = append(raw, byte(len("front.example")))
	raw = append(raw, "front.example"...)
	raw = append(raw, make([]byte, 32)...)
	raw[len(raw)-1] = 1
	return raw
}

func repeatFinalHex(value string, count int) string {
	result := ""
	for range count {
		result += value
	}
	return result
}
