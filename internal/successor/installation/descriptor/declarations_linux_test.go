package descriptor

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

const independentRequest = `{"schema":"ardents-descriptor-installation-v1","invocation":"descriptor","profile":"ardents-descriptor-process-v1","installation_root":"/opt/ardents-d","state_root":"/var/lib/ardents-state","participation_root":"/var/lib/ardents-participation","spend_root":"/var/lib/ardents-spend","hosting_root":"/var/lib/ardents-hosting","descriptor_root":"/var/lib/ardents-descriptors","credential_directory":"/etc/ardents-d","plan":"/etc/ardents-delivery/descriptor-plan.json","certificate":"/etc/ardents-delivery/certificate.pem","private_key":"/etc/ardents-delivery/private-key.pem","clock":"/var/lib/ardents-clock/observation"}` + "\n"

func TestRequestKeepsStoreAndEveryOriginalNamespaceIndependent(t *testing.T) {
	r, err := DecodeRequest([]byte(independentRequest))
	if err != nil || r.DescriptorRoot != "/var/lib/ardents-descriptors" {
		t.Fatal("independent Descriptor declaration refused", err)
	}
	for _, root := range []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.CredentialDirectory, r.Plan, r.Certificate, r.PrivateKey, r.Clock, r.InstallationRoot + ".descriptor-initial-journal", r.InstallationRoot + ".descriptor-retirement-00000063-journal"} {
		for _, store := range []string{root, filepath.Join(root, "store")} {
			changed := r
			changed.DescriptorRoot = store
			raw, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeRequest(append(raw, '\n')); err == nil {
				t.Fatal("Store root borrowed an original input/root/journal", store)
			}
		}
	}
	for _, path := range []string{r.DescriptorRoot, filepath.Join(r.DescriptorRoot, "output"), filepath.Dir(r.DescriptorRoot), r.InstallationRoot + ".descriptor-hosting", r.InstallationRoot + ".descriptor-replacement-pending"} {
		if r.CheckPreparationOutput(path) == nil {
			t.Fatal("public preparation overlapped retained resources", path)
		}
	}
	if err := r.CheckPreparationOutput("/var/lib/ardents-public-preparation"); err != nil {
		t.Fatal("independent public output refused", err)
	}
}

func TestRequestRefusesForeignGrammarAndOverlongCompletionNamespace(t *testing.T) {
	for _, changed := range []string{
		strings.Replace(independentRequest, `"descriptor"`, `"join"`, 1),
		strings.Replace(independentRequest, "ardents-descriptor-process-v1", "ardents-join-process-v1", 1),
		strings.Replace(independentRequest, "ardents-descriptor-installation-v1", "ardents-join-installation-v1", 1),
		strings.Replace(independentRequest, `"descriptor_root":`, `"descriptor_root":"/foreign","descriptor_root":`, 1),
		strings.TrimSpace(independentRequest), independentRequest + "\n",
	} {
		if _, err := DecodeRequest([]byte(changed)); err == nil {
			t.Fatal("foreign or noncanonical declaration accepted")
		}
	}
	r, err := DecodeRequest([]byte(independentRequest))
	if err != nil {
		t.Fatal(err)
	}
	longest := "/descriptor-invocation-00000063/start-completion.socket"
	r.InstallationRoot = "/opt/" + strings.Repeat("x", 107-len(longest)-len("/opt/"))
	for _, extra := range []string{"", "x"} {
		changed := r
		changed.InstallationRoot += extra
		raw, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		_, err = DecodeRequest(append(raw, '\n'))
		if (err == nil) != (extra == "") {
			t.Fatal("complete original Unix socket namespace bound differs", len(changed.InstallationRoot), err)
		}
	}
}

func TestPlanBindsItsStoreAndPublicTrustWithoutOpeningThem(t *testing.T) {
	r, err := DecodeRequest([]byte(independentRequest))
	if err != nil {
		t.Fatal(err)
	}
	p := Plan{Schema: "ardents-descriptor-runtime-v1", Network: NetworkInputs{Root: r.StateRoot, NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{3}, ClockObservationFile: r.Clock}, NodeID: [32]byte{4}, ProcessProfile: r.Profile,
		ParticipationRoot: r.ParticipationRoot, SpendRoot: r.SpendRoot, HostingRoot: r.HostingRoot, DescriptorRoot: r.DescriptorRoot,
		Certificate: filepath.Join(r.CredentialDirectory, "certificate.pem"), PrivateKey: filepath.Join(r.CredentialDirectory, "private-key.pem"), RetirementWaitMS: 5000}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePlan(append(raw, '\n'))
	if err != nil || r.MatchPlan(decoded) != nil {
		t.Fatal("exact declared inputs refused", err)
	}
	decoded.DescriptorRoot = r.HostingRoot
	if r.MatchPlan(decoded) == nil {
		t.Fatal("plan replaced the independent Store with Hosting")
	}
	for _, invalid := range [][]byte{bytes.Replace(raw, []byte("ardents-descriptor-runtime-v1"), []byte("ardents-join-runtime-v1"), 1), bytes.Replace(raw, []byte(`"retirement_wait_ms":5000`), []byte(`"retirement_wait_ms":5001`), 1), bytes.Replace(raw, []byte(`"threshold":1`), []byte(`"threshold":2`), 1)} {
		if _, err := DecodePlan(append(invalid, '\n')); err == nil {
			t.Fatal("foreign or unavailable original plan accepted")
		}
	}
}
