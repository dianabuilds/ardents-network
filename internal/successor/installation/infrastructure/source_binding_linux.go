package infrastructure

import (
	"bytes"
	"encoding/json"
	"strings"
)

// sourceBinding is immutable selected configuration. Parsing it supplies no
// Release, initial-installation, native-process or startup authority.
type sourceBinding struct {
	Schema           string        `json:"schema"`
	Invocation       string        `json:"invocation"`
	Generation       string        `json:"generation"`
	Predecessor      string        `json:"predecessor"`
	RequestSHA256    string        `json:"request_sha256"`
	ProgramTarget    string        `json:"program_target"`
	GenerationTarget string        `json:"generation_target"`
	ProgramSHA256    string        `json:"program_sha256"`
	TemplateSHA256   string        `json:"template_sha256"`
	InventorySHA256  string        `json:"inventory_sha256"`
	TargetsSHA256    string        `json:"targets_sha256"`
	PlanSHA256       string        `json:"plan_sha256"`
	UnitSHA256       string        `json:"unit_sha256"`
	Account          string        `json:"account"`
	UID              uint32        `json:"uid"`
	GID              uint32        `json:"gid"`
	Request          SourceRequest `json:"request"`
}

// The first-start reader intentionally accepts only initial bindings. A future
// replacement cannot enter this path merely by supplying a predecessor string.
func decodeSourceBinding(raw []byte) (sourceBinding, error) {
	var b sourceBinding
	if len(raw) == 0 || len(raw) > 32<<10 || json.Unmarshal(raw, &b) != nil {
		return sourceBinding{}, ErrInput
	}
	canonical, err := json.Marshal(b)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return sourceBinding{}, ErrInput
	}
	if b.Schema != "ardents-source-binding-v1" || b.Invocation != "source" || b.Predecessor != "" ||
		b.ProgramTarget != ProgramTarget || b.GenerationTarget != GenerationTarget || b.Account != "ardents-source" ||
		b.UID < 100 || b.UID > 999 || b.GID != b.UID {
		return sourceBinding{}, ErrBinding
	}
	for _, d := range []string{b.Generation, b.RequestSHA256, b.ProgramSHA256, b.TemplateSHA256, b.InventorySHA256, b.TargetsSHA256, b.PlanSHA256, b.UnitSHA256} {
		if len(d) != 64 || strings.Trim(d, "0123456789abcdef") != "" {
			return sourceBinding{}, ErrBinding
		}
	}
	request, err := json.Marshal(b.Request)
	if err != nil {
		return sourceBinding{}, ErrInput
	}
	request = append(request, '\n')
	if _, err = DecodeSourceRequest(request); err != nil || digest(request) != b.RequestSHA256 {
		return sourceBinding{}, ErrBinding
	}
	unit, err := RenderSourceUnit([]byte(SourceTemplate), b.Request, b.Generation)
	if err != nil || b.TemplateSHA256 != digest([]byte(SourceTemplate)) || b.UnitSHA256 != digest(unit) {
		return sourceBinding{}, ErrBinding
	}
	return b, nil
}
