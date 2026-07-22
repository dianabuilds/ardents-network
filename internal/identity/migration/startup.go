package migration

import (
	"fmt"
	"path/filepath"

	"ardents/internal/storage"
)

// CheckStartupState rejects every partially applied PIA migration before the
// daemon opens product stores. PIA-004C will add the only activation marker
// that permits a migrated p1_ epoch to start.
func CheckStartupState(dataDir string) error {
	activated, found, err := loadActivationMarker(filepath.Join(dataDir, applyDirName, activationMarkerName))
	if err != nil {
		return fmt.Errorf("Principal epoch activation state is unreadable; resume or restore before startup")
	}
	if found {
		return checkActivatedStartupState(dataDir, activated)
	}
	if err := checkReissueStartupState(dataDir); err != nil {
		return err
	}
	marker, found, err := loadApplyMarker(filepath.Join(dataDir, applyDirName, applyMarkerName))
	if err != nil {
		return fmt.Errorf("Principal migration state is unreadable; resume or restore before startup")
	}
	if !found || marker.Phase == phaseRestored {
		return nil
	}
	if marker.SchemaVersion != applyMarkerSchema || marker.Node == "" || marker.Legacy == "" || marker.PrincipalV1 == "" {
		return fmt.Errorf("Principal migration state is unreadable; resume or restore before startup")
	}
	return fmt.Errorf("Principal migration phase %s forbids daemon startup; resume or restore the coordinated epoch", marker.Phase)
}

func checkActivatedStartupState(dataDir string, activation activationMarker) error {
	if activation.Phase != activationActivated {
		return fmt.Errorf("Principal epoch is prepared but not activated for startup")
	}
	apply, found, err := loadApplyMarker(filepath.Join(dataDir, applyDirName, applyMarkerName))
	if err != nil || !found || apply.Phase != phaseVerified {
		return fmt.Errorf("Principal epoch activation does not match Node migration state")
	}
	bPhase, err := loadReissueNodePhase(dataDir)
	if err != nil || bPhase != reissuePhaseVerified {
		return fmt.Errorf("Principal epoch activation does not match signed-artifact state")
	}
	var node *activationNode
	for i := range activation.Nodes {
		if activation.Nodes[i].Principal == apply.PrincipalV1 {
			if node != nil {
				return fmt.Errorf("Principal epoch activation contains duplicate Node")
			}
			node = &activation.Nodes[i]
		}
	}
	if node == nil || node.ConfigPath == "" {
		return fmt.Errorf("Principal epoch activation omits this Node")
	}
	checks := map[string]string{filepath.Join(dataDir, "ardents.db"): node.DatabaseSHA256, filepath.Join(dataDir, "capabilities.db"): node.CapabilitiesSHA256, filepath.Join(dataDir, "identity_key.json"): node.RootKeySHA256, node.ConfigPath: node.ConfigSHA256}
	for path, expected := range checks {
		actual, hashErr := fileHash(path)
		if hashErr != nil || expected == "" || actual != expected {
			return fmt.Errorf("Principal epoch activation consistency hash mismatch")
		}
	}
	return nil
}

func checkReissueStartupState(dataDir string) error {
	path := filepath.Join(dataDir, applyDirName, reissueNodeMarkerName)
	raw, found, err := storage.ReadPrivateFile(path)
	if err != nil {
		return fmt.Errorf("signed-artifact migration state is unreadable; resume or restore before startup")
	}
	if !found {
		return nil
	}
	var marker struct {
		SchemaVersion uint32       `json:"schema_version"`
		Phase         reissuePhase `json:"phase"`
	}
	if decodeStrict(raw, &marker) != nil || marker.SchemaVersion != 1 {
		return fmt.Errorf("signed-artifact migration state is unreadable; resume or restore before startup")
	}
	if marker.Phase != reissuePhaseRestored {
		return fmt.Errorf("signed-artifact migration phase %s forbids daemon startup; resume or restore the coordinated epoch", marker.Phase)
	}
	return nil
}
