package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHeadlessReaderPlanRefusesMixedAuthorityBeforeEffects(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*headlessRuntimePlan)
	}{
		{"unknown role", func(p *headlessRuntimePlan) { p.Role = "publisher" }},
		{"Instance", func(p *headlessRuntimePlan) {
			p.ServiceInstanceRoot = filepath.Join(filepath.Dir(p.NetworkStateRoot), "instance")
		}},
		{"Publication", func(p *headlessRuntimePlan) {
			p.PublicationRoot = filepath.Join(filepath.Dir(p.NetworkStateRoot), "publication")
		}},
		{"Administration socket", func(p *headlessRuntimePlan) {
			p.AdministrationSocket = filepath.Join(filepath.Dir(p.NetworkStateRoot), "administration.sock")
		}},
		{"Administration principal", func(p *headlessRuntimePlan) { p.AdministrationPrincipal = p.ConnectionPrincipal }},
		{"Publisher Permission", func(p *headlessRuntimePlan) { p.PublisherPermission.Maxima = [3]uint32{1} }},
		{"missing Reader Permission", func(p *headlessRuntimePlan) { p.ReaderPermission = headlessPermissionPlan{} }},
		{"missing State", func(p *headlessRuntimePlan) { p.NetworkStateRoot = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := headlessTextPlanFixture(t)
			plan.Role = "reader"
			plan.ServiceInstanceRoot, plan.PublicationRoot = "", ""
			plan.AdministrationSocket, plan.AdministrationPrincipal = "", ""
			plan.PublisherPermission = headlessPermissionPlan{}
			test.change(&plan)
			var output bytes.Buffer
			if err := runHeadlessRuntime(t.Context(), writeHeadlessTextPlan(t, plan), &output); err == nil {
				t.Fatal("mixed or incomplete Reader plan accepted")
			}
			if output.Len() != 0 {
				t.Fatal("rejected plan emitted runtime output")
			}
			assertHeadlessPlanPathsAbsent(t, plan)
		})
	}
}

func TestHeadlessReaderPlanDoesNotRequirePublisherInputs(t *testing.T) {
	plan := headlessTextPlanFixture(t)
	plan.ServiceInstanceRoot, plan.PublicationRoot = "", ""
	plan.AdministrationSocket, plan.AdministrationPrincipal = "", ""
	plan.PublisherPermission = headlessPermissionPlan{}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	fields["role"] = json.RawMessage(`"reader"`)
	raw, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "reader.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	decoded, err := loadHeadlessRuntimePlan(path)
	if err != nil {
		t.Fatalf("Connection-only plan refused: %v", err)
	}
	if decoded.AdministrationPrincipal != [32]byte{} || decoded.ServiceInstanceRoot != "" || decoded.PublicationRoot != "" {
		t.Fatal("Reader acquired Publisher inputs")
	}
}
