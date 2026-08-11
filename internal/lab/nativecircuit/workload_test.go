package nativecircuit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQualificationWorkloadIsExact(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "workload.json")
	valid := `{"schema_version":"carrier-lab-native-workload/v1","profile":"c5-c2","kind":"stream","direction":"user-to-service","duration_seconds":60,"seed":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	workload, err := readNativeWorkload(path)
	if err != nil {
		t.Fatal(err)
	}
	if workload.Direction != streamUpload || workload.DurationSeconds != 60 {
		t.Fatalf("unexpected workload: %+v", workload)
	}
}

func TestQualificationWorkloadRejectsUnknownAndShortenedInputs(t *testing.T) {
	t.Parallel()
	tests := []string{
		`{"schema_version":"carrier-lab-native-workload/v1","profile":"c5-c2","kind":"stream","direction":"user-to-service","duration_seconds":1,"seed":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`,
		`{"schema_version":"carrier-lab-native-workload/v1","profile":"c5-c2","kind":"stream","direction":"user-to-service","duration_seconds":60,"seed":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","extra":true}`,
	}
	for index, input := range tests {
		path := filepath.Join(t.TempDir(), "workload.json")
		if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readNativeWorkload(path); err == nil {
			t.Fatalf("case %d accepted an invalid workload", index)
		}
	}
}
