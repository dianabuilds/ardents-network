package recoverysmoke

import "testing"

func TestReplacementCampaignPrecommitsExactMatrix(t *testing.T) {
	cells, err := replacementCampaignCells(map[string][32]byte{
		"client-to-publisher": {1}, "publisher-to-client": {2},
	})
	if err != nil {
		t.Fatal(err)
	}
	wanted := []string{"c2p-isolated-initiator", "c2p-isolated-introduction", "c2p-isolated-rendezvous",
		"c2p-isolated-responder", "c2p-sequential-three", "p2c-isolated-initiator",
		"p2c-isolated-introduction", "p2c-isolated-rendezvous", "p2c-isolated-responder", "p2c-sequential-three"}
	if len(cells) != len(wanted) {
		t.Fatalf("cells = %d", len(cells))
	}
	for index, cell := range cells {
		if cell.CellID != wanted[index] || cell.ManifestDigest == "" {
			t.Fatalf("cell %d = %+v", index, cell)
		}
	}
}
