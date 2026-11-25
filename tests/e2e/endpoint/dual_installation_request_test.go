package endpoint

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDualInstallationRequestBindsBothRoles(t *testing.T) {
	base := filepath.Join(t.TempDir(), "participant")
	input := InstallationRequest(t, filepath.Join(base, "bundle"), strings.Repeat("ab", 32), filepath.Join(base, "floors"))
	var report struct {
		Headless struct {
			Role   json.RawMessage `json:"role"`
			Reader struct {
				Maxima [3]int `json:"maxima"`
			} `json:"reader_permission"`
			Publisher struct {
				Request  string `json:"request_path"`
				Response string `json:"response_path"`
				Maxima   [3]int `json:"maxima"`
			} `json:"publisher_permission"`
			AdministrationSocket string `json:"administration_socket"`
			InstanceRoot         string `json:"service_instance_root"`
		} `json:"headless"`
	}
	if err := json.Unmarshal([]byte(DualInstallationRequest(t, string(input), base)), &report); err != nil {
		t.Fatal(err)
	}
	got := report.Headless
	if got.Role != nil || got.Reader.Maxima != [3]int{32, 32, 0} || got.Publisher.Maxima != [3]int{32, 32, 8} ||
		got.Publisher.Request != filepath.Join(base, "publisher-permission", "request") ||
		got.Publisher.Response != filepath.Join(base, "publisher-permission", "response") ||
		got.AdministrationSocket != filepath.Join(base, "administration", "socket") ||
		got.InstanceRoot != filepath.Join(base, "instance") {
		t.Fatalf("unexpected dual-role fixture: %+v", got)
	}
}
