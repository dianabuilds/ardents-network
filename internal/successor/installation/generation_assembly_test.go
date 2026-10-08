package installation

import (
	"bytes"
	"testing"
)

func inventoryFixture(t *testing.T) (Request, map[string][]byte, generationSelection) {
	t.Helper()
	selected, binding, files := installedObservationFixture(t)
	raw, err := canonicalJSON(binding)
	if err != nil {
		t.Fatal(err)
	}
	files["binding.json"] = raw
	request, err := DecodeRequest(t.Context(), files["request.json"], true)
	if err != nil {
		t.Fatal(err)
	}
	return request, files, selected
}

func TestGenerationInventoryFreezeRetainsDetachedClosedBytes(t *testing.T) {
	request, files, selected := inventoryFixture(t)
	frozen, _, err := freezeGenerationInventory(request, "/installation", files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(frozen["ardents-linux-amd64"])
	files["ardents-linux-amd64"][0] ^= 1
	if !bytes.Equal(frozen["ardents-linux-amd64"], original) {
		t.Fatal("caller changed frozen program bytes")
	}
	for _, change := range []string{"missing", "extra", "changed", "foreign-root", "foreign-group", "binding-digest"} {
		request, files, selected := inventoryFixture(t)
		root, gid := "/installation", uint32(65534)
		switch change {
		case "missing":
			delete(files, "ardents-text-reader.socket")
		case "extra":
			files["foreign"] = []byte("foreign")
		case "changed":
			files["ardents-linux-amd64"][0] ^= 1
		case "foreign-root":
			root = "/foreign"
		case "foreign-group":
			gid = 65533
		case "binding-digest":
			selected.BindingDigest = selected.GenerationDigest
		}
		if frozen, _, err := freezeGenerationInventory(request, root, files, selected, gid); err == nil || frozen != nil {
			t.Fatal("changed staging inventory accepted", change, err)
		}
	}
}
