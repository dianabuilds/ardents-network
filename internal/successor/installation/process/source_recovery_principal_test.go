package process

import "testing"

func TestSourceRecoveryRequiresCompleteProcVisibility(t *testing.T) {
	for _, test := range []struct {
		mount string
		valid bool
	}{
		{"10 1 0:5 / /proc rw,nosuid,nodev,noexec - proc proc rw\n", true},
		{"10 1 0:5 / /proc rw - proc proc rw,hidepid=0\n", true},
		{"10 1 0:5 / /proc rw - proc proc rw,hidepid=2\n", false},
		{"10 1 0:5 / /proc rw - proc proc rw,hidepid=4\n", false},
		{"10 1 0:5 / /proc rw - proc proc rw,subset=pid\n", false},
		{"10 1 0:5 /996 /proc rw - proc proc rw\n", false},
		{"10 1 0:5 / /proc rw - tmpfs tmpfs rw\n", false},
		{"", false},
	} {
		if (sourceRecoveryProcMount([]byte(test.mount)) == nil) != test.valid {
			t.Fatalf("incorrect proc visibility for %q", test.mount)
		}
	}
}

func TestSourceRecoveryRetainsAnyOriginalPrincipalCredential(t *testing.T) {
	for _, test := range []struct {
		body              string
		retained, invalid bool
	}{
		{"Uid:\t996 996 996 996\n", true, false},
		{"Uid:\t0 996 0 0\n", true, false},
		{"Uid:\t0 0 996 0\n", true, false},
		{"Uid:\t0 0 0 996\n", true, false},
		{"Uid:\t0 0 0 0\n", false, false},
		{"Gid:\t996 996 996 996\n", false, true},
		{"Uid:\t996 996\n", false, true},
		{"Uid:\t-1 0 0 0\n", false, true},
		{"Uid:\t0996 0 0 0\n", false, true},
		{"Uid:\t0 0 0 0\nUid:\t996 996 996 996\n", false, true},
	} {
		retained, err := sourceRecoveryPrincipal([]byte(test.body), 996)
		if (err != nil) != test.invalid || !test.invalid && retained != test.retained {
			t.Fatalf("credential visibility = %t, %v for %q", retained, err, test.body)
		}
	}
}
