package observed

import model "ardents/internal/data/model"

type LocalPayloadInfo func(id string) (present bool, size int64)

func Inventory(objectsCount, manifestsCount int, blobs map[string]model.Blob, localPayload LocalPayloadInfo) model.Inventory {
	out := model.Inventory{
		Objects:   objectsCount,
		Manifests: manifestsCount,
		Blobs:     len(blobs),
	}
	for id, blob := range blobs {
		applyBlobInventory(&out, id, blob, localPayload)
	}
	return out
}

func applyBlobInventory(out *model.Inventory, id string, blob model.Blob, localPayload LocalPayloadInfo) {
	present, size := localPayload(id)
	if blob.Encrypted {
		out.Encrypted++
	}
	applyBlobStateInventory(out, blob, present)
	if !present {
		return
	}
	applyBlobByteInventory(out, blob, size)
}

func applyBlobStateInventory(out *model.Inventory, blob model.Blob, present bool) {
	switch blob.State {
	case "available-local":
		if present {
			out.LocalBlobs++
			out.AvailableForResend++
		}
	case "available-remote":
		out.RemoteBlobs++
	case "retained-temporary":
		if present {
			out.RetainedTemporary++
			out.LocalBlobs++
			out.AvailableForResend++
			if blob.Retention == "relay-temporary" {
				out.RelayRetained++
			}
		}
	case "pinned":
		if present {
			out.Pinned++
			out.LocalBlobs++
			out.AvailableForResend++
		}
	case "expired":
		out.Expired++
	case "deleted":
		out.Deleted++
	}
}

func applyBlobByteInventory(out *model.Inventory, blob model.Blob, size int64) {
	out.LocalBytes += size
	if blob.Retention == "relay-temporary" {
		out.RelayBytes += size
	}
}
