package installation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndpointUnitRejectsWeakenedTemplate(t *testing.T) {
	template, err := os.ReadFile("../../../packaging/text-worker/ardents-endpoint.service")
	if err != nil {
		t.Fatal(err)
	}
	request := installationRequestFixture(t)
	for name, mutation := range map[string]func(string) string{
		"missing socket dependency": func(s string) string {
			return strings.Replace(s, "Requires=ardents-text-reader.socket ardents-text-publisher.socket\n", "", 1)
		},
		"root account":          func(s string) string { return strings.Replace(s, "User=ardents-endpoint", "User=root", 1) },
		"privileged executable": func(s string) string { return strings.Replace(s, "ExecStart=:", "ExecStart=+:", 1) },
		"headless fallback":     func(s string) string { return strings.Replace(s, "start-installed", "start-headless", 1) },
		"missing confinement":   func(s string) string { return strings.Replace(s, "NoNewPrivileges=yes\n", "", 1) },
		"extra command":         func(s string) string { return s + "ExecStartPost=/bin/true\n" },
		"duplicate directive":   func(s string) string { return s + "User=ardents-endpoint\n" },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := renderEndpointUnit([]byte(mutation(string(template))), request, request.InstallationRoot); err == nil {
				t.Fatal("weakened template accepted")
			}
		})
	}
}

func TestEndpointUnitQuotesPathsWithoutEnvironmentOrSpecifierExpansion(t *testing.T) {
	template, err := os.ReadFile("../../../packaging/text-worker/ardents-endpoint.service")
	if err != nil {
		t.Fatal(err)
	}
	request := installationRequestFixture(t)
	request.InstallationRoot = filepath.Join(t.TempDir(), `space $NAME %n "quoted"`)
	directory := filepath.Join(request.InstallationRoot, "generations", "digest")
	body, err := renderEndpointUnit(template, request, directory)
	if err != nil {
		t.Fatal(err)
	}
	expected := "ExecStart=:" + quoteUnitPath(filepath.Join(directory, "ardents-linux-amd64")) + " endpoint start-installed " + quoteUnitPath(request.InstallationRoot) + "\n"
	if !strings.Contains(string(body), expected) || !strings.Contains(string(body), "%%n") || strings.Contains(string(body), "@ARDENTS_") {
		t.Fatalf("unexpected rendered command: %s", body)
	}
}

func TestEndpointUnitRefusesControlCharactersInExecutablePath(t *testing.T) {
	template, err := os.ReadFile("../../../packaging/text-worker/ardents-endpoint.service")
	if err != nil {
		t.Fatal(err)
	}
	request := installationRequestFixture(t)
	for _, control := range []string{"\t", "\x01", "\u0085"} {
		if _, err := renderEndpointUnit(template, request, filepath.Join(request.InstallationRoot, "bad"+control+"path")); err == nil {
			t.Fatal("control character accepted in executable path")
		}
	}
}
