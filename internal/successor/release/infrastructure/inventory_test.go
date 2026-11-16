package infrastructure

import (
	"bytes"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
)

func TestRoleInventoryChecksExactCommittedBytes(t *testing.T) {
	for _, test := range []struct {
		role, template, artifact string
		check                    func(map[string][]byte, InventoryCommitments) (enrollment.InfrastructureFacts, map[string]string, bool, error)
	}{
		{"join", JoinTemplateName, JoinDescriptorName, CheckJoinInventory},
		{"descriptor", DescriptorTemplateName, DescriptorArtifactName, CheckDescriptorInventory},
		{"introduction", IntroductionTemplateName, IntroductionArtifactName, CheckIntroductionInventory},
		{"forwarding", ForwardingTemplateName, ForwardingDescriptorName, CheckForwardingInventory},
	} {
		t.Run(test.role, func(t *testing.T) {
			files := map[string][]byte{
				ProgramName: []byte("program"), test.template: []byte("template"), test.artifact: []byte("artifact"),
				"root.json": []byte("root"), "timestamp.json": []byte("timestamp"), "snapshot.json": []byte("snapshot"), "targets.json": []byte("targets"),
				test.role + "-request.json": []byte("request"), test.role + "-plan.json": []byte("plan"), "rendered-" + test.role + ".service": []byte("unit"),
			}
			public := map[string][]byte{}
			for _, name := range []string{ProgramName, test.template, test.artifact, "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
				public[name] = files[name]
			}
			facts := enrollment.InfrastructureFacts{Platform: "linux-amd64", ReleaseIdentity: "release", ReleaseVersion: 1, Invocation: test.role}
			var err error
			files[enrollment.InfrastructureInventoryName], err = enrollment.PrepareInfrastructureInventory(facts, public)
			if err != nil {
				t.Fatal(err)
			}
			want := InventoryCommitments{digest(files[ProgramName]), digest(files[test.template]), digest(files[test.artifact]), digest(files[enrollment.InfrastructureInventoryName]), digest(files["targets.json"])}
			got, hashes, ok, err := test.check(files, want)
			if err != nil || !ok || got != facts || hashes[ProgramName] != want.Program {
				t.Fatal("original inventory refused", got, ok, err)
			}
			declaration := Declaration{Invocation: test.role, Files: map[string]string{ProgramName: want.Program, test.template: want.Template}}
			if !MatchesInventoryResources(declaration, hashes) {
				t.Fatal("original declaration did not match inventory")
			}
			wrong := map[string]string{ProgramName: want.Program, test.template: "foreign"}
			if MatchesInventoryResources(declaration, wrong) {
				t.Fatal("foreign template digest accepted")
			}
			declaration.Invocation = "unknown"
			if MatchesInventoryResources(declaration, hashes) {
				t.Fatal("unknown role accepted")
			}
			for _, name := range []string{ProgramName, test.template, test.artifact, "root.json", "timestamp.json", "snapshot.json", "targets.json", enrollment.InfrastructureInventoryName} {
				original := files[name]
				files[name] = append(bytes.Clone(original), '!')
				_, _, ok, err := test.check(files, want)
				if err == nil && ok {
					t.Fatalf("changed %s accepted", name)
				}
				files[name] = original
			}
			for _, change := range []func(*InventoryCommitments){
				func(c *InventoryCommitments) { c.Program = "other" },
				func(c *InventoryCommitments) { c.Template = "other" },
				func(c *InventoryCommitments) { c.Artifact = "other" },
				func(c *InventoryCommitments) { c.Inventory = "other" },
				func(c *InventoryCommitments) { c.Targets = "other" },
			} {
				wrong := want
				change(&wrong)
				if _, _, ok, err := test.check(files, wrong); err != nil || ok {
					t.Fatal("foreign commitment accepted", ok, err)
				}
			}
			if test.role != "join" {
				if _, _, ok, err := CheckJoinInventory(files, want); err == nil && ok {
					t.Fatal("foreign role accepted")
				}
			}
		})
	}
}
