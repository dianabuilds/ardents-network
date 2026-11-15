package generation

import "testing"

func TestInventoryIsClosedAndDetached(t *testing.T) {
	expected := []string{"ardents-linux-amd64", "ardents-text-linux-amd64", "ardents-text-reader@.service", "ardents-text-publisher@.service", "ardents-text-reader.socket", "ardents-text-publisher.socket", "50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service", "protected-endpoint.json", "request.json", "headless.json", "source.json", "endpoint-unit.service", "binding.json"}
	first := Names()
	if len(first) != len(expected) {
		t.Fatalf("inventory length %d", len(first))
	}
	for i, name := range expected {
		if first[i] != name {
			t.Fatalf("inventory entry %d: %q", i, first[i])
		}
	}
	first[0] = "foreign"
	if Names()[0] != expected[0] {
		t.Fatal("caller changed the inventory")
	}
}
