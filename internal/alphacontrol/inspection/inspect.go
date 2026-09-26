package inspection

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/release"
)

func inspect(ctx context.Context, config Config) (Report, error) {
	if ctx == nil || config.Root == "" || config.At.IsZero() {
		return Report{}, errors.New("alpha control inspection configuration is incomplete")
	}
	catalogRoot, releaseRoot, networkRoot, err := prepareInspectionRoot(config.Root)
	if err != nil {
		return Report{}, err
	}
	request := config.Enrollment
	request.ReferenceTime = config.At.UTC()
	verified, err := enrollment.Verify(request)
	if err != nil {
		return Report{}, fmt.Errorf("verify alpha enrollment: %w", err)
	}
	disclosure, roots, err := componentRoots(verified)
	if err != nil {
		return Report{}, err
	}
	reader, err := alphacontrol.OpenReader(alphacontrol.ReaderConfig{Root: catalogRoot, DisclosureKey: disclosure,
		ComponentKeys: roots, Clock: func() time.Time { return config.At.UTC() }})
	if err != nil {
		return Report{}, fmt.Errorf("open alpha control reader: %w", err)
	}
	report := Report{}
	if catalog, _, catalogErr := alphacontrol.Verify(verified.ControlCatalog, disclosure, config.At.UTC()); catalogErr == nil {
		report.CatalogCohort, report.CatalogGeneration = catalog.Cohort, catalog.Generation
		report.CatalogNotBefore, report.CatalogNotAfter = catalog.NotBefore, catalog.NotAfter
		for index, component := range catalog.Components {
			report.ComponentDetails[index] = ComponentDetails{RootID: component.RootID, Generation: component.Generation,
				Digest: component.Digest, NotAfter: component.NotAfter}
		}
	}
	var releaseDecision release.Decision
	var networkSnapshot state.Snapshot
	var releaseAccepted, networkAccepted bool
	var componentErr error
	componentVerify := func(component alphacontrol.Component, statement alphacontrol.ComponentStatement, at time.Time) alphacontrol.Outcome {
		index := int(component.Class) - int(alphacontrol.ComponentRelease)
		if index >= 0 && index < len(report.ComponentDetails) {
			report.ComponentDetails[index] = ComponentDetails{RootID: component.RootID, Generation: component.Generation,
				Digest: component.Digest, NotBefore: statement.NotBefore, NotAfter: statement.NotAfter}
		}
		switch component.Class {
		case alphacontrol.ComponentRelease:
			outcome, err := verifyRelease(ctx, releaseRoot, verified.Inputs, statement.Body, &releaseDecision, &releaseAccepted)
			componentErr = errors.Join(componentErr, err)
			return outcome
		case alphacontrol.ComponentNetwork:
			outcome, err := verifyNetwork(ctx, networkRoot, statement.Body, at, &networkSnapshot, &networkAccepted)
			componentErr = errors.Join(componentErr, err)
			return outcome
		case alphacontrol.ComponentCompatibility:
			return verifyCompatibility(statement.Body, releaseDecision, releaseAccepted, networkSnapshot, networkAccepted)
		default:
			return alphacontrol.OutcomeInvalid
		}
	}
	result, err := reader.Inspect(verified.ControlCatalog, [3][]byte{verified.ControlRelease, verified.ControlNetwork, verified.ControlCompatibility}, componentVerify)
	readerCloseErr := reader.Close()
	if readerCloseErr != nil {
		readerCloseErr = fmt.Errorf("close alpha control catalog reader: %w", readerCloseErr)
	}
	report.Inspection = result
	report.Release = string(releaseDecision.Outcome)
	report.ReleaseIdentity, report.BuildIdentity, report.ProtocolPhase = releaseDecision.ReleaseIdentity, releaseDecision.BuildIdentity, releaseDecision.ProtocolPhase
	report.BuildSafetyNoNewWorkAfter = releaseDecision.BuildSafetyNoNewWorkAfter
	report.BuildSafetyTerminateAfter = releaseDecision.BuildSafetyTerminateAfter
	_, report.ReleaseAuthorizationPresent = releaseDecision.Authorization()
	if len(releaseDecision.Digest) == len(report.ArtifactDigest) {
		copy(report.ArtifactDigest[:], releaseDecision.Digest)
	}
	if networkAccepted {
		report.NetworkID, report.NetworkEpoch, report.NetworkDigest = networkSnapshot.NetworkID, networkSnapshot.Epoch, networkSnapshot.Digest
		report.NetworkProfile = networkSnapshot.Profile
		report.NetworkValidUntil = networkSnapshot.ValidUntil
	}
	if err != nil {
		err = fmt.Errorf("inspect alpha control catalog: %w", err)
	}
	return report, errors.Join(err, componentErr, readerCloseErr)
}

func componentRoots(verified enrollment.Verified) (ed25519.PublicKey, [3]ed25519.PublicKey, error) {
	if len(verified.DisclosureRoot) != ed25519.PublicKeySize {
		return nil, [3]ed25519.PublicKey{}, errors.New("alpha disclosure root is invalid")
	}
	values := [3][]byte{verified.ControlReleaseRoot, verified.ControlNetworkRoot, verified.ControlCompatibilityRoot}
	var roots [3]ed25519.PublicKey
	for index, value := range values {
		if len(value) != ed25519.PublicKeySize {
			return nil, [3]ed25519.PublicKey{}, errors.New("alpha component root is invalid")
		}
		roots[index] = append(ed25519.PublicKey(nil), value...)
	}
	return append(ed25519.PublicKey(nil), verified.DisclosureRoot...), roots, nil
}

