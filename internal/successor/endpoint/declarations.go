package endpoint

import (
	"encoding/json"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/runtimeplan"
)

// These projections consume independently bound declarations; they confer no
// runtime authority. Runtime behavior remains with its original owners.
type PermissionPlan = runtimeplan.Permission
type HeadlessPlan runtimeplan.Headless
type SourcePlan runtimeplan.Source

func Declarations(headlessRaw, sourceRaw []byte) (HeadlessPlan, SourcePlan, error) {
	var headless HeadlessPlan
	var source SourcePlan
	if len(headlessRaw) > 16<<10 || len(sourceRaw) > 32<<10 || json.Unmarshal(headlessRaw, &headless) != nil || json.Unmarshal(sourceRaw, &source) != nil || headless.Schema != "ardents-headless-runtime-v2" || source.Schema != "ardents-source-plan-v1" || len(source.Sources) != 2 {
		return headless, source, errors.New("installed bound declarations are unavailable")
	}
	return headless, source, nil
}
