//go:build live

package network_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

type finalRunnerSupplyLock struct {
	Schema           string `json:"schema"`
	GoBuilderImageID string `json:"go_builder_image_id"`
	GoBuilderVersion string `json:"go_builder_version"`
	GoArchiveSHA256  string `json:"go_archive_sha256"`
	GoRecipeSHA256   string `json:"go_builder_recipe_sha256"`
	GoModuleSHA256   string `json:"go_module_cache_sha256"`
	ToolImageID      string `json:"tool_image_id"`
	ToolLockSHA256   string `json:"tool_lock_sha256"`
	CarrierSHA256    string `json:"carrier_sha256"`
}

func verifyFinalRunnerSupplyLock(root string, value finalRunnerSchedule) error {
	raw, err := finalSupplyOutput("git", "-C", root, "show",
		value.RepositoryCommit+":tests/live/stage5-final/supply.lock.json")
	if err != nil || len(raw) == 0 || len(raw) > 4<<10 {
		return errors.Join(err, errors.New("accepted final supply lock is unavailable"))
	}
	recipe, err := finalSupplyOutput("git", "-C", root, "show",
		value.RepositoryCommit+":tests/live/stage5-final/go-builder.Dockerfile")
	digest := sha256.Sum256(recipe)
	if err != nil || hex.EncodeToString(digest[:]) != value.ProductReceipt.GoRecipeSHA256 {
		return errors.Join(err, errors.New("accepted Go builder recipe differs from its product receipt"))
	}
	toolLock, err := finalSupplyOutput("git", "-C", root, "show",
		value.RepositoryCommit+":lab/carrier/tools.lock")
	toolDigest := sha256.Sum256(toolLock)
	if err != nil || hex.EncodeToString(toolDigest[:]) != value.ToolReceipt.ToolLockSHA256 {
		return errors.Join(err, errors.New("accepted Carrier tool lock differs from its product receipt"))
	}
	return validateFinalRunnerSupplyLock(raw, value)
}

func validateFinalRunnerSupplyLock(raw []byte, value finalRunnerSchedule) error {
	var lock finalRunnerSupplyLock
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&lock); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("accepted final supply lock is malformed")
	}
	canonical, err := json.MarshalIndent(lock, "", "  ")
	digest := sha256.Sum256(raw)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) ||
		hex.EncodeToString(digest[:]) != value.SupplyLock.SHA256 || int64(len(raw)) != value.SupplyLock.Bytes {
		return errors.New("accepted final supply lock differs from its commitment")
	}
	if lock.Schema != "ardents-h3-s5-supply-lock-v1" ||
		lock.GoBuilderImageID != value.GoBuilderImageID || lock.GoBuilderVersion != value.GoBuilderVersion ||
		lock.GoArchiveSHA256 != "708effb774be8237570d0add163225abbdfaf4fca28b2611df167beba4feef89" ||
		lock.GoArchiveSHA256 != value.ProductReceipt.GoArchiveSHA256 ||
		lock.GoRecipeSHA256 != value.ProductReceipt.GoRecipeSHA256 ||
		lock.GoModuleSHA256 != value.ProductReceipt.GoModuleSHA256 ||
		lock.ToolImageID != value.ToolImageID || lock.ToolLockSHA256 != value.ToolReceipt.ToolLockSHA256 ||
		lock.CarrierSHA256 != value.ToolReceipt.CarrierSHA256 {
		return errors.New("final supply differs from the accepted repository lock")
	}
	return nil
}
