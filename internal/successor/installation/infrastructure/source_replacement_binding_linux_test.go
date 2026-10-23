package infrastructure

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func sourceReplacementBindingVector() []byte {
	configuration := strings.TrimSuffix(string(sourceBindingVector()), "\n")
	configuration = strings.Replace(configuration, "ardents-source-binding-v1", "ardents-source-replacement-configuration-v1", 1)
	configuration = strings.Replace(configuration, `"predecessor":""`, `"predecessor":"`+strings.Repeat("f", 64)+`"`, 1)
	return fmt.Appendf(nil, `{"schema":"ardents-source-replacement-binding-v1","configuration":%s,"initial_generation":"%s","predecessor_binding_sha256":"%s","retirement_sha256":"%s","predecessor_attempt":0}`+"\n", configuration, strings.Repeat("f", 64), strings.Repeat("1", 64), strings.Repeat("2", 64))
}

func TestSourceReplacementBindingRetainsDistinctProvenanceGrammar(t *testing.T) {
	raw := sourceReplacementBindingVector()
	b, err := decodeSourceReplacementBinding(raw)
	if err != nil || b.Configuration.Predecessor != strings.Repeat("f", 64) || b.PredecessorAttempt != 0 {
		t.Fatal("independent replacement vector", err)
	}
	if _, err := decodeSourceBinding(raw); err == nil {
		t.Fatal("replacement entered initial grammar")
	}
	if _, err := decodeSourceReplacementBinding(sourceBindingVector()); err == nil {
		t.Fatal("initial entered replacement grammar")
	}
	projected, provenance, err := decodeSourceSelectedBinding(raw)
	if err != nil || provenance == nil || projected.Generation != strings.Repeat("a", 64) {
		t.Fatal("configuration projection", err)
	}
	for name, change := range map[string][2]string{
		"arbitrary-predecessor": {strings.Repeat("f", 64), "operator-supplied"},
		"self-predecessor":      {strings.Repeat("f", 64), strings.Repeat("a", 64)},
		"initial-configuration": {"ardents-source-replacement-configuration-v1", "ardents-source-binding-v1"},
		"missing-retirement":    {`"retirement_sha256":"` + strings.Repeat("2", 64) + `",`, ""},
		"foreign-retirement":    {strings.Repeat("2", 64), strings.Repeat("z", 64)},
		"duplicate-attempt":     {`"predecessor_attempt":0`, `"predecessor_attempt":0,"predecessor_attempt":0`},
		"attempt-exhaustion":    {`"predecessor_attempt":0`, `"predecessor_attempt":63`},
		"attempt-overflow":      {`"predecessor_attempt":0`, `"predecessor_attempt":4294967295`},
		"unknown-authority":     {`"schema":`, `"authorized":true,"schema":`},
	} {
		t.Run(name, func(t *testing.T) {
			altered := bytes.Replace(raw, []byte(change[0]), []byte(change[1]), 1)
			if bytes.Equal(altered, raw) {
				t.Fatal("control did not mutate")
			}
			if _, err := decodeSourceReplacementBinding(altered); err == nil {
				t.Fatal("foreign provenance accepted")
			}
		})
	}
}
