package enrollment

import (
	"strings"
	"testing"
)

func TestDescriptorAcceptsOnlyTheExactBrowserV4Companions(t *testing.T) {
	t.Parallel()
	platform := "linux-amd64"
	raw := strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v4",
		"cohort=cohort-1",
		"release=alpha-1",
		"platform=" + platform,
		"environment=alpha",
		"network=network-1",
		"target_path=ardents/linux-amd64/endpoint",
		"artifact=ardents-linux-amd64",
		"trusted_root=1.root.json",
		"control_catalog=catalog.ac1",
		"disclosure_root=catalog.pub",
		"control_release=release.ac1",
		"control_network=network.ac1",
		"control_compatibility=compatibility.ac1",
		"control_release_root=release.pub",
		"control_network_root=network.pub",
		"control_compatibility_root=compatibility.pub",
		"corpus_authority=corpus.pub",
		"control_artifact=ardents-control-" + platform,
		"browser_adapter_artifact=ardents-browser-" + platform,
		"browser_entry_artifact=ardents-browser-entry-" + platform,
		"browser_entry_extension=ardents-alpha-browser-entry.xpi",
	}, "\n") + "\n"
	descriptor, err := parseDescriptor([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.browserAdapter != "ardents-browser-linux-amd64" || descriptor.browserEntry != "ardents-browser-entry-linux-amd64" || descriptor.browserExtension != "ardents-alpha-browser-entry.xpi" {
		t.Fatalf("Browser companions = %+v", descriptor)
	}
	if _, err := parseDescriptor([]byte(strings.Replace(raw, "enrollment-v4", "enrollment-v3", 1))); err == nil {
		t.Fatal("Application verifier accepted Network enrollment-v3")
	}
}
