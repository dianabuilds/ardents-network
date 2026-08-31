package administration

import (
	"encoding/json"
	"os"
	"testing"
)

func TestVersionedConformanceVectors(t *testing.T) {
	t.Parallel()
	var vector struct {
		Schema           string `json:"schema"`
		InterfaceVersion string `json:"interface_version"`
		Cases            []struct {
			Operation Operation `json:"operation"`
			Request   string    `json:"request"`
			Success   string    `json:"success"`
		} `json:"cases"`
		Failure string `json:"failure"`
	}
	raw, err := os.ReadFile("testdata/conformance-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	if vector.Schema != "ardents-application-interface-conformance-v1" || vector.InterfaceVersion != InterfaceVersion || vector.Failure != "unavailable\n" {
		t.Fatalf("Administration vector header = %q / %q / %q", vector.Schema, vector.InterfaceVersion, vector.Failure)
	}
	want := map[Operation][2]string{Publish: {"publish\n", "published\n"}, Withdraw: {"withdraw\n", "withdrawn\n"}}
	if len(vector.Cases) != len(want) {
		t.Fatalf("Administration vector cases = %d", len(vector.Cases))
	}
	for _, test := range vector.Cases {
		if got, found := want[test.Operation]; !found || got != [2]string{test.Request, test.Success} {
			t.Errorf("Administration vector case = %+v", test)
		}
	}
}
