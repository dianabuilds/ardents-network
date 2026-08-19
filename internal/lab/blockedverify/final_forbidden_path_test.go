package blockedverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestFinalG7ReceiptRecomputesRawComponentObservation(t *testing.T) {
	raw := fixtureFinalG7Receipt(t)
	if !validFinalForbiddenPathReceipt(raw, "dns") {
		t.Fatal("complete raw G7 component observation was rejected")
	}
	var receipt finalForbiddenPathReceipt
	if json.Unmarshal(raw, &receipt) != nil {
		t.Fatal("decode fixture receipt")
	}
	var contract finalG7ComponentContract
	if json.Unmarshal(receipt.CandidateContract, &contract) != nil {
		t.Fatal("decode fixture component contract")
	}
	contract.ObservedTargets[0] = "203.0.113.8:8443"
	receipt.CandidateContract, _ = json.Marshal(contract)
	digest := sha256.Sum256(receipt.CandidateContract)
	receipt.CandidateContractSHA256 = hex.EncodeToString(digest[:])
	raw, _ = json.Marshal(receipt)
	if validFinalForbiddenPathReceipt(raw, "dns") {
		t.Fatal("component observation inconsistent with the candidate envelope was accepted")
	}
}

func fixtureFinalG7Receipt(t *testing.T) []byte {
	t.Helper()
	envelope := append([]byte("ardents-h3-wt1"), 1, byte(len("webtunnel-v0.0.6")))
	envelope = append(envelope, "webtunnel-v0.0.6"...)
	envelope = append(envelope, 203, 0, 113, 7, 0x20, 0xfb)
	envelope = append(envelope, 0, byte(len("/entry")))
	envelope = append(envelope, "/entry"...)
	envelope = append(envelope, byte(len("front.example")))
	envelope = append(envelope, "front.example"...)
	envelope = append(envelope, make([]byte, 32)...)
	envelope[len(envelope)-1] = 1
	input, _ := json.Marshal(struct {
		CandidateEnvelope string   `json:"candidate_envelope"`
		AmbientProxy      []string `json:"ambient_proxy"`
	}{hex.EncodeToString(envelope), []string{"http://127.0.0.1:1", "http://127.0.0.1:2", "socks5://127.0.0.1:3"}})
	contract, _ := json.Marshal(finalG7ComponentContract{Schema: "ardents-h3-g7-component-v1", Variant: "dns",
		Component: "adapter-resolver", Input: input, ReachableTargets: []string{"203.0.113.7:8443"},
		ObservedTargets: []string{"203.0.113.7:8443"}, ChildEnvironment: []string{"TOR_PT_CLIENT_TRANSPORTS=webtunnel",
			"TOR_PT_EXIT_ON_STDIN_CLOSE=1", "TOR_PT_MANAGED_TRANSPORT_VER=1", "TOR_PT_STATE_LOCATION=/tmp/state"},
		StateEntries: []string{}})
	inputDigest, contractDigest := sha256.Sum256(input), sha256.Sum256(contract)
	receipt, _ := json.Marshal(finalForbiddenPathReceipt{Schema: "ardents-h3-g7-receipt-v2", Variant: "dns",
		Source: "host-alias", Component: "adapter-resolver", InputSHA256: hex.EncodeToString(inputDigest[:]),
		Calls: 1, ContactStarts: 1, Terminal: "bridge-attempt-exhausted", DeadlineOffset: 1,
		CandidateContract: contract, CandidateContractSHA256: hex.EncodeToString(contractDigest[:])})
	return receipt
}
