package processfixture

import (
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

type DescriptorAuthorityPlan struct {
	Network                                   state.AuthorityDeclaration
	Participation, Spend, Hosting, Descriptor string
	Certificate, PrivateKey, Fault            string
	Successor                                 networkfixture.Closed
	Conflict                                  []byte
}

type IntroductionAuthorityPlan struct {
	Network                                     state.AuthorityDeclaration
	Participation, Spend, Hosting, Introduction string
	Certificate, PrivateKey, Fault              string
	Successor                                   networkfixture.Closed
	Conflict                                    []byte
}

type IssuerAuthorityPlan struct {
	Network                        state.AuthorityDeclaration
	Participation, Spend, Hosting  string
	Certificate, PrivateKey, Fault string
	Successor                      networkfixture.Closed
	Conflict                       []byte
	Issuance                       issuer.Plan
}

type JoinAuthorityPlan struct {
	Network                        state.AuthorityDeclaration
	Participation, Spend, Hosting  string
	Certificate, PrivateKey, Fault string
	Successor                      networkfixture.Closed
	Conflict                       []byte
}

type DescriptorOperationPlan struct {
	Original DescriptorAuthorityPlan
	Signals  string
}

type IntroductionOperationPlan struct {
	Original IntroductionAuthorityPlan
	Signals  string
}

type IssuerOperationPlan struct {
	Original    IssuerAuthorityPlan
	Signals     string
	ResultBytes int64
	QuotaBytes  int64
}

type JoinOperationPlan struct {
	Original JoinAuthorityPlan
	Signals  string
}
