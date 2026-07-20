package replication

import (
	"sort"

	appdata "ardents/internal/data"
)

type repairBatch struct {
	intentVersion uint64
	blobID        string
	repairs       []appdata.RepairRecord
}

type repairBatchKey struct {
	intentVersion uint64
	blobID        string
}

func batchDueRepairs(repairs []appdata.RepairRecord) []repairBatch {
	ordered := append([]appdata.RepairRecord(nil), repairs...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	batches := make([]repairBatch, 0, len(ordered))
	indices := make(map[repairBatchKey]int, len(ordered))
	for _, repair := range ordered {
		key := repairBatchKey{intentVersion: repair.IntentVersion, blobID: repair.BlobID}
		index, ok := indices[key]
		if !ok {
			index = len(batches)
			indices[key] = index
			batches = append(batches, repairBatch{intentVersion: key.intentVersion, blobID: key.blobID})
		}
		batches[index].repairs = append(batches[index].repairs, repair)
	}
	return batches
}
