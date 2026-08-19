//go:build live

package network_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type finalFaultExercise struct {
	Schema         string `json:"schema"`
	CellID         string `json:"cell_id"`
	Group          string `json:"group"`
	Variant        string `json:"variant"`
	Episode        string `json:"episode"`
	InjectionClass string `json:"injection_class"`
	Subject        string `json:"subject"`
	BeforeSHA256   string `json:"before_sha256"`
	AfterSHA256    string `json:"after_sha256"`
	Relation       string `json:"relation"`
	ReceiptSHA256  string `json:"receipt_sha256"`
	Receipt        []byte `json:"receipt"`
	ActorSHA256    string `json:"actor_sha256"`
	SeedSHA256     string `json:"seed_sha256"`
	OffsetMillis   uint64 `json:"offset_millis"`
	External       bool   `json:"external"`
}

var finalFaultState struct {
	sync.Mutex
	value *finalFaultExercise
}

func recordFinalFault(cell string, before, after, receipt []byte) {
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[0] != "hostile" || len(before) == 0 || len(after) == 0 ||
		len(receipt) == 0 || len(receipt) > 16<<10 {
		return
	}
	class := finalFaultClass(parts[1])
	beforeHash, afterHash := sha256.Sum256(before), sha256.Sum256(after)
	receiptHash := sha256.Sum256(receipt)
	seed, seedErr := hex.DecodeString(os.Getenv("ARDENTS_FINAL_CELL_SEED"))
	if seedErr != nil || len(seed) != 32 {
		return
	}
	seedHash := sha256.Sum256(seed)
	executable, _ := os.Executable()
	actorHash := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d", executable, os.Getpid())))
	offset := uint64(time.Since(finalWorkerProcessStart).Milliseconds())
	if offset == 0 {
		offset = 1
	}
	finalFaultState.Lock()
	defer finalFaultState.Unlock()
	relation := "different"
	if beforeHash == afterHash {
		relation = "same"
	}
	finalFaultState.value = &finalFaultExercise{Schema: "ardents-h3-final-fault-exercise-v4", CellID: cell,
		Group: parts[1], Variant: parts[2], Episode: parts[3], InjectionClass: class,
		Subject: parts[1] + "/" + parts[2], BeforeSHA256: hex.EncodeToString(beforeHash[:]),
		AfterSHA256: hex.EncodeToString(afterHash[:]), Relation: relation,
		ReceiptSHA256: hex.EncodeToString(receiptHash[:]), Receipt: append([]byte(nil), receipt...),
		ActorSHA256: hex.EncodeToString(actorHash[:]), SeedSHA256: hex.EncodeToString(seedHash[:]),
		OffsetMillis: offset, External: true}
}

func finalFaultClass(group string) string {
	return map[string]string{"G1-invite": "input-mutation", "G2-domain-collision": "state-mutation",
		"G3-replay-replacement": "state-mutation", "G4-restart": "lifecycle-fault",
		"G5-adapter-fault": "adapter-fault", "G6-substitution": "binding-substitution",
		"G7-forbidden-path": "forbidden-path", "G8-lifecycle": "lifecycle-fault",
		"G9-ledger-leakage": "ledger-mutation"}[group]
}

func consumeFinalFaultExercise(cell string) *finalFaultExercise {
	finalFaultState.Lock()
	defer finalFaultState.Unlock()
	value := finalFaultState.value
	finalFaultState.value = nil
	if value == nil || value.CellID != cell {
		return nil
	}
	return value
}
