package forwarding

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
)

const programName = infrastructure.ProgramName

// binding fixes the original initial generation and shared host Node resources.
// Parsing copied bytes grants no Release, account, writer or startup authority.
type binding struct {
	Schema            string  `json:"schema"`
	Invocation        string  `json:"invocation"`
	Generation        string  `json:"generation"`
	ProgramTarget     string  `json:"program_target"`
	GenerationTarget  string  `json:"generation_target"`
	ProgramSHA256     string  `json:"program_sha256"`
	TemplateSHA256    string  `json:"template_sha256"`
	InventorySHA256   string  `json:"inventory_sha256"`
	TargetsSHA256     string  `json:"targets_sha256"`
	RequestSHA256     string  `json:"request_sha256"`
	PlanSHA256        string  `json:"plan_sha256"`
	UnitSHA256        string  `json:"unit_sha256"`
	CertificateSHA256 string  `json:"certificate_sha256"`
	Account           string  `json:"account"`
	UID               uint32  `json:"uid"`
	GID               uint32  `json:"gid"`
	History           string  `json:"history"`
	Request           Request `json:"request"`
}

func decodeBinding(raw []byte) (binding, error) {
	var b binding
	if len(raw) == 0 || len(raw) > 32<<10 || json.Unmarshal(raw, &b) != nil {
		return b, ErrInput
	}
	encoded, err := json.Marshal(b)
	if err != nil || !bytes.Equal(raw, append(encoded, '\n')) || b.Schema != "ardents-forwarding-binding-v1" || b.Invocation != "forwarding" || b.ProgramTarget != infrastructure.ProgramTarget || b.GenerationTarget != infrastructure.ForwardingGenerationTarget || b.Account != "ardents-node" || b.UID < 100 || b.UID > 999 || b.GID != b.UID || !validPath(b.History) {
		return binding{}, ErrBinding
	}
	for _, sha := range []string{b.Generation, b.ProgramSHA256, b.TemplateSHA256, b.InventorySHA256, b.TargetsSHA256, b.RequestSHA256, b.PlanSHA256, b.UnitSHA256, b.CertificateSHA256} {
		if len(sha) != 64 || strings.Trim(sha, "0123456789abcdef") != "" {
			return binding{}, ErrBinding
		}
	}
	request, err := json.Marshal(b.Request)
	if err != nil {
		return binding{}, err
	}
	request = append(request, '\n')
	if _, err = DecodeRequest(request); err != nil || digest(request) != b.RequestSHA256 {
		return binding{}, ErrBinding
	}
	for _, path := range b.Request.ownedPaths() {
		if contains(b.History, path) || contains(path, b.History) {
			return binding{}, ErrBinding
		}
	}
	unit, err := RenderUnit([]byte(Template), b.Request, b.Generation)
	if err != nil || digest([]byte(Template)) != b.TemplateSHA256 || digest(unit) != b.UnitSHA256 {
		return binding{}, ErrBinding
	}
	return b, nil
}
