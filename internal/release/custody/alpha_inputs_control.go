package custody

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/release"
)

func buildAlphaInputFiles(ctx context.Context, request alphaInputsRequest, record seedRecord, endpoint []byte) (map[string][]byte, error) {
	return buildAlphaVersionedInputFiles(ctx, request, record, endpoint, nil, 1, [32]byte{})
}

func buildAlphaVersionedInputFiles(ctx context.Context, request alphaInputsRequest, record seedRecord, endpoint, root []byte,
	generation uint64, previousDigest [32]byte) (map[string][]byte, error) {
	if generation == 0 {
		return nil, ErrInvalid
	}
	files, err := buildAlphaTUFVersion(request, record, endpoint, root, generation)
	if err != nil {
		return nil, err
	}
	releaseDecision, err := preflightAlphaRelease(ctx, request, files, endpoint, generation)
	if err != nil {
		return nil, err
	}
	networkSnapshot, err := preflightAlphaNetwork(ctx, request)
	if err != nil {
		return nil, err
	}
	releaseDigest := [32]byte{}
	copy(releaseDigest[:], releaseDecision.Digest)
	releaseBody, err := inspection.EncodeReleaseEvidence(inspection.ReleaseEvidence{
		ArtifactDigest: releaseDigest, TargetPath: releaseDecision.Path, ReleaseIdentity: releaseDecision.ReleaseIdentity,
		BuildIdentity: releaseDecision.BuildIdentity, ProtocolPhase: releaseDecision.ProtocolPhase, BuildState: releaseDecision.BuildState,
	})
	if err != nil {
		return nil, fmt.Errorf("encode alpha release evidence: %w", err)
	}
	networkBody, err := inspection.EncodeNetworkEvidence(inspection.NetworkEvidence{
		NetworkID: request.NetworkState.NetworkID, EpochDigest: request.NetworkState.EpochDigest,
		Profile: request.NetworkState.Profile, Threshold: request.NetworkState.Threshold,
		Authorities: request.NetworkState.Authorities, Epoch: request.NetworkState.Epoch,
		Inputs: request.NetworkState.Inputs, Materials: request.NetworkState.Materials,
	})
	if err != nil {
		return nil, fmt.Errorf("encode alpha network evidence: %w", err)
	}
	compatibilityBody, err := inspection.EncodeCompatibilityEvidence(inspection.CompatibilityEvidence{
		ReleaseDigest: releaseDigest, ReleaseBuildIdentity: releaseDecision.BuildIdentity, ProtocolPhase: releaseDecision.ProtocolPhase,
		NetworkDigest: networkSnapshot.Digest, NetworkEpoch: networkSnapshot.Epoch, NetworkProfile: networkSnapshot.Profile,
	})
	if err != nil {
		return nil, fmt.Errorf("encode alpha compatibility evidence: %w", err)
	}
	bodies := [3][]byte{releaseBody, networkBody, compatibilityBody}
	componentKeys := [3]int{roleAlphaReleaseComponent, roleAlphaNetworkComponent, roleAlphaCompatibilityComponent}
	componentNames := [3]string{"release.ac1", "network.ac1", "compatibility.ac1"}
	rootNames := [3]string{"release.pub", "network.pub", "compatibility.pub"}
	var components [3]alphacontrol.Component
	for index := range bodies {
		signed, signErr := alphacontrol.SignComponent(alphacontrol.ComponentStatement{Class: alphacontrol.ComponentClass(index + 1),
			Generation: generation, NotBefore: request.NotBefore, NotAfter: request.NotAfter, Body: bodies[index]}, alphaRolePrivate(record, componentKeys[index]))
		if signErr != nil {
			return nil, fmt.Errorf("sign alpha component %d: %w", index+1, signErr)
		}
		public := alphaRolePublic(record, componentKeys[index])
		files[componentNames[index]], files[rootNames[index]] = signed, append([]byte(nil), public...)
		components[index] = alphacontrol.Component{Class: alphacontrol.ComponentClass(index + 1), RootID: sha256.Sum256(public),
			Generation: generation, NotAfter: request.NotAfter, Size: uint32(len(signed)), Digest: sha256.Sum256(signed)}
	}
	catalog, err := alphacontrol.Sign(alphacontrol.Catalog{Cohort: request.Cohort, Generation: generation,
		NotBefore: request.NotBefore, NotAfter: request.NotAfter, PreviousDigest: previousDigest, Components: components}, alphaRolePrivate(record, roleAlphaDisclosure))
	if err != nil {
		return nil, fmt.Errorf("sign alpha catalog: %w", err)
	}
	files["catalog.ac1"] = catalog
	files["catalog.pub"] = append([]byte(nil), alphaRolePublic(record, roleAlphaDisclosure)...)
	files["corpus.pub"] = append([]byte(nil), alphaRolePublic(record, roleAlphaCorpusAuthority)...)
	files["RELEASE"] = alphaReleaseDescriptor(request)
	if len(files) != len(alphaStaticFileNames(generation)) {
		return nil, ErrInvalid
	}
	return files, nil
}

