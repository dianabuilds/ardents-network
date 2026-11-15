package endpoint

import (
	"strings"
	"testing"
)

func TestBoundDeclarationsKeepClosedSchemaAndSourceCount(t *testing.T) {
	headless := []byte(`{"schema":"ardents-headless-runtime-v2","role":"reader","network_id":"network"}`)
	source := []byte(`{"schema":"ardents-source-plan-v1","sources":[{"address":"one"},{"address":"two"}]}`)
	p, s, err := Declarations(headless, source)
	if err != nil || p.NetworkID != "network" || p.Role != "reader" || len(s.Sources) != 2 || s.Sources[1].Address != "two" {
		t.Fatal("bound declarations changed", p, s, err)
	}
	for _, tc := range []struct{ headless, source string }{
		{`{"schema":"ardents-headless-runtime-v1"}`, string(source)},
		{string(headless), `{"schema":"ardents-source-plan-v1","sources":[{}]}`},
		{string(headless), `{"schema":"other","sources":[{},{}]}`},
		{strings.Repeat(" ", (16<<10)+1), string(source)},
		{string(headless), strings.Repeat(" ", (32<<10)+1)},
	} {
		if _, _, err := Declarations([]byte(tc.headless), []byte(tc.source)); err == nil {
			t.Fatal("invalid bound declarations accepted")
		}
	}
}
