package content

import (
	"errors"
	"sort"

	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
)

var ErrUnknownProcedure = errors.New("application content procedure is not registered")

type ProcedureRule struct {
	Action       string
	ResourceKind string
}

var procedureAccess = map[string]ProcedureRule{
	applicationv1connect.ContentServicePutProcedure: {
		Action:       ActionPut,
		ResourceKind: "content-owner",
	},
	applicationv1connect.ContentServiceGetProcedure: {
		Action:       ActionGet,
		ResourceKind: "owned-content",
	},
}

func RuleForProcedure(procedure string) (ProcedureRule, error) {
	rule, ok := procedureAccess[procedure]
	if !ok {
		return ProcedureRule{}, ErrUnknownProcedure
	}
	return rule, nil
}

func ProtectedProcedures() []string {
	procedures := make([]string, 0, len(procedureAccess))
	for procedure := range procedureAccess {
		procedures = append(procedures, procedure)
	}
	sort.Strings(procedures)
	return procedures
}