func preflightAlphaRelease(ctx context.Context, request alphaInputsRequest, files map[string][]byte, endpoint []byte, generation uint64) (release.Decision, error) {
	if generation == 0 {
		return release.Decision{}, ErrInvalid
	}
	version := fmt.Sprintf("%d", generation)
	root, err := os.MkdirTemp("", "ardents-alpha-release-preflight-")
	if err != nil {
		return release.Decision{}, fmt.Errorf("create release preflight root: %w", err)
	}
	defer os.RemoveAll(root)
	verifier, err := release.Open(root)
	if err != nil {
		return release.Decision{}, fmt.Errorf("open release preflight: %w", err)
	}
	defer verifier.Close()
	metadataFiles := map[string][]byte{
		release.MetadataURL("timestamp.json"):           files["timestamp.json"],
		release.MetadataURL(version + ".snapshot.json"): files[version+".snapshot.json"],
		release.MetadataURL(version + ".targets.json"):  files[version+".targets.json"],
	}
	decision := verifier.Evaluate(ctx, release.Inputs{RootBytes: files["1.root.json"], Files: metadataFiles,
		TargetPath: alphaEndpointTargetPath, Artifact: endpoint,
		Local: release.LocalEnvironment{Environment: request.Environment, Network: request.Network,
			Platform: "linux-amd64", Architecture: "amd64", RefTime: request.ReferenceTime}})
	if decision.Outcome != release.OutcomeReleaseAccepted {
		return decision, fmt.Errorf("%w: release is %s (%s)", ErrPreflight, decision.Outcome, decision.Notice)
	}
	return decision, nil
}

func preflightAlphaNetwork(ctx context.Context, request alphaInputsRequest) (state.Snapshot, error) {
	root, err := os.MkdirTemp("", "ardents-alpha-network-preflight-")
	if err != nil {
		return state.Snapshot{}, fmt.Errorf("create network preflight root: %w", err)
	}
	defer os.RemoveAll(root)
	authorities := make(map[[32]byte]ed25519.PublicKey, len(request.NetworkState.Authorities))
	for _, authority := range request.NetworkState.Authorities {
		authorities[sha256.Sum256(authority)] = append(ed25519.PublicKey(nil), authority...)
	}
	opened, err := state.Open(state.Config{Root: root, NetworkID: request.NetworkState.NetworkID, Authorities: authorities,
		Threshold: int(request.NetworkState.Threshold), AcceptedProfile: request.NetworkState.Profile, Now: request.ReferenceTime})
	if err != nil {
		return state.Snapshot{}, fmt.Errorf("open network preflight: %w", err)
	}
	defer opened.Close()
	snapshot, err := opened.Accept(ctx, request.NetworkState.Epoch, request.NetworkState.Inputs, request.NetworkState.Materials)
	if err != nil {
		return state.Snapshot{}, fmt.Errorf("%w: network state: %v", ErrPreflight, err)
	}
	if snapshot.NetworkID != request.NetworkState.NetworkID || snapshot.Digest != request.NetworkState.EpochDigest || snapshot.Profile != request.NetworkState.Profile {
		return state.Snapshot{}, fmt.Errorf("%w: network identity mismatch", ErrPreflight)
	}
	return snapshot, nil
}

func alphaReleaseDescriptor(request alphaInputsRequest) []byte {
	return []byte(strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v3",
		"cohort=" + request.Cohort,
		"release=" + request.Release,
		"platform=linux-amd64",
		"environment=" + request.Environment,
		"network=" + request.Network,
		"target_path=" + alphaEndpointTargetPath,
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
		"control_artifact=ardents-control-linux-amd64",
	}, "\n") + "\n")
}
