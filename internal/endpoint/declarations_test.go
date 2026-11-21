package endpoint

import "testing"

func TestBoundDeclarationsUsePortablePathsAndRequireBothSources(t *testing.T) {
	headless := []byte(`{"schema":"ardents-headless-runtime-v2","role":"reader","application_socket":"/run/ardents/application.sock"}`)
	complete := []byte(`{"schema":"ardents-source-plan-v1","sources":[{"address":"127.0.0.1:9401"},{"address":"127.0.0.1:9402"}]}`)
	plan, sources, err := Declarations(headless, complete)
	if err != nil || plan.ApplicationSocket != "/run/ardents/application.sock" || len(sources.Sources) != 2 {
		t.Fatalf("bound declarations: plan=%+v sources=%+v err=%v", plan, sources, err)
	}
	incomplete := []byte(`{"schema":"ardents-source-plan-v1","sources":[{"address":"127.0.0.1:9401"}]}`)
	if _, _, err := Declarations(headless, incomplete); err == nil {
		t.Fatal("accepted a single Source")
	}
}
