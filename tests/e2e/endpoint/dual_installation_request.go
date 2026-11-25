package endpoint

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// DualInstallationRequest projects the independent request into the dual-role scenario.
func DualInstallationRequest(t testing.TB, raw, base string) string {
	t.Helper()
	quoted := func(value string) string {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	for _, change := range [][2]string{
		{`"role":"reader",`, ""},
		{`"maxima":[1,0,0]`, `"maxima":[32,32,0]`},
		{`"publisher_permission":{"request_path":"","response_path":"","maxima":[0,0,0]}`,
			`"publisher_permission":{"request_path":` + quoted(filepath.Join(base, "publisher-permission", "request")) + `,"response_path":` + quoted(filepath.Join(base, "publisher-permission", "response")) + `,"maxima":[32,32,8]}`},
		{`"administration_principal":""`, `"administration_principal":` + quoted(hex.EncodeToString(bytes.Repeat([]byte{9}, 32)))},
		{`"administration_socket":""`, `"administration_socket":` + quoted(filepath.Join(base, "administration", "socket"))},
		{`"publication_root":""`, `"publication_root":` + quoted(filepath.Join(base, "publication")) + `,"service_instance_root":` + quoted(filepath.Join(base, "instance"))},
	} {
		if strings.Count(raw, change[0]) != 1 {
			t.Fatal("platform-led declaration fixture changed")
		}
		raw = strings.Replace(raw, change[0], change[1], 1)
	}
	return raw
}
