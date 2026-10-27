package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

func selectedGenerationVector(t *testing.T) (binding, []byte, map[string][]byte) {
	t.Helper()
	b, err := decodeBinding(forwardingBindingVector())
	if err != nil {
		t.Fatal(err)
	}
	p := Plan{Schema: "ardents-forwarding-runtime-v1", ProcessProfile: "ardents-forwarding-process-v1", NodeID: [32]byte{4},
		Network: NetworkInputs{Root: b.Request.StateRoot, ClockObservationFile: b.Request.Clock, NetworkID: [32]byte{1},
			Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{3}},
		ParticipationRoot: b.Request.ParticipationRoot, SpendRoot: b.Request.SpendRoot, HostingRoot: b.Request.HostingRoot,
		Certificate: filepath.Join(b.Request.CredentialDirectory, "certificate.pem"), PrivateKey: filepath.Join(b.Request.CredentialDirectory, "private-key.pem"), RetirementWaitMS: 1000}
	p.Work.Tx, p.Work.Rx, p.Termination.Tx, p.Termination.Rx = 40000000, 40000000, 1024, 1024
	encode := func(value any) []byte {
		t.Helper()
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return append(body, '\n')
	}
	// Pure codec fixture only: these bytes supply no executable, signature,
	// Enrollment provenance, Release decision or accepting installed observation.
	files := map[string][]byte{
		programName: []byte("pure Node byte fixture"), "ardents-forwarding.service": []byte(Template),
		"root.json": []byte("root fixture"), "timestamp.json": []byte("timestamp fixture"),
		"snapshot.json": []byte("snapshot fixture"), "targets.json": []byte("targets fixture"),
		"forwarding-request.json": []byte(forwardingRequestVector), "forwarding-plan.json": encode(p),
	}
	files["infrastructure-forwarding.json"] = fmt.Appendf(nil, "{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":1,\"invocation\":\"forwarding\",\"files\":{\"ardents-forwarding.service\":\"%s\",\"ardents-node-linux-amd64\":\"%s\"}}\n", digest(files["ardents-forwarding.service"]), digest(files[programName]))
	b.Generation = digest(files["infrastructure-forwarding.json"])
	public := map[string]string{}
	for _, name := range []string{programName, "ardents-forwarding.service", "infrastructure-forwarding.json", "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		public[name] = digest(files[name])
	}
	hashes, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	files["infrastructure-enrollment.json"] = fmt.Appendf(nil, "{\"schema\":\"ardents-infrastructure-enrollment-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":1,\"invocation\":\"forwarding\",\"files\":%s}\n", hashes)
	files["rendered-forwarding.service"], err = RenderUnit([]byte(Template), b.Request, b.Generation)
	if err != nil {
		t.Fatal(err)
	}
	b.ProgramSHA256, b.PlanSHA256, b.InventorySHA256, b.TargetsSHA256 = digest(files[programName]), digest(files["forwarding-plan.json"]), digest(files["infrastructure-enrollment.json"]), digest(files["targets.json"])
	b.UnitSHA256 = digest(files["rendered-forwarding.service"])
	selected := encode(b)
	files["binding.json"] = bytes.Clone(selected)
	return b, selected, files
}

func TestSelectedForwardingGenerationRequiresEveryOriginalCommittedByte(t *testing.T) {
	b, selected, files := selectedGenerationVector(t)
	if _, err := decodeBinding(selected); err != nil {
		t.Fatal("independent binding vector", err)
	}
	if _, err := validateSelectedGeneration(b, selected, files); err != nil {
		t.Fatal("independent twelve-file byte vector", err)
	}
	for _, name := range forwardingGenerationNames() {
		t.Run(name, func(t *testing.T) {
			original := files[name]
			defer func() { files[name] = original }()
			delete(files, name)
			if _, err := validateSelectedGeneration(b, selected, files); !errors.Is(err, ErrBinding) {
				t.Fatal("missing original file accepted", err)
			}
			files[name] = append(bytes.Clone(original), '\n')
			if _, err := validateSelectedGeneration(b, selected, files); !errors.Is(err, ErrBinding) {
				t.Fatal("substituted original byte accepted", err)
			}
		})
	}
	files["unexpected"] = []byte("extra")
	if _, err := validateSelectedGeneration(b, selected, files); !errors.Is(err, ErrBinding) {
		t.Fatal("open inventory accepted", err)
	}
}
