package introduction

import (
	"bytes"
	"encoding/json"
	"path"
	"strings"
	"testing"
)

const independentRequest = `{"schema":"ardents-introduction-installation-v1","invocation":"introduction","profile":"ardents-introduction-process-v1","installation_root":"/opt/ardents-d","state_root":"/var/lib/ardents-state","participation_root":"/var/lib/ardents-participation","spend_root":"/var/lib/ardents-spend","hosting_root":"/var/lib/ardents-hosting","introduction_root":"/var/lib/ardents-introductions","credential_directory":"/etc/ardents-d","plan":"/etc/ardents-delivery/introduction-plan.json","certificate":"/etc/ardents-delivery/certificate.pem","private_key":"/etc/ardents-delivery/private-key.pem","clock":"/var/lib/ardents-clock/observation"}` + "\n"

func TestRequestKeepsSlotHistoryAndEveryOriginalNamespaceIndependent(t *testing.T) {
	r, err := DecodeRequest([]byte(independentRequest))
	if err != nil || r.IntroductionRoot != "/var/lib/ardents-introductions" {
		t.Fatal("independent Introduction declaration refused", err)
	}
	for _, root := range []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.CredentialDirectory, r.Plan, r.Certificate, r.PrivateKey, r.Clock, r.InstallationRoot + ".introduction-initial-journal", r.InstallationRoot + ".introduction-retirement-00000063-journal"} {
		for _, slots := range []string{root, path.Join(root, "slots")} {
			changed := r
			changed.IntroductionRoot = slots
			raw, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeRequest(append(raw, '\n')); err == nil {
				t.Fatal("slot history root borrowed an original input/root/journal", slots)
			}
		}
	}
	for _, path := range []string{r.IntroductionRoot, path.Join(r.IntroductionRoot, "output"), path.Dir(r.IntroductionRoot), r.InstallationRoot + ".introduction-hosting", r.InstallationRoot + ".introduction-replacement-pending"} {
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
		strings.Replace(independentRequest, `"introduction"`, `"join"`, 1),
		strings.Replace(independentRequest, "ardents-introduction-process-v1", "ardents-join-process-v1", 1),
		strings.Replace(independentRequest, "ardents-introduction-installation-v1", "ardents-join-installation-v1", 1),
		strings.Replace(independentRequest, `"introduction_root":`, `"introduction_root":"/foreign","introduction_root":`, 1),
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
	longest := "/introduction-invocation-00000063/start-completion.socket"
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

func TestPlanBindsItsSlotHistoryAndPublicTrustWithoutOpeningThem(t *testing.T) {
	r, err := DecodeRequest([]byte(independentRequest))
	if err != nil {
		t.Fatal(err)
	}
	p := Plan{Schema: "ardents-introduction-runtime-v1", Network: NetworkInputs{Root: r.StateRoot, NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{3}, ClockObservationFile: r.Clock}, NodeID: [32]byte{4}, ProcessProfile: r.Profile,
		ParticipationRoot: r.ParticipationRoot, SpendRoot: r.SpendRoot, HostingRoot: r.HostingRoot, IntroductionRoot: r.IntroductionRoot,
		Certificate: path.Join(r.CredentialDirectory, "certificate.pem"), PrivateKey: path.Join(r.CredentialDirectory, "private-key.pem"), RetirementWaitMS: 5000}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePlan(append(raw, '\n'))
	if err != nil || r.MatchPlan(decoded) != nil {
		t.Fatal("exact declared inputs refused", err)
	}
	decoded.IntroductionRoot = r.HostingRoot
	if r.MatchPlan(decoded) == nil {
		t.Fatal("plan replaced the independent slot history with Hosting")
	}
	for _, invalid := range [][]byte{bytes.Replace(raw, []byte("ardents-introduction-runtime-v1"), []byte("ardents-join-runtime-v1"), 1), bytes.Replace(raw, []byte(`"retirement_wait_ms":5000`), []byte(`"retirement_wait_ms":5001`), 1), bytes.Replace(raw, []byte(`"threshold":1`), []byte(`"threshold":2`), 1)} {
		if _, err := DecodePlan(append(invalid, '\n')); err == nil {
			t.Fatal("foreign or unavailable original plan accepted")
		}
	}
}
