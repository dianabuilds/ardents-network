package platform

import "testing"

func TestUbuntu2404RequiresExactIdentityAndUniqueKeys(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{"ID=ubuntu\nVERSION_ID=\"24.04\"\n", true},
		{"ID=debian\nVERSION_ID=24.04\n", false},
		{"ID=ubuntu\nVERSION_ID=22.04\n", false},
		{"ID=ubuntu\nID=ubuntu\nVERSION_ID=24.04\n", false},
		{"ID=ubuntu\nVERSION_ID=24.04\nNAME=a\nNAME=b\n", false},
	} {
		if got := Ubuntu2404([]byte(test.value)); got != test.want {
			t.Fatalf("%q: %v", test.value, got)
		}
	}
}