func verifyRelease(ctx context.Context, root string, inputs release.Inputs, raw []byte, decision *release.Decision, accepted *bool) (outcome alphacontrol.Outcome, closeErr error) {
	evidence, err := decodeReleaseEvidence(raw)
	if err != nil {
		return alphacontrol.OutcomeInvalid, nil
	}
	verifier, err := release.Open(root)
	if err != nil {
		return alphacontrol.OutcomeInvalid, nil
	}
	defer func() {
		if err := verifier.Close(); err != nil {
			outcome = alphacontrol.OutcomeUnavailable
			*accepted = false
			closeErr = fmt.Errorf("close alpha control release verifier: %w", err)
		}
	}()
	*decision = verifier.Evaluate(ctx, inputs)
	*accepted = decision.Outcome == release.OutcomeReleaseAccepted || decision.Outcome == release.OutcomeNoUpdate
	if !*accepted {
		return releaseOutcome(decision.Outcome), nil
	}
	if len(decision.Digest) != 32 || evidence.TargetPath != decision.Path || evidence.ReleaseIdentity != decision.ReleaseIdentity ||
		evidence.BuildIdentity != decision.BuildIdentity || evidence.ProtocolPhase != decision.ProtocolPhase || evidence.BuildState != decision.BuildState {
		return alphacontrol.OutcomeInvalid, nil
	}
	var digest [32]byte
	copy(digest[:], decision.Digest)
	if evidence.ArtifactDigest != digest {
		return alphacontrol.OutcomeInvalid, nil
	}
	return alphacontrol.OutcomeAccepted, nil
}

func releaseOutcome(outcome release.Outcome) alphacontrol.Outcome {
	switch outcome {
	case release.OutcomeReleaseExpired, release.OutcomeUpdateRequired:
		return alphacontrol.OutcomeExpired
	case release.OutcomeReleaseConflict:
		return alphacontrol.OutcomeConflict
	case release.OutcomeReleaseUnavailable:
		return alphacontrol.OutcomeUnavailable
	default:
		return alphacontrol.OutcomeInvalid
	}
}

func verifyNetwork(ctx context.Context, root string, raw []byte, at time.Time, snapshot *state.Snapshot, accepted *bool) (outcome alphacontrol.Outcome, closeErr error) {
	evidence, err := decodeNetworkEvidence(raw)
	if err != nil {
		return alphacontrol.OutcomeInvalid, nil
	}
	authorities := make(map[[32]byte]ed25519.PublicKey, len(evidence.Authorities))
	for _, authority := range evidence.Authorities {
		authorities[sha256Digest(authority)] = authority
	}
	opened, err := state.Open(state.Config{Root: root, NetworkID: evidence.NetworkID, Authorities: authorities,
		Threshold: int(evidence.Threshold), AcceptedProfile: evidence.Profile, Now: at})
	if err != nil {
		return alphacontrol.OutcomeInvalid, nil
	}
	defer func() {
		if err := opened.Close(); err != nil {
			outcome = alphacontrol.OutcomeUnavailable
			*accepted = false
			closeErr = fmt.Errorf("close alpha control network state: %w", err)
		}
	}()
	if current, currentErr := opened.Current(); currentErr == nil {
		if current.NetworkID == evidence.NetworkID && current.Digest == evidence.EpochDigest && current.Profile == evidence.Profile {
			*snapshot, *accepted = current, true
			return alphacontrol.OutcomeAccepted, nil
		}
		return alphacontrol.OutcomeConflict, nil
	}
	value, err := opened.Accept(ctx, evidence.Epoch, evidence.Inputs, evidence.Materials)
	if err != nil {
		return alphacontrol.OutcomeInvalid, nil
	}
	if value.Digest != evidence.EpochDigest {
		return alphacontrol.OutcomeInvalid, nil
	}
	*snapshot, *accepted = value, true
	return alphacontrol.OutcomeAccepted, nil
}

func verifyCompatibility(raw []byte, releaseDecision release.Decision, releaseAccepted bool, network state.Snapshot, networkAccepted bool) alphacontrol.Outcome {
	if !releaseAccepted || !networkAccepted {
		return alphacontrol.OutcomeUnavailable
	}
	evidence, err := decodeCompatibilityEvidence(raw)
	if err != nil || len(releaseDecision.Digest) != 32 {
		return alphacontrol.OutcomeInvalid
	}
	var releaseDigest [32]byte
	copy(releaseDigest[:], releaseDecision.Digest)
	if evidence.ReleaseDigest != releaseDigest || evidence.ReleaseBuildIdentity != releaseDecision.BuildIdentity ||
		evidence.ProtocolPhase != releaseDecision.ProtocolPhase || evidence.NetworkDigest != network.Digest ||
		evidence.NetworkEpoch != network.Epoch || evidence.NetworkProfile != network.Profile {
		return alphacontrol.OutcomeInvalid
	}
	return alphacontrol.OutcomeAccepted
}
